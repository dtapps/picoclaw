package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/identity"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/utils"
)

// browserConn 表示一个 WebSocket 连接
type browserConn struct {
	id        string
	conn      *websocket.Conn
	sessionID string
	writeMu   sync.Mutex
	closed    atomic.Bool
	cancel    context.CancelFunc
}

// pendingAction 跟踪一个进行中的浏览器操作请求
type pendingAction struct {
	resultChan chan map[string]any
}

// writeJSON 加锁发送 JSON 消息到连接
func (bc *browserConn) writeJSON(v any) error {
	if bc.closed.Load() {
		return fmt.Errorf("connection closed")
	}
	bc.writeMu.Lock()
	defer bc.writeMu.Unlock()
	return bc.conn.WriteJSON(v)
}

// close 关闭连接
func (bc *browserConn) close() {
	if bc.closed.CompareAndSwap(false, true) {
		if bc.cancel != nil {
			bc.cancel()
		}
		bc.conn.Close()
	}
}

// BrowserChannel 浏览器扩展的 WebSocket 频道。
// 使用 Pico Protocol 线格式但独立配置，
// 允许浏览器扩展直接连接，无需 Launcher 仪表板的 Cookie 认证层。
type BrowserChannel struct {
	*channels.BaseChannel
	bc                 *config.Channel
	config             *config.BrowserSettings
	upgrader           websocket.Upgrader
	connections        map[string]*browserConn            // connID -> *browserConn
	sessionConnections map[string]map[string]*browserConn // sessionID -> connID -> *browserConn
	connsMu            sync.RWMutex
	pendingActions     map[string]*pendingAction // requestID -> pendingAction
	pendingMu          sync.RWMutex
	ctx                context.Context
	cancel             context.CancelFunc
	progress           *channels.ToolFeedbackAnimator
	deleteMessageFn    func(context.Context, string, string) error
}

// NewBrowserChannel 创建浏览器扩展频道
func NewBrowserChannel(
	bc *config.Channel,
	cfg *config.BrowserSettings,
	messageBus *bus.MessageBus,
) (*BrowserChannel, error) {
	if cfg.Token.String() == "" {
		return nil, fmt.Errorf("browser token is required")
	}

	base := channels.NewBaseChannel("browser", cfg, messageBus, bc.AllowFrom)

	allowOrigins := cfg.AllowOrigins
	checkOrigin := func(r *http.Request) bool {
		if len(allowOrigins) == 0 {
			return true
		}
		origin := r.Header.Get("Origin")
		for _, allowed := range allowOrigins {
			if allowed == "*" || allowed == origin {
				return true
			}
		}
		return false
	}

	ch := &BrowserChannel{
		BaseChannel: base,
		bc:          bc,
		config:      cfg,
		upgrader: websocket.Upgrader{
			CheckOrigin:     checkOrigin,
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		connections:        make(map[string]*browserConn),
		sessionConnections: make(map[string]map[string]*browserConn),
		pendingActions:     make(map[string]*pendingAction),
	}
	ch.progress = channels.NewToolFeedbackAnimator(ch.EditMessage)
	ch.deleteMessageFn = ch.DeleteMessage
	return ch, nil
}

// createAndAddConnection 检查最大连接数并原子性注册连接
func (c *BrowserChannel) createAndAddConnection(
	conn *websocket.Conn,
	sessionID string,
	maxConns int,
) (*browserConn, error) {
	c.connsMu.Lock()
	defer c.connsMu.Unlock()
	if len(c.connections) >= maxConns {
		return nil, channels.ErrTemporary
	}

	var connID string
	for {
		connID = uuid.New().String()
		if _, exists := c.connections[connID]; !exists {
			break
		}
	}

	bc := &browserConn{
		id:        connID,
		conn:      conn,
		sessionID: sessionID,
	}

	c.connections[bc.id] = bc
	bySession, ok := c.sessionConnections[bc.sessionID]
	if !ok {
		bySession = make(map[string]*browserConn)
		c.sessionConnections[bc.sessionID] = bySession
	}
	bySession[bc.id] = bc

	return bc, nil
}

// removeConnection 从索引中删除连接并返回
func (c *BrowserChannel) removeConnection(connID string) *browserConn {
	c.connsMu.Lock()
	defer c.connsMu.Unlock()

	bc, ok := c.connections[connID]
	if !ok {
		return nil
	}

	delete(c.connections, connID)
	if bySession, ok := c.sessionConnections[bc.sessionID]; ok {
		delete(bySession, connID)
		if len(bySession) == 0 {
			delete(c.sessionConnections, bc.sessionID)
		}
	}

	return bc
}

// takeAllConnections 快照并清空所有连接索引
func (c *BrowserChannel) takeAllConnections() []*browserConn {
	c.connsMu.Lock()
	defer c.connsMu.Unlock()

	all := make([]*browserConn, 0, len(c.connections))
	for _, bc := range c.connections {
		all = append(all, bc)
	}
	clear(c.connections)
	clear(c.sessionConnections)

	return all
}

// sessionConnectionsSnapshot 返回某个 session 的所有活跃连接
func (c *BrowserChannel) sessionConnectionsSnapshot(sessionID string) []*browserConn {
	c.connsMu.RLock()
	defer c.connsMu.RUnlock()

	bySession, ok := c.sessionConnections[sessionID]
	if !ok || len(bySession) == 0 {
		return nil
	}

	conns := make([]*browserConn, 0, len(bySession))
	for _, bc := range bySession {
		conns = append(conns, bc)
	}
	return conns
}

// currentConnCount 加锁返回活跃连接数
func (c *BrowserChannel) currentConnCount() int {
	c.connsMu.RLock()
	defer c.connsMu.RUnlock()
	return len(c.connections)
}

// Start 实现 Channel 接口
func (c *BrowserChannel) Start(ctx context.Context) error {
	logger.InfoC("browser", "Starting Browser channel")
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.SetRunning(true)
	logger.InfoC("browser", "Browser channel started")
	return nil
}

// Stop 实现 Channel 接口
func (c *BrowserChannel) Stop(ctx context.Context) error {
	logger.InfoC("browser", "Stopping Browser channel")
	c.SetRunning(false)

	for _, bc := range c.takeAllConnections() {
		bc.close()
	}

	if c.cancel != nil {
		c.cancel()
	}
	if c.progress != nil {
		c.progress.StopAll()
	}

	logger.InfoC("browser", "Browser channel stopped")
	return nil
}

// WebhookPath 实现 channels.WebhookHandler 接口
func (c *BrowserChannel) WebhookPath() string { return "/browser/" }

// ServeHTTP 实现 http.Handler，用于共享 HTTP 服务器
func (c *BrowserChannel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/browser")

	switch path {
	case "/ws", "/ws/":
		c.handleWebSocket(w, r)
	default:
		if strings.HasPrefix(path, "/media/") {
			c.handleMediaDownload(w, r)
			return
		}
		http.NotFound(w, r)
	}
}

// Send 实现 Channel 接口，发送消息到对应的 WebSocket 连接
func (c *BrowserChannel) Send(ctx context.Context, msg bus.OutboundMessage) ([]string, error) {
	if !c.IsRunning() {
		return nil, channels.ErrNotRunning
	}
	isThought := outboundMessageIsThought(msg)
	isToolFeedback := outboundMessageIsToolFeedback(msg)
	isToolCalls := outboundMessageIsToolCalls(msg)
	if isToolFeedback {
		if msgID, handled, err := c.progress.Update(ctx, msg.ChatID, msg.Content); handled {
			if err != nil {
				return nil, err
			}
			return []string{msgID}, nil
		}
	}
	trackedMsgID, hasTrackedMsg := c.currentToolFeedbackMessage(msg.ChatID)
	if outboundMessageFinalizesTrackedToolFeedback(msg) {
		if msgIDs, handled := c.FinalizeToolFeedbackMessage(ctx, msg); handled {
			return msgIDs, nil
		}
	}

	content := msg.Content
	if isToolFeedback {
		content = channels.InitialAnimatedToolFeedbackContent(msg.Content)
	}
	msgID := uuid.New().String()

	payload := map[string]any{
		PayloadKeyContent: content,
		"message_id":      msgID,
	}
	switch {
	case isThought:
		payload[PayloadKeyKind] = MessageKindThought
		payload[PayloadKeyThought] = true
	case isToolCalls:
		payload[PayloadKeyKind] = MessageKindToolCalls
		if toolCalls, ok := browserToolCallsPayload(msg); ok {
			payload[PayloadKeyToolCalls] = toolCalls
		}
	}
	setContextUsagePayload(payload, msg.ContextUsage)
	outMsg := newBrowserMessage(TypeMessageCreate, payload)

	if err := c.broadcastToSession(msg.ChatID, outMsg); err != nil {
		return nil, err
	}
	if isToolFeedback {
		c.RecordToolFeedbackMessage(msg.ChatID, msgID, msg.Content)
	} else if hasTrackedMsg && outboundMessageFinalizesTrackedToolFeedback(msg) {
		c.dismissTrackedToolFeedbackMessage(ctx, msg.ChatID, trackedMsgID)
	}
	return []string{msgID}, nil
}

// EditMessage 实现 channels.MessageEditor 接口
func (c *BrowserChannel) EditMessage(ctx context.Context, chatID string, messageID string, content string) error {
	return c.editMessage(ctx, chatID, messageID, content, nil)
}

// DeleteMessage 按 ID 删除消息
func (c *BrowserChannel) DeleteMessage(ctx context.Context, chatID string, messageID string) error {
	outMsg := newBrowserMessage(TypeMessageDelete, map[string]any{
		"message_id": messageID,
	})
	return c.broadcastToSession(chatID, outMsg)
}

func (c *BrowserChannel) currentToolFeedbackMessage(chatID string) (string, bool) {
	if c.progress == nil {
		return "", false
	}
	return c.progress.Current(chatID)
}

func (c *BrowserChannel) takeToolFeedbackMessage(chatID string) (string, string, bool) {
	if c.progress == nil {
		return "", "", false
	}
	return c.progress.Take(chatID)
}

func (c *BrowserChannel) RecordToolFeedbackMessage(chatID, messageID, content string) {
	if c.progress == nil {
		return
	}
	c.progress.Record(chatID, messageID, content)
}

func (c *BrowserChannel) ClearToolFeedbackMessage(chatID string) {
	if c.progress == nil {
		return
	}
	c.progress.Clear(chatID)
}

func (c *BrowserChannel) DismissToolFeedbackMessage(ctx context.Context, chatID string) {
	msgID, ok := c.currentToolFeedbackMessage(chatID)
	if !ok {
		return
	}
	c.dismissTrackedToolFeedbackMessage(ctx, chatID, msgID)
}

func (c *BrowserChannel) dismissTrackedToolFeedbackMessage(ctx context.Context, chatID, messageID string) {
	if strings.TrimSpace(chatID) == "" || strings.TrimSpace(messageID) == "" {
		return
	}
	c.ClearToolFeedbackMessage(chatID)
	deleteFn := c.deleteMessageFn
	if deleteFn == nil {
		deleteFn = c.DeleteMessage
	}
	_ = deleteFn(ctx, chatID, messageID)
}

func (c *BrowserChannel) finalizeTrackedToolFeedbackMessage(
	ctx context.Context,
	chatID string,
	content string,
	editFn func(context.Context, string, string, string, *bus.ContextUsage) error,
	contextUsage *bus.ContextUsage,
) ([]string, bool) {
	msgID, baseContent, ok := c.takeToolFeedbackMessage(chatID)
	if !ok || editFn == nil {
		return nil, false
	}
	if err := editFn(ctx, chatID, msgID, content, contextUsage); err != nil {
		c.RecordToolFeedbackMessage(chatID, msgID, baseContent)
		return nil, false
	}
	return []string{msgID}, true
}

func (c *BrowserChannel) FinalizeToolFeedbackMessage(ctx context.Context, msg bus.OutboundMessage) ([]string, bool) {
	if !outboundMessageFinalizesTrackedToolFeedback(msg) {
		return nil, false
	}
	return c.finalizeTrackedToolFeedbackMessage(ctx, msg.ChatID, msg.Content, c.editMessage, msg.ContextUsage)
}

// StartTyping 实现 channels.TypingCapable 接口
func (c *BrowserChannel) StartTyping(ctx context.Context, chatID string) (func(), error) {
	startMsg := newBrowserMessage(TypeTypingStart, nil)
	if err := c.broadcastToSession(chatID, startMsg); err != nil {
		return func() {}, err
	}
	return func() {
		stopMsg := newBrowserMessage(TypeTypingStop, nil)
		c.broadcastToSession(chatID, stopMsg)
	}, nil
}

// SendPlaceholder 实现 channels.PlaceholderCapable 接口
func (c *BrowserChannel) SendPlaceholder(ctx context.Context, chatID string) (string, error) {
	if !c.bc.Placeholder.Enabled {
		return "", nil
	}

	text := c.bc.Placeholder.GetRandomText()

	msgID := uuid.New().String()
	outMsg := newBrowserMessage(TypeMessageCreate, map[string]any{
		PayloadKeyContent: text,
		"message_id":      msgID,
	})

	if err := c.broadcastToSession(chatID, outMsg); err != nil {
		return "", err
	}

	return msgID, nil
}

// broadcastToSession 向匹配 session 的所有连接发送消息
func (c *BrowserChannel) broadcastToSession(chatID string, msg BrowserMessage) error {
	// chatID 格式: "browser:<sessionID>"
	sessionID := strings.TrimPrefix(chatID, "browser:")
	msg.SessionID = sessionID

	var sent bool
	for _, bc := range c.sessionConnectionsSnapshot(sessionID) {
		if err := bc.writeJSON(msg); err != nil {
			logger.DebugCF("browser", "Write to connection failed", map[string]any{
				"conn_id": bc.id,
				"error":   err.Error(),
			})
		} else {
			sent = true
		}
	}

	if !sent {
		return fmt.Errorf("no active connections for session %s: %w", sessionID, channels.ErrSendFailed)
	}
	return nil
}

// handleWebSocket 升级 HTTP 连接并管理 WebSocket 生命周期
func (c *BrowserChannel) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !c.IsRunning() {
		http.Error(w, "channel not running", http.StatusServiceUnavailable)
		return
	}

	// 认证
	if !c.authenticate(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// 检查连接数限制
	maxConns := c.config.MaxConnections
	if maxConns <= 0 {
		maxConns = 100
	}
	if c.currentConnCount() >= maxConns {
		http.Error(w, "too many connections", http.StatusServiceUnavailable)
		return
	}

	// 回显匹配的子协议，使浏览器接受升级
	var responseHeader http.Header
	if proto := c.matchedSubprotocol(r); proto != "" {
		responseHeader = http.Header{"Sec-WebSocket-Protocol": {proto}}
	}

	conn, err := c.upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		logger.ErrorCF("browser", "WebSocket upgrade failed", map[string]any{
			"error": err.Error(),
		})
		return
	}

	// 从查询参数获取 session ID，或生成新的
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	bc, err := c.createAndAddConnection(conn, sessionID, maxConns)
	if err != nil {
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "too many connections"),
			time.Now().Add(2*time.Second),
		)
		_ = conn.Close()
		return
	}

	logger.InfoCF("browser", "WebSocket client connected", map[string]any{
		"conn_id":    bc.id,
		"session_id": sessionID,
	})

	go c.readLoop(bc)
}

// authenticate 检查请求中的有效 token：
//  1. Authorization: Bearer <token> 请求头
//  2. Sec-WebSocket-Protocol "token.<value>"（适用于无法设置请求头的浏览器）
//  3. 查询参数 "token"（仅在 AllowTokenQuery 开启时）
func (c *BrowserChannel) authenticate(r *http.Request) bool {
	token := c.config.Token.String()
	if token == "" {
		return false
	}

	// 检查 Authorization 请求头
	auth := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
		if after == token {
			return true
		}
	}

	// 检查 Sec-WebSocket-Protocol 子协议（"token.<value>"）
	if c.matchedSubprotocol(r) != "" {
		return true
	}

	// 仅在显式允许时检查查询参数
	if c.config.AllowTokenQuery {
		if r.URL.Query().Get("token") == token {
			return true
		}
	}

	return false
}

// matchedSubprotocol 返回匹配配置 token 的 "token.<value>" 子协议，无匹配则返回 ""
func (c *BrowserChannel) matchedSubprotocol(r *http.Request) string {
	token := c.config.Token.String()
	for _, proto := range websocket.Subprotocols(r) {
		if after, ok := strings.CutPrefix(proto, "token."); ok && after == token {
			return proto
		}
	}
	return ""
}

// readLoop 从 WebSocket 连接读取消息
func (c *BrowserChannel) readLoop(bc *browserConn) {
	defer func() {
		bc.close()
		if removed := c.removeConnection(bc.id); removed != nil {
			logger.InfoCF("browser", "WebSocket client disconnected", map[string]any{
				"conn_id":    removed.id,
				"session_id": removed.sessionID,
			})
		}
	}()

	readTimeout := time.Duration(c.config.ReadTimeout) * time.Second
	if readTimeout <= 0 {
		readTimeout = 60 * time.Second
	}

	_ = bc.conn.SetReadDeadline(time.Now().Add(readTimeout))
	bc.conn.SetPongHandler(func(appData string) error {
		_ = bc.conn.SetReadDeadline(time.Now().Add(readTimeout))
		return nil
	})

	// 启动 ping 心跳
	pingInterval := time.Duration(c.config.PingInterval) * time.Second
	if pingInterval <= 0 {
		pingInterval = 30 * time.Second
	}
	go c.pingLoop(bc, pingInterval)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		_, rawMsg, err := bc.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logger.DebugCF("browser", "WebSocket read error", map[string]any{
					"conn_id": bc.id,
					"error":   err.Error(),
				})
			}
			return
		}

		_ = bc.conn.SetReadDeadline(time.Now().Add(readTimeout))

		var msg BrowserMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			errMsg := newBrowserError("invalid_message", "failed to parse message")
			bc.writeJSON(errMsg)
			continue
		}

		c.handleMessage(bc, msg)
	}
}

// pingLoop 定期发送 ping 帧保持连接存活
func (c *BrowserChannel) pingLoop(bc *browserConn, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if bc.closed.Load() {
				return
			}
			bc.writeMu.Lock()
			err := bc.conn.WriteMessage(websocket.PingMessage, nil)
			bc.writeMu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

// handleMessage 处理入站的浏览器消息
func (c *BrowserChannel) handleMessage(bc *browserConn, msg BrowserMessage) {
	switch msg.Type {
	case TypePing:
		pong := newBrowserMessage(TypePong, nil)
		pong.ID = msg.ID
		bc.writeJSON(pong)

	case TypeMessageSend, TypeMediaSend:
		c.handleMessageSend(bc, msg)

	case TypeActionResult:
		c.handleActionResult(msg)

	default:
		errMsg := newBrowserError("unknown_type", fmt.Sprintf("unknown message type: %s", msg.Type))
		bc.writeJSON(errMsg)
	}
}

// handleMessageSend 处理客户端发来的 message.send 消息
func (c *BrowserChannel) handleMessageSend(bc *browserConn, msg BrowserMessage) {
	content, _ := msg.Payload["content"].(string)
	media, err := parseInlineImageMedia(msg.Payload)
	if err != nil {
		errMsg := newBrowserErrorWithPayload("invalid_media", err.Error(), map[string]any{
			"request_id": msg.ID,
		})
		bc.writeJSON(errMsg)
		return
	}

	if strings.TrimSpace(content) == "" && len(media) == 0 {
		errMsg := newBrowserErrorWithPayload("empty_content", "message content is empty", map[string]any{
			"request_id": msg.ID,
		})
		bc.writeJSON(errMsg)
		return
	}

	sessionID := msg.SessionID
	if sessionID == "" {
		sessionID = bc.sessionID
	}

	chatID := "browser:" + sessionID
	senderID := "browser-user"

	metadata := map[string]string{
		"platform":   "browser",
		"session_id": sessionID,
		"conn_id":    bc.id,
	}

	logger.DebugCF("browser", "Received message", map[string]any{
		"session_id": sessionID,
		"preview":    truncate(content, 50),
		"media":      len(media),
	})

	sender := bus.SenderInfo{
		Platform:    "browser",
		PlatformID:  senderID,
		CanonicalID: identity.BuildCanonicalID("browser", senderID),
	}

	if !c.IsAllowedSender(sender) {
		return
	}

	inboundCtx := bus.InboundContext{
		Channel:   "browser",
		ChatID:    chatID,
		ChatType:  "direct",
		SenderID:  senderID,
		MessageID: msg.ID,
		Raw:       metadata,
	}

	c.HandleInboundContext(c.ctx, chatID, content, media, inboundCtx, sender)
}

// handleMediaDownload 为已认证的浏览器客户端提供媒体文件下载
func (c *BrowserChannel) handleMediaDownload(w http.ResponseWriter, r *http.Request) {
	if !c.IsRunning() {
		http.Error(w, "channel not running", http.StatusServiceUnavailable)
		return
	}
	if !c.authenticate(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	refID := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/browser/media/"), "/"))
	if refID == "" {
		http.NotFound(w, r)
		return
	}

	store := c.GetMediaStore()
	if store == nil {
		http.Error(w, "media store unavailable", http.StatusServiceUnavailable)
		return
	}

	localPath, meta, err := store.ResolveWithMeta("media://" + refID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	file, err := os.Open(localPath)
	if err != nil {
		http.Error(w, "failed to open media", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		http.Error(w, "failed to stat media", http.StatusInternalServerError)
		return
	}

	filename := strings.TrimSpace(meta.Filename)
	if filename == "" {
		filename = filepath.Base(localPath)
	}
	contentType := strings.TrimSpace(meta.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	dispositionType := "attachment"
	if allowsInlineDisplay(filename, contentType) {
		dispositionType = "inline"
	}

	if cd := mime.FormatMediaType(dispositionType, map[string]string{"filename": filename}); cd != "" {
		w.Header().Set("Content-Disposition", cd)
	}
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, filename, info.ModTime(), file)
}

func (c *BrowserChannel) editMessage(
	ctx context.Context,
	chatID string,
	messageID string,
	content string,
	contextUsage *bus.ContextUsage,
) error {
	payload := map[string]any{
		"message_id": messageID,
		"content":    content,
	}
	setContextUsagePayload(payload, contextUsage)
	outMsg := newBrowserMessage(TypeMessageUpdate, payload)
	return c.broadcastToSession(chatID, outMsg)
}

// ===== 协议类型（与 Pico Protocol 线格式相同）=====

const (
	TypeMessageSend = "message.send"
	TypeMediaSend   = "media.send"
	TypePing        = "ping"

	TypeMessageCreate = "message.create"
	TypeMessageUpdate = "message.update"
	TypeMessageDelete = "message.delete"
	TypeMediaCreate   = "media.create"
	TypeTypingStart   = "typing.start"
	TypeTypingStop    = "typing.stop"
	TypeError         = "error"
	TypePong          = "pong"

	TypeActionRequest = "action.request"
	TypeActionResult  = "action.result"

	PayloadKeyContent   = "content"
	PayloadKeyThought   = "thought"
	PayloadKeyKind      = "kind"
	PayloadKeyToolCalls = "tool_calls"

	MessageKindThought   = "thought"
	MessageKindToolCalls = "tool_calls"
)

// BrowserMessage 线格式（与 Pico Protocol 相同）
type BrowserMessage struct {
	Type      string         `json:"type"`
	ID        string         `json:"id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Timestamp int64          `json:"timestamp,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

func newBrowserMessage(msgType string, payload map[string]any) BrowserMessage {
	return BrowserMessage{
		Type:      msgType,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	}
}

func newBrowserErrorWithPayload(code, message string, extra map[string]any) BrowserMessage {
	payload := map[string]any{
		"code":    code,
		"message": message,
	}
	for key, value := range extra {
		payload[key] = value
	}
	return newBrowserMessage(TypeError, payload)
}

func newBrowserError(code, message string) BrowserMessage {
	return newBrowserErrorWithPayload(code, message, nil)
}

// ===== 辅助函数 =====

var allowedInlineImageMIMETypes = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/gif":  {},
	"image/webp": {},
	"image/bmp":  {},
}

func outboundMessageIsThought(msg bus.OutboundMessage) bool {
	if len(msg.Context.Raw) == 0 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(msg.Context.Raw[PayloadKeyKind]), MessageKindThought)
}

func outboundMessageIsToolFeedback(msg bus.OutboundMessage) bool {
	if len(msg.Context.Raw) == 0 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(msg.Context.Raw["message_kind"]), "tool_feedback")
}

func outboundMessageIsToolCalls(msg bus.OutboundMessage) bool {
	if len(msg.Context.Raw) == 0 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(msg.Context.Raw[PayloadKeyKind]), MessageKindToolCalls)
}

func outboundMessageFinalizesTrackedToolFeedback(msg bus.OutboundMessage) bool {
	return !outboundMessageIsToolFeedback(msg) &&
		!outboundMessageIsThought(msg) &&
		!outboundMessageIsToolCalls(msg)
}

func browserToolCallsPayload(msg bus.OutboundMessage) ([]utils.VisibleToolCall, bool) {
	raw := strings.TrimSpace(msg.Context.Raw[PayloadKeyToolCalls])
	if raw == "" {
		return nil, false
	}

	var toolCalls []utils.VisibleToolCall
	if err := json.Unmarshal([]byte(raw), &toolCalls); err != nil || len(toolCalls) == 0 {
		return nil, false
	}
	return toolCalls, true
}

func setContextUsagePayload(payload map[string]any, u *bus.ContextUsage) {
	if u == nil {
		return
	}
	payload["context_usage"] = map[string]any{
		"used_tokens":        u.UsedTokens,
		"total_tokens":       u.TotalTokens,
		"compress_at_tokens": u.CompressAtTokens,
		"used_percent":       u.UsedPercent,
	}
}

func parseInlineImageMedia(payload map[string]any) ([]string, error) {
	if len(payload) == 0 {
		return nil, nil
	}

	raw, ok := payload["media"]
	if !ok || raw == nil {
		return nil, nil
	}

	switch values := raw.(type) {
	case []any:
		media := make([]string, 0, len(values))
		for i, item := range values {
			value, err := inlineImageValue(item)
			if err != nil {
				return nil, fmt.Errorf("media[%d]: %w", i, err)
			}
			if err := validateInlineImageDataURL(value); err != nil {
				return nil, fmt.Errorf("media[%d]: %w", i, err)
			}
			media = append(media, value)
		}
		return media, nil
	case []string:
		media := make([]string, 0, len(values))
		for i, value := range values {
			value = strings.TrimSpace(value)
			if err := validateInlineImageDataURL(value); err != nil {
				return nil, fmt.Errorf("media[%d]: %w", i, err)
			}
			media = append(media, value)
		}
		return media, nil
	case string:
		value := strings.TrimSpace(values)
		if err := validateInlineImageDataURL(value); err != nil {
			return nil, err
		}
		return []string{value}, nil
	default:
		return nil, fmt.Errorf("media must be a string or array of strings")
	}
}

func inlineImageValue(item any) (string, error) {
	switch value := item.(type) {
	case string:
		value = strings.TrimSpace(value)
		if value == "" {
			return "", fmt.Errorf("image payload is empty")
		}
		return value, nil
	case map[string]any:
		for _, key := range []string{"url", "data_url"} {
			if raw, ok := value[key].(string); ok && strings.TrimSpace(raw) != "" {
				return strings.TrimSpace(raw), nil
			}
		}
		return "", fmt.Errorf("image payload must include url or data_url")
	default:
		return "", fmt.Errorf("image payload must be a string or object")
	}
}

func validateInlineImageDataURL(mediaURL string) error {
	if mediaURL == "" {
		return fmt.Errorf("image payload is empty")
	}
	if !strings.HasPrefix(mediaURL, "data:image/") {
		return fmt.Errorf("only inline image data URLs are supported")
	}

	header, data, found := strings.Cut(mediaURL, ",")
	if !found || strings.TrimSpace(data) == "" {
		return fmt.Errorf("image data URL is malformed")
	}
	if !strings.Contains(header, ";base64") {
		return fmt.Errorf("image data URL must be base64 encoded")
	}
	mimeType, _, _ := strings.Cut(strings.TrimPrefix(header, "data:"), ";")
	if _, ok := allowedInlineImageMIMETypes[mimeType]; !ok {
		return fmt.Errorf("unsupported image format: %s", mimeType)
	}

	data = strings.TrimSpace(data)
	if base64.StdEncoding.DecodedLen(len(data)) > config.DefaultMaxMediaSize {
		return fmt.Errorf("image exceeds %d byte limit", config.DefaultMaxMediaSize)
	}
	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return fmt.Errorf("invalid base64 image data")
	}

	return nil
}

func allowsInlineDisplay(filename, contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	filename = strings.ToLower(strings.TrimSpace(filename))

	if strings.Contains(contentType, "svg") || filepath.Ext(filename) == ".svg" {
		return false
	}

	return inferAttachmentType(filename, contentType) == "image"
}

func inferAttachmentType(filename, contentType string) string {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	filename = strings.ToLower(strings.TrimSpace(filename))

	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	case strings.HasPrefix(contentType, "audio/"):
		return "audio"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	}

	switch ext := filepath.Ext(filename); ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg":
		return "image"
	case ".mp3", ".wav", ".ogg", ".m4a", ".flac", ".aac", ".wma", ".opus":
		return "audio"
	case ".mp4", ".avi", ".mov", ".webm", ".mkv":
		return "video"
	default:
		return "file"
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// ===== 浏览器操作（工具 → 扩展）=====

// handleActionResult 处理插件返回的操作结果
func (c *BrowserChannel) handleActionResult(msg BrowserMessage) {
	requestID, _ := msg.Payload["request_id"].(string)
	if requestID == "" {
		logger.WarnC("browser", "action.result missing request_id")
		return
	}

	c.pendingMu.RLock()
	pending, ok := c.pendingActions[requestID]
	c.pendingMu.RUnlock()

	if !ok {
		logger.WarnCF("browser", "action.result for unknown request", map[string]any{"request_id": requestID})
		return
	}

	// 将结果发送给等待的调用者
	select {
	case pending.resultChan <- msg.Payload:
	default:
		logger.WarnCF("browser", "action.result channel full", map[string]any{"request_id": requestID})
	}

	// 清理
	c.pendingMu.Lock()
	delete(c.pendingActions, requestID)
	c.pendingMu.Unlock()
}

// ExecuteAction 通过浏览器插件执行操作，供 BrowserExtTool 回调使用
func (c *BrowserChannel) ExecuteAction(
	ctx context.Context,
	chatID string,
	action string,
	params map[string]any,
) (map[string]any, error) {
	if !c.IsRunning() {
		return nil, fmt.Errorf("browser channel not running")
	}

	// 从 chatID 提取 sessionID（格式: "browser:<sessionID>"）
	sessionID := strings.TrimPrefix(chatID, "browser:")

	// 找到该 session 的一个活跃连接
	c.connsMu.RLock()
	conns, ok := c.sessionConnections[sessionID]
	c.connsMu.RUnlock()

	if !ok || len(conns) == 0 {
		return nil, fmt.Errorf("no browser extension connected for session %s", sessionID)
	}

	// 取第一个可用连接
	var bc *browserConn
	for _, conn := range conns {
		if !conn.closed.Load() {
			bc = conn
			break
		}
	}
	if bc == nil {
		return nil, fmt.Errorf("no active browser extension connection for session %s", sessionID)
	}

	// 生成请求 ID
	requestID := uuid.New().String()

	// 注册 pending action
	pending := &pendingAction{
		resultChan: make(chan map[string]any, 1),
	}
	c.pendingMu.Lock()
	c.pendingActions[requestID] = pending
	c.pendingMu.Unlock()

	// 确保清理
	defer func() {
		c.pendingMu.Lock()
		delete(c.pendingActions, requestID)
		c.pendingMu.Unlock()
	}()

	// 发送 action.request 到插件
	requestMsg := BrowserMessage{
		Type:      TypeActionRequest,
		ID:        requestID,
		SessionID: sessionID,
		Timestamp: time.Now().UnixMilli(),
		Payload: map[string]any{
			"request_id": requestID,
			"action":     action,
			"params":     params,
		},
	}

	if err := bc.writeJSON(requestMsg); err != nil {
		return nil, fmt.Errorf("failed to send action request: %w", err)
	}

	// 等待结果
	select {
	case result := <-pending.resultChan:
		// 检查是否有错误
		if errMsg, _ := result["error"].(string); errMsg != "" {
			return nil, fmt.Errorf("browser action failed: %s", errMsg)
		}
		return result, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("browser action timed out: %w", ctx.Err())
	}
}

// GetActionCallback 返回给 BrowserExtTool 使用的回调函数
func (c *BrowserChannel) GetActionCallback() func(ctx context.Context, chatID string, action string, params map[string]any) (map[string]any, error) {
	return c.ExecuteAction
}
