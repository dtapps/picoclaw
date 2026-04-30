package providers

import (
	"context"
	"fmt"

	"github.com/sipeed/picoclaw/pkg/providers/common"
	"github.com/sipeed/picoclaw/pkg/providers/protocoltypes"
)

type (
	ToolCall               = protocoltypes.ToolCall
	FunctionCall           = protocoltypes.FunctionCall
	LLMResponse            = protocoltypes.LLMResponse
	UsageInfo              = protocoltypes.UsageInfo
	Message                = protocoltypes.Message
	ToolDefinition         = protocoltypes.ToolDefinition
	ToolFunctionDefinition = protocoltypes.ToolFunctionDefinition
	ExtraContent           = protocoltypes.ExtraContent
	GoogleExtra            = protocoltypes.GoogleExtra
	ContentBlock           = protocoltypes.ContentBlock
	CacheControl           = protocoltypes.CacheControl
	Attachment             = protocoltypes.Attachment
)

type LLMProvider interface {
	Chat(
		ctx context.Context,
		messages []Message,
		tools []ToolDefinition,
		model string,
		options map[string]any,
	) (*LLMResponse, error)
	GetDefaultModel() string
}

type StatefulProvider interface {
	LLMProvider
	Close()
}

// StreamingProvider is an optional interface for providers that support token streaming.
// onChunk receives the accumulated text so far (not individual deltas).
// The returned LLMResponse is the same complete response for compatibility with tool-call handling.
type StreamingProvider interface {
	ChatStream(
		ctx context.Context,
		messages []Message,
		tools []ToolDefinition,
		model string,
		options map[string]any,
		onChunk func(accumulated string),
	) (*LLMResponse, error)
}

// ThinkingCapable is an optional interface for providers that support
// extended thinking (e.g. Anthropic). Used by the agent loop to warn
// when thinking_level is configured but the active provider cannot use it.
type ThinkingCapable interface {
	SupportsThinking() bool
}

// NativeSearchCapable is an optional interface for providers that support
// built-in web search during LLM inference (e.g. OpenAI web_search_preview,
// xAI Grok search). When the active provider implements this interface and
// returns true, the agent loop can hide the client-side web_search tool to
// avoid duplicate search surfaces and use the provider's native search instead.
type NativeSearchCapable interface {
	SupportsNativeSearch() bool
}

// FailoverReason classifies why an LLM request failed for fallback decisions.
type FailoverReason string

const (
	FailoverAuth            FailoverReason = "auth"
	FailoverRateLimit       FailoverReason = "rate_limit"
	FailoverBilling         FailoverReason = "billing"
	FailoverNetwork         FailoverReason = "network"
	FailoverTimeout         FailoverReason = "timeout"
	FailoverFormat          FailoverReason = "format"
	FailoverContextOverflow FailoverReason = "context_overflow"
	FailoverOverloaded      FailoverReason = "overloaded"
	FailoverUnknown         FailoverReason = "unknown"
)

// FailoverError wraps an LLM provider error with classification metadata.
type FailoverError struct {
	Reason   FailoverReason
	Provider string
	Model    string
	Status   int
	Wrapped  error
}

func (e *FailoverError) Error() string {
	return fmt.Sprintf("failover(%s): provider=%s model=%s status=%d: %v",
		e.Reason, e.Provider, e.Model, e.Status, e.Wrapped)
}

func (e *FailoverError) Unwrap() error {
	return e.Wrapped
}

// IsRetriable returns true if this error should trigger fallback to next candidate.
// Non-retriable: Format errors (bad request structure, image dimension/size).
func (e *FailoverError) IsRetriable() bool {
	return e.Reason != FailoverFormat && e.Reason != FailoverContextOverflow
}

// ModelConfig holds primary model and fallback list.
type ModelConfig struct {
	Primary   string
	Fallbacks []string
}

// NormalizeInlineToolCalls 对 LLMResponse 进行后处理：提取部分模型（如 kimi-k2）
// 嵌入在 content 文本中的内联工具调用，并将其转换为标准 tool_calls 字段。
func NormalizeInlineToolCalls(resp *LLMResponse) {
	common.NormalizeInlineToolCalls(resp)
}

// CleanModelContent 清理模型响应 content 中的 Anthropic 风格包装和特殊 token。
// 典型场景：kimi-k2 返回纯文本响应时，内容被包装为
// [{'type': 'text', 'text': '好，我重新来一遍！'}<|tool_call_end|><|tool_calls_section_end|>
// 此函数提取其中的实际文本内容，移除包装和特殊 token。
func CleanModelContent(content string) string {
	return common.CleanModelContent(content)
}
