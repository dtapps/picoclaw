package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
)

// testBrowserSettings 创建用于测试的有效 BrowserSettings
func testBrowserSettings() *config.BrowserSettings {
	s := &config.BrowserSettings{
		Token:          *config.NewSecureString("test-token"),
		MaxConnections: 10,
		PingInterval:   30,
		ReadTimeout:    60,
	}
	return s
}

// dialWS 连接 WebSocket 测试服务器，确保关闭 response body
func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("WebSocket 连接失败: %v", err)
	}
	if resp.Body != nil {
		resp.Body.Close()
	}
	return conn
}

// dialWSIgnoreErr 连接 WebSocket 测试服务器，忽略错误，确保关闭 response body
func dialWSIgnoreErr(url string) {
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err == nil && conn != nil {
		conn.Close()
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
}

// newTestBrowserChannel 创建用于测试的 BrowserChannel 实例
func newTestBrowserChannel(
	t *testing.T,
	cfg *config.BrowserSettings,
	bc *config.Channel,
) (*BrowserChannel, *bus.MessageBus) {
	t.Helper()

	if cfg == nil {
		cfg = testBrowserSettings()
	}
	msgBus := bus.NewMessageBus()
	if bc == nil {
		bc = &config.Channel{Type: config.ChannelBrowser, Enabled: true}
	}
	ch, err := NewBrowserChannel(bc, cfg, msgBus)
	if err != nil {
		t.Fatalf("创建频道失败: %v", err)
	}
	return ch, msgBus
}

// ---------------------------------------------------------------------------
// NewBrowserChannel 构造函数测试
// ---------------------------------------------------------------------------

// TestNewBrowserChannel_MissingToken 缺少 token 时应返回错误
func TestNewBrowserChannel_MissingToken(t *testing.T) {
	cfg := &config.BrowserSettings{}
	bc := &config.Channel{Type: config.ChannelBrowser, Enabled: true}
	msgBus := bus.NewMessageBus()

	_, err := NewBrowserChannel(bc, cfg, msgBus)
	if err == nil {
		t.Fatal("缺少 token 时应返回错误")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Fatalf("错误信息应包含 token，实际: %v", err)
	}
}

// TestNewBrowserChannel_ValidConfig 有效配置应成功创建频道
func TestNewBrowserChannel_ValidConfig(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	if ch.Name() != "browser" {
		t.Errorf("Name() = %q, 期望 %q", ch.Name(), "browser")
	}
	if ch.IsRunning() {
		t.Error("新创建的频道不应处于运行状态")
	}
}

// TestNewBrowserChannel_WebhookPath WebhookPath 应返回 /browser/
func TestNewBrowserChannel_WebhookPath(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	if ch.WebhookPath() != "/browser/" {
		t.Errorf("WebhookPath() = %q, 期望 %q", ch.WebhookPath(), "/browser/")
	}
}

// TestNewBrowserChannel_AllowOriginsEmpty 空 AllowOrigins 时应允许所有来源
func TestNewBrowserChannel_AllowOriginsEmpty(t *testing.T) {
	cfg := testBrowserSettings()
	cfg.AllowOrigins = nil

	ch, _ := newTestBrowserChannel(t, cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/browser/ws", nil)
	req.Header.Set("Origin", "https://example.com")
	// CheckOrigin 为空时应返回 true
	if !ch.upgrader.CheckOrigin(req) {
		t.Error("空 AllowOrigins 应允许所有来源")
	}
}

// TestNewBrowserChannel_AllowOriginsWildcard AllowOrigins 包含 "*" 时应允许所有来源
func TestNewBrowserChannel_AllowOriginsWildcard(t *testing.T) {
	cfg := testBrowserSettings()
	cfg.AllowOrigins = []string{"*"}

	ch, _ := newTestBrowserChannel(t, cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/browser/ws", nil)
	req.Header.Set("Origin", "https://any-site.com")
	if !ch.upgrader.CheckOrigin(req) {
		t.Error("AllowOrigins 含 '*' 时应允许所有来源")
	}
}

// TestNewBrowserChannel_AllowOriginsSpecific 指定 AllowOrigins 时只允许匹配的来源
func TestNewBrowserChannel_AllowOriginsSpecific(t *testing.T) {
	cfg := testBrowserSettings()
	cfg.AllowOrigins = []string{"https://allowed.com"}

	ch, _ := newTestBrowserChannel(t, cfg, nil)

	// 匹配的来源
	req1 := httptest.NewRequest(http.MethodGet, "/browser/ws", nil)
	req1.Header.Set("Origin", "https://allowed.com")
	if !ch.upgrader.CheckOrigin(req1) {
		t.Error("匹配的来源应被允许")
	}

	// 不匹配的来源
	req2 := httptest.NewRequest(http.MethodGet, "/browser/ws", nil)
	req2.Header.Set("Origin", "https://blocked.com")
	if ch.upgrader.CheckOrigin(req2) {
		t.Error("不匹配的来源应被拒绝")
	}
}

// ---------------------------------------------------------------------------
// Start / Stop 测试
// ---------------------------------------------------------------------------

// TestStart 启动后频道应处于运行状态
func TestStart(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	err := ch.Start(context.Background())
	if err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	if !ch.IsRunning() {
		t.Error("启动后应处于运行状态")
	}
}

// TestStop 停止后频道不应处于运行状态
func TestStop(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	_ = ch.Start(context.Background())
	err := ch.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop 失败: %v", err)
	}
	if ch.IsRunning() {
		t.Error("停止后不应处于运行状态")
	}
}

// TestStop_Idempotent Stop 应可幂等调用
func TestStop_Idempotent(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	// 未启动时调用 Stop 也不应 panic
	_ = ch.Stop(context.Background())
	_ = ch.Stop(context.Background())

	if ch.IsRunning() {
		t.Error("停止后不应处于运行状态")
	}
}

// ---------------------------------------------------------------------------
// Send 未运行时发送消息测试
// ---------------------------------------------------------------------------

// TestSend_NotRunning 频道未运行时发送消息应返回 ErrNotRunning
func TestSend_NotRunning(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	_, err := ch.Send(context.Background(), bus.OutboundMessage{
		ChatID:  "browser:session-1",
		Content: "hello",
	})
	if err == nil {
		t.Fatal("未运行的频道发送消息应返回错误")
	}
	if !errors.Is(err, channels.ErrNotRunning) {
		t.Errorf("期望 ErrNotRunning，实际: %v", err)
	}
}

// TestSend_NoActiveConnections 无活跃连接时应返回 ErrSendFailed
func TestSend_NoActiveConnections(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)
	_ = ch.Start(context.Background())

	_, err := ch.Send(context.Background(), bus.OutboundMessage{
		ChatID:  "browser:session-1",
		Content: "hello",
	})
	if err == nil {
		t.Fatal("无活跃连接时应返回错误")
	}
	if !errors.Is(err, channels.ErrSendFailed) {
		t.Errorf("期望 ErrSendFailed，实际: %v", err)
	}
}

// ---------------------------------------------------------------------------
// EditMessage 测试（无连接时应返回 ErrSendFailed）
// ---------------------------------------------------------------------------

// TestEditMessage_NoActiveConnections 无连接时 EditMessage 应返回错误
func TestEditMessage_NoActiveConnections(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)
	_ = ch.Start(context.Background())

	err := ch.EditMessage(context.Background(), "browser:session-1", "msg-1", "new content")
	if err == nil {
		t.Fatal("无活跃连接时 EditMessage 应返回错误")
	}
}

// ---------------------------------------------------------------------------
// DeleteMessage 测试（无连接时应返回 ErrSendFailed）
// ---------------------------------------------------------------------------

// TestDeleteMessage_NoActiveConnections 无连接时 DeleteMessage 应返回错误
func TestDeleteMessage_NoActiveConnections(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)
	_ = ch.Start(context.Background())

	err := ch.DeleteMessage(context.Background(), "browser:session-1", "msg-1")
	if err == nil {
		t.Fatal("无活跃连接时 DeleteMessage 应返回错误")
	}
}

// ---------------------------------------------------------------------------
// authenticate 认证测试
// ---------------------------------------------------------------------------

// TestAuthenticate_BearerHeader 通过 Authorization Bearer 请求头认证
func TestAuthenticate_BearerHeader(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/browser/ws", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	if !ch.authenticate(req) {
		t.Error("有效的 Bearer token 应认证通过")
	}
}

// TestAuthenticate_BearerHeaderWrongToken 错误的 Bearer token 应认证失败
func TestAuthenticate_BearerHeaderWrongToken(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/browser/ws", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")

	if ch.authenticate(req) {
		t.Error("错误的 Bearer token 应认证失败")
	}
}

// TestAuthenticate_Subprotocol 通过 Sec-WebSocket-Protocol 子协议认证
func TestAuthenticate_Subprotocol(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/browser/ws", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "token.test-token, other")

	if !ch.authenticate(req) {
		t.Error("有效的子协议 token 应认证通过")
	}
}

// TestAuthenticate_SubprotocolWrongToken 错误的子协议 token 应认证失败
func TestAuthenticate_SubprotocolWrongToken(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/browser/ws", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "token.wrong-token")

	if ch.authenticate(req) {
		t.Error("错误的子协议 token 应认证失败")
	}
}

// TestAuthenticate_QueryTokenForbidden 默认不允许查询参数 token
func TestAuthenticate_QueryTokenForbidden(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/browser/ws?token=test-token", nil)

	if ch.authenticate(req) {
		t.Error("默认不允许通过查询参数 token 认证")
	}
}

// TestAuthenticate_QueryTokenAllowed AllowTokenQuery 开启时允许查询参数 token
func TestAuthenticate_QueryTokenAllowed(t *testing.T) {
	cfg := testBrowserSettings()
	cfg.AllowTokenQuery = true

	ch, _ := newTestBrowserChannel(t, cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/browser/ws?token=test-token", nil)

	if !ch.authenticate(req) {
		t.Error("AllowTokenQuery 开启时应允许查询参数 token 认证")
	}
}

// TestAuthenticate_NoCredentials 无任何凭证应认证失败
func TestAuthenticate_NoCredentials(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/browser/ws", nil)

	if ch.authenticate(req) {
		t.Error("无凭证应认证失败")
	}
}

// ---------------------------------------------------------------------------
// matchedSubprotocol 测试
// ---------------------------------------------------------------------------

// TestMatchedSubprotocol 测试子协议匹配逻辑
func TestMatchedSubprotocol(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	tests := []struct {
		name     string
		protocol string
		want     string
	}{
		{
			name:     "匹配的 token 子协议",
			protocol: "token.test-token",
			want:     "token.test-token",
		},
		{
			name:     "不匹配的 token 子协议",
			protocol: "token.wrong-token",
			want:     "",
		},
		{
			name:     "非 token 前缀的子协议",
			protocol: "other-protocol",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/browser/ws", nil)
			req.Header.Set("Sec-WebSocket-Protocol", tt.protocol)

			got := ch.matchedSubprotocol(req)
			if got != tt.want {
				t.Errorf("matchedSubprotocol() = %q, 期望 %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 连接管理测试
// ---------------------------------------------------------------------------

// TestCreateAndAddConnection 创建并注册连接
func TestCreateAndAddConnection(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	// 创建一对 WebSocket 连接
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// 注册连接
		bc, err := ch.createAndAddConnection(conn, "session-1", 10)
		if err != nil {
			t.Errorf("createAndAddConnection 失败: %v", err)
			return
		}
		if bc.sessionID != "session-1" {
			t.Errorf("sessionID = %q, 期望 %q", bc.sessionID, "session-1")
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn := dialWS(t, wsURL)
	defer conn.Close()

	// 验证连接已被注册
	if ch.currentConnCount() < 1 {
		t.Error("应至少有一个连接")
	}
}

// TestCreateAndAddConnection_MaxExceeded 超过最大连接数时应返回错误
func TestCreateAndAddConnection_MaxExceeded(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// 第一次连接应成功（max=1）
		_, err = ch.createAndAddConnection(conn, "session-1", 1)
		if err != nil {
			t.Errorf("第一次连接应成功: %v", err)
		}

		// 直接再添加一个（模拟超过限制）
		srv2Conn, _ := upgrader.Upgrade(w, r, nil)
		if srv2Conn != nil {
			_, err = ch.createAndAddConnection(srv2Conn, "session-1", 1)
			if err == nil {
				t.Error("超过最大连接数应返回错误")
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	dialWSIgnoreErr(wsURL)
}

// TestRemoveConnection 移除已注册的连接
func TestRemoveConnection(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		bc, err := ch.createAndAddConnection(conn, "session-1", 10)
		if err != nil {
			return
		}

		// 移除连接
		removed := ch.removeConnection(bc.id)
		if removed == nil {
			t.Error("应能移除已注册的连接")
		}
		if removed.id != bc.id {
			t.Errorf("移除的连接 ID 不匹配: got %q, want %q", removed.id, bc.id)
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	dialWSIgnoreErr(wsURL)

	// 移除后连接数应为 0
	if ch.currentConnCount() != 0 {
		t.Errorf("移除后连接数应为 0，实际: %d", ch.currentConnCount())
	}
}

// TestRemoveConnection_NonExistent 移除不存在的连接应返回 nil
func TestRemoveConnection_NonExistent(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	removed := ch.removeConnection("non-existent")
	if removed != nil {
		t.Error("移除不存在的连接应返回 nil")
	}
}

// TestSessionConnectionsSnapshot 获取 session 的活跃连接快照
func TestSessionConnectionsSnapshot(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		_, _ = ch.createAndAddConnection(conn, "session-1", 10)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	dialWSIgnoreErr(wsURL)

	// 获取 session-1 的连接快照
	conns := ch.sessionConnectionsSnapshot("session-1")
	if len(conns) < 1 {
		t.Error("session-1 应至少有一个连接")
	}

	// 不存在的 session 应返回 nil
	conns = ch.sessionConnectionsSnapshot("non-existent")
	if conns != nil {
		t.Error("不存在的 session 应返回 nil")
	}
}

// TestTakeAllConnections 快照并清空所有连接
func TestTakeAllConnections(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		_, _ = ch.createAndAddConnection(conn, "session-1", 10)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	dialWSIgnoreErr(wsURL)

	all := ch.takeAllConnections()
	if len(all) < 1 {
		t.Error("应至少取出一个连接")
	}
	if ch.currentConnCount() != 0 {
		t.Error("取出后连接数应为 0")
	}
}

// ---------------------------------------------------------------------------
// BrowserMessage 消息构建测试
// ---------------------------------------------------------------------------

// TestNewBrowserMessage 测试消息构建
func TestNewBrowserMessage(t *testing.T) {
	payload := map[string]any{"content": "hello"}
	msg := newBrowserMessage(TypeMessageCreate, payload)

	if msg.Type != TypeMessageCreate {
		t.Errorf("Type = %q, 期望 %q", msg.Type, TypeMessageCreate)
	}
	if msg.Timestamp <= 0 {
		t.Error("Timestamp 应大于 0")
	}
	if msg.Payload["content"] != "hello" {
		t.Errorf("Payload content = %v, 期望 %q", msg.Payload["content"], "hello")
	}
}

// TestNewBrowserError 测试错误消息构建
func TestNewBrowserError(t *testing.T) {
	msg := newBrowserError("test_code", "test message")

	if msg.Type != TypeError {
		t.Errorf("Type = %q, 期望 %q", msg.Type, TypeError)
	}
	if msg.Payload["code"] != "test_code" {
		t.Errorf("code = %v, 期望 %q", msg.Payload["code"], "test_code")
	}
	if msg.Payload["message"] != "test message" {
		t.Errorf("message = %v, 期望 %q", msg.Payload["message"], "test message")
	}
}

// TestNewBrowserErrorWithPayload 测试带附加字段的错误消息构建
func TestNewBrowserErrorWithPayload(t *testing.T) {
	extra := map[string]any{"request_id": "req-123"}
	msg := newBrowserErrorWithPayload("err", "desc", extra)

	if msg.Payload["request_id"] != "req-123" {
		t.Errorf("request_id = %v, 期望 %q", msg.Payload["request_id"], "req-123")
	}
	if msg.Payload["code"] != "err" {
		t.Errorf("code = %v, 期望 %q", msg.Payload["code"], "err")
	}
}

// TestBrowserMessage_JSONRoundTrip 测试消息的 JSON 序列化/反序列化
func TestBrowserMessage_JSONRoundTrip(t *testing.T) {
	original := newBrowserMessage(TypeMessageCreate, map[string]any{
		"content":    "测试消息",
		"message_id": "msg-1",
	})
	original.ID = "id-1"
	original.SessionID = "session-1"

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	var parsed BrowserMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	if parsed.Type != original.Type {
		t.Errorf("Type = %q, 期望 %q", parsed.Type, original.Type)
	}
	if parsed.ID != original.ID {
		t.Errorf("ID = %q, 期望 %q", parsed.ID, original.ID)
	}
	if parsed.SessionID != original.SessionID {
		t.Errorf("SessionID = %q, 期望 %q", parsed.SessionID, original.SessionID)
	}
	if parsed.Payload["content"] != "测试消息" {
		t.Errorf("content = %v, 期望 %q", parsed.Payload["content"], "测试消息")
	}
}

// ---------------------------------------------------------------------------
// 辅助函数测试
// ---------------------------------------------------------------------------

// TestTruncate 测试字符串截断
func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"短字符串不截断", "hello", 10, "hello"},
		{"恰好不截断", "hello", 5, "hello"},
		{"需要截断", "hello world", 5, "hello..."},
		{"空字符串", "", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, 期望 %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// TestInferAttachmentType 测试附件类型推断
func TestInferAttachmentType(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		contentType string
		want        string
	}{
		{"JPEG 图片", "photo.jpg", "image/jpeg", "image"},
		{"PNG 图片", "icon.png", "image/png", "image"},
		{"MP3 音频", "song.mp3", "audio/mpeg", "audio"},
		{"MP4 视频", "clip.mp4", "video/mp4", "video"},
		{"未知内容类型按扩展名判断图片", "photo.gif", "", "image"},
		{"未知内容类型按扩展名判断音频", "song.wav", "", "audio"},
		{"未知内容类型按扩展名判断视频", "clip.mkv", "", "video"},
		{"未知类型", "doc.pdf", "application/pdf", "file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferAttachmentType(tt.filename, tt.contentType)
			if got != tt.want {
				t.Errorf("inferAttachmentType(%q, %q) = %q, 期望 %q",
					tt.filename, tt.contentType, got, tt.want)
			}
		})
	}
}

// TestAllowsInlineDisplay 测试内联显示判断
func TestAllowsInlineDisplay(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		contentType string
		want        bool
	}{
		{"JPEG 可内联", "photo.jpg", "image/jpeg", true},
		{"PNG 可内联", "icon.png", "image/png", true},
		{"SVG 不可内联（内容类型）", "drawing.svg", "image/svg+xml", false},
		{"SVG 不可内联（扩展名）", "drawing.svg", "application/octet-stream", false},
		{"PDF 不可内联", "doc.pdf", "application/pdf", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := allowsInlineDisplay(tt.filename, tt.contentType)
			if got != tt.want {
				t.Errorf("allowsInlineDisplay(%q, %q) = %v, 期望 %v",
					tt.filename, tt.contentType, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// validateInlineImageDataURL 测试
// ---------------------------------------------------------------------------

// TestValidateInlineImageDataURL 测试内联图片 data URL 验证
func TestValidateInlineImageDataURL(t *testing.T) {
	validPNG := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("fake image data"))

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"有效的 PNG data URL", validPNG, false},
		{"空字符串", "", true},
		{"非 data: 前缀", "https://example.com/img.png", true},
		{"非 image/ 前缀", "data:text/plain;base64,abc", true},
		{"缺少 base64 编码标记", "data:image/png;ascii,fake", true},
		{"缺少逗号分隔符", "data:image/png;base64", true},
		{"不支持的图片格式", "data:image/tiff;base64," + base64.StdEncoding.EncodeToString([]byte("x")), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInlineImageDataURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateInlineImageDataURL(%q) 错误 = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseInlineImageMedia 测试
// ---------------------------------------------------------------------------

// TestParseInlineImageMedia 测试解析内联图片媒体
func TestParseInlineImageMedia(t *testing.T) {
	validPNG := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("fake"))

	tests := []struct {
		name    string
		payload map[string]any
		wantLen int
		wantErr bool
	}{
		{
			name:    "空 payload",
			payload: nil,
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "无 media 字段",
			payload: map[string]any{"content": "hello"},
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "media 为 nil",
			payload: map[string]any{"media": nil},
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "media 为字符串数组",
			payload: map[string]any{"media": []any{validPNG}},
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "media 为单个字符串",
			payload: map[string]any{"media": validPNG},
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "media 为无效类型",
			payload: map[string]any{"media": 123},
			wantLen: 0,
			wantErr: true,
		},
		{
			name:    "media 含无效 data URL",
			payload: map[string]any{"media": "https://example.com/img.png"},
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseInlineImageMedia(tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInlineImageMedia() 错误 = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && len(result) != tt.wantLen {
				t.Errorf("结果长度 = %d, 期望 %d", len(result), tt.wantLen)
			}
		})
	}
}

// TestParseInlineImageMedia_ObjectPayload 测试媒体对象格式的解析
func TestParseInlineImageMedia_ObjectPayload(t *testing.T) {
	validPNG := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("fake"))

	// 对象格式含 url 字段
	result, err := parseInlineImageMedia(map[string]any{
		"media": []any{
			map[string]any{"url": validPNG},
		},
	})
	if err != nil {
		t.Fatalf("解析对象格式失败: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("结果长度 = %d, 期望 1", len(result))
	}
	if result[0] != validPNG {
		t.Errorf("结果 = %q, 期望 %q", result[0], validPNG)
	}

	// 对象格式含 data_url 字段
	result2, err := parseInlineImageMedia(map[string]any{
		"media": []any{
			map[string]any{"data_url": validPNG},
		},
	})
	if err != nil {
		t.Fatalf("解析 data_url 格式失败: %v", err)
	}
	if len(result2) != 1 {
		t.Errorf("结果长度 = %d, 期望 1", len(result2))
	}
}

// TestParseInlineImageMedia_ObjectMissingURL 对象格式缺少 url 和 data_url 应报错
func TestParseInlineImageMedia_ObjectMissingURL(t *testing.T) {
	_, err := parseInlineImageMedia(map[string]any{
		"media": []any{
			map[string]any{"other": "value"},
		},
	})
	if err == nil {
		t.Error("对象缺少 url/data_url 字段应返回错误")
	}
}

// ---------------------------------------------------------------------------
// outboundMessage 辅助判断函数测试
// ---------------------------------------------------------------------------

// TestOutboundMessageIsThought 测试判断是否为思考消息
func TestOutboundMessageIsThought(t *testing.T) {
	tests := []struct {
		name string
		msg  bus.OutboundMessage
		want bool
	}{
		{
			name: "思考消息",
			msg: bus.OutboundMessage{
				Context: bus.InboundContext{Raw: map[string]string{"kind": "thought"}},
			},
			want: true,
		},
		{
			name: "非思考消息",
			msg: bus.OutboundMessage{
				Context: bus.InboundContext{Raw: map[string]string{"kind": "normal"}},
			},
			want: false,
		},
		{
			name: "空 Raw",
			msg:  bus.OutboundMessage{},
			want: false,
		},
		{
			name: "大写 THOUGHT",
			msg: bus.OutboundMessage{
				Context: bus.InboundContext{Raw: map[string]string{"kind": "THOUGHT"}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := outboundMessageIsThought(tt.msg)
			if got != tt.want {
				t.Errorf("outboundMessageIsThought() = %v, 期望 %v", got, tt.want)
			}
		})
	}
}

// TestOutboundMessageIsToolFeedback 测试判断是否为工具反馈消息
func TestOutboundMessageIsToolFeedback(t *testing.T) {
	tests := []struct {
		name string
		msg  bus.OutboundMessage
		want bool
	}{
		{
			name: "工具反馈消息",
			msg: bus.OutboundMessage{
				Context: bus.InboundContext{Raw: map[string]string{"message_kind": "tool_feedback"}},
			},
			want: true,
		},
		{
			name: "非工具反馈消息",
			msg: bus.OutboundMessage{
				Context: bus.InboundContext{Raw: map[string]string{"message_kind": "normal"}},
			},
			want: false,
		},
		{
			name: "空 Raw",
			msg:  bus.OutboundMessage{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := outboundMessageIsToolFeedback(tt.msg)
			if got != tt.want {
				t.Errorf("outboundMessageIsToolFeedback() = %v, 期望 %v", got, tt.want)
			}
		})
	}
}

// TestOutboundMessageIsToolCalls 测试判断是否为工具调用消息
func TestOutboundMessageIsToolCalls(t *testing.T) {
	tests := []struct {
		name string
		msg  bus.OutboundMessage
		want bool
	}{
		{
			name: "工具调用消息",
			msg: bus.OutboundMessage{
				Context: bus.InboundContext{Raw: map[string]string{"kind": "tool_calls"}},
			},
			want: true,
		},
		{
			name: "非工具调用消息",
			msg: bus.OutboundMessage{
				Context: bus.InboundContext{Raw: map[string]string{"kind": "normal"}},
			},
			want: false,
		},
		{
			name: "空 Raw",
			msg:  bus.OutboundMessage{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := outboundMessageIsToolCalls(tt.msg)
			if got != tt.want {
				t.Errorf("outboundMessageIsToolCalls() = %v, 期望 %v", got, tt.want)
			}
		})
	}
}

// TestOutboundMessageFinalizesTrackedToolFeedback 测试判断是否终结工具反馈
func TestOutboundMessageFinalizesTrackedToolFeedback(t *testing.T) {
	tests := []struct {
		name string
		msg  bus.OutboundMessage
		want bool
	}{
		{
			name: "普通消息终结工具反馈",
			msg: bus.OutboundMessage{
				Context: bus.InboundContext{Raw: map[string]string{"kind": "normal"}},
			},
			want: true,
		},
		{
			name: "思考消息不终结",
			msg: bus.OutboundMessage{
				Context: bus.InboundContext{Raw: map[string]string{"kind": "thought"}},
			},
			want: false,
		},
		{
			name: "工具反馈消息不终结",
			msg: bus.OutboundMessage{
				Context: bus.InboundContext{Raw: map[string]string{"message_kind": "tool_feedback"}},
			},
			want: false,
		},
		{
			name: "工具调用消息不终结",
			msg: bus.OutboundMessage{
				Context: bus.InboundContext{Raw: map[string]string{"kind": "tool_calls"}},
			},
			want: false,
		},
		{
			name: "空 Raw 终结",
			msg:  bus.OutboundMessage{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := outboundMessageFinalizesTrackedToolFeedback(tt.msg)
			if got != tt.want {
				t.Errorf("outboundMessageFinalizesTrackedToolFeedback() = %v, 期望 %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// setContextUsagePayload 测试
// ---------------------------------------------------------------------------

// TestSetContextUsagePayload 测试上下文用量设置
func TestSetContextUsagePayload(t *testing.T) {
	t.Run("nil ContextUsage 不设置字段", func(t *testing.T) {
		payload := map[string]any{"content": "hello"}
		setContextUsagePayload(payload, nil)
		if _, ok := payload["context_usage"]; ok {
			t.Error("nil ContextUsage 不应设置 context_usage 字段")
		}
	})

	t.Run("有效 ContextUsage 设置字段", func(t *testing.T) {
		payload := map[string]any{"content": "hello"}
		usage := &bus.ContextUsage{
			UsedTokens:  100,
			TotalTokens: 1000,
		}
		setContextUsagePayload(payload, usage)
		cu, ok := payload["context_usage"]
		if !ok {
			t.Fatal("应设置 context_usage 字段")
		}
		cuMap, ok := cu.(map[string]any)
		if !ok {
			t.Fatal("context_usage 应为 map[string]any 类型")
		}
		if cuMap["used_tokens"] != 100 {
			t.Errorf("used_tokens = %v, 期望 100", cuMap["used_tokens"])
		}
		if cuMap["total_tokens"] != 1000 {
			t.Errorf("total_tokens = %v, 期望 1000", cuMap["total_tokens"])
		}
	})
}

// ---------------------------------------------------------------------------
// ServeHTTP 路由测试
// ---------------------------------------------------------------------------

// TestServeHTTP_NotRunning 频道未运行时 WebSocket 请求应返回 503
func TestServeHTTP_NotRunning(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)
	// 不调用 Start

	req := httptest.NewRequest(http.MethodGet, "/browser/ws", nil)
	rec := httptest.NewRecorder()

	ch.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("状态码 = %d, 期望 %d", rec.Code, http.StatusServiceUnavailable)
	}
}

// TestServeHTTP_UnknownPath 未知路径应返回 404
func TestServeHTTP_UnknownPath(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)
	_ = ch.Start(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/browser/unknown", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	ch.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("状态码 = %d, 期望 %d", rec.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// 连接管理并发测试
// ---------------------------------------------------------------------------

// TestConcurrentConnectionAccess 并发操作连接索引不应数据竞争
func TestConcurrentConnectionAccess(t *testing.T) {
	ch, _ := newTestBrowserChannel(t, nil, nil)

	var wg sync.WaitGroup
	const goroutines = 50

	// 并发读写连接索引
	for i := range goroutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// 并发读取连接数
			_ = ch.currentConnCount()
			_ = ch.sessionConnectionsSnapshot("session-1")
		}(i)
	}
	wg.Wait()
}
