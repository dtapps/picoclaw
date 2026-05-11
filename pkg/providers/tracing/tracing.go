// Package tracing 提供通用的请求追踪功能。
// 通过配置文件将上下文字段映射为自定义 HTTP 请求头，
// 不绑定任何特定的追踪服务或协议。
package tracing

import "context"

// contextKey 用于在 context.Context 中存储追踪信息的 key 类型
type contextKey string

const (
	// headersKey 在 context 中存储请求头映射配置（header名→上下文字段名）
	headersKey contextKey = "tracing-headers"
	// 以下为上下文字段值的 key
	sessionKeyKey   contextKey = "tracing-session-key"
	turnIDKey       contextKey = "tracing-turn-id"
	agentIDKey      contextKey = "tracing-agent-id"
	agentNameKey    contextKey = "tracing-agent-name"
	channelKey      contextKey = "tracing-channel"
	chatIDKey       contextKey = "tracing-chat-id"
	parentTurnIDKey contextKey = "tracing-parent-turn-id"
	senderIDKey     contextKey = "tracing-sender-id"
	messageIDKey    contextKey = "tracing-message-id"
	modelKey        contextKey = "tracing-model"
)

// 已注册的上下文字段名到 context key 的映射
// 可用字段：session_key, turn_id, agent_id, agent_name, channel, chat_id,
// parent_turn_id, sender_id, message_id, model
var fieldToKey = map[string]contextKey{
	"session_key":    sessionKeyKey,
	"turn_id":        turnIDKey,
	"agent_id":       agentIDKey,
	"agent_name":     agentNameKey,
	"channel":        channelKey,
	"chat_id":        chatIDKey,
	"parent_turn_id": parentTurnIDKey,
	"sender_id":      senderIDKey,
	"message_id":     messageIDKey,
	"model":          modelKey,
}

// AvailableFields 返回所有可用的上下文字段名，供 UI 展示或校验使用
func AvailableFields() []string {
	fields := make([]string, 0, len(fieldToKey))
	for k := range fieldToKey {
		fields = append(fields, k)
	}
	return fields
}

// WithHeaders 将请求头映射配置注入到 context 中
func WithHeaders(ctx context.Context, headers map[string]string) context.Context {
	return context.WithValue(ctx, headersKey, headers)
}

// WithSessionKey 将会话标识注入到 context 中
func WithSessionKey(ctx context.Context, sessionKey string) context.Context {
	return context.WithValue(ctx, sessionKeyKey, sessionKey)
}

// WithTurnID 将轮次标识注入到 context 中
func WithTurnID(ctx context.Context, turnID string) context.Context {
	return context.WithValue(ctx, turnIDKey, turnID)
}

// WithAgentID 将 Agent 标识注入到 context 中
func WithAgentID(ctx context.Context, agentID string) context.Context {
	return context.WithValue(ctx, agentIDKey, agentID)
}

// WithAgentName 将 Agent 名称注入到 context 中
func WithAgentName(ctx context.Context, agentName string) context.Context {
	return context.WithValue(ctx, agentNameKey, agentName)
}

// WithChannel 将消息渠道标识注入到 context 中（如 discord、telegram 等）
func WithChannel(ctx context.Context, channel string) context.Context {
	return context.WithValue(ctx, channelKey, channel)
}

// WithChatID 将平台会话 ID 注入到 context 中
func WithChatID(ctx context.Context, chatID string) context.Context {
	return context.WithValue(ctx, chatIDKey, chatID)
}

// WithParentTurnID 将父轮次 ID 注入到 context 中（SubTurn 场景）
func WithParentTurnID(ctx context.Context, parentTurnID string) context.Context {
	return context.WithValue(ctx, parentTurnIDKey, parentTurnID)
}

// WithSenderID 将消息发送者 ID 注入到 context 中
func WithSenderID(ctx context.Context, senderID string) context.Context {
	return context.WithValue(ctx, senderIDKey, senderID)
}

// WithMessageID 将消息 ID 注入到 context 中
func WithMessageID(ctx context.Context, messageID string) context.Context {
	return context.WithValue(ctx, messageIDKey, messageID)
}

// WithModel 将当前使用的模型名注入到 context 中
func WithModel(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, modelKey, model)
}

// HeadersFromContext 根据配置的映射从 context 中提取追踪信息，
// 返回需要设置的 HTTP 请求头。如果没有配置映射或所有字段值为空，返回 nil。
func HeadersFromContext(ctx context.Context) map[string]string {
	headers, _ := ctx.Value(headersKey).(map[string]string)
	if len(headers) == 0 {
		return nil
	}

	result := make(map[string]string, len(headers))
	for headerName, fieldName := range headers {
		ctxKey, ok := fieldToKey[fieldName]
		if !ok {
			continue // 忽略未知的字段名
		}
		if v, _ := ctx.Value(ctxKey).(string); v != "" {
			result[headerName] = v
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}
