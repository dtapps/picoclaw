package agent

import (
	"context"
	"sync"
	"testing"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/providers/protocoltypes"
)

// emptyThenSuccessProvider 第一次调用返回空响应，之后返回正常响应的 mock provider
type emptyThenSuccessProvider struct {
	mu           sync.Mutex
	calls        int
	emptyContent string
	successResp  string
}

func (p *emptyThenSuccessProvider) Chat(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	opts map[string]any,
) (*protocoltypes.LLMResponse, error) {
	p.mu.Lock()
	p.calls++
	callNum := p.calls
	p.mu.Unlock()

	if callNum == 1 {
		return &protocoltypes.LLMResponse{
			Content:   p.emptyContent,
			ToolCalls: []protocoltypes.ToolCall{},
		}, nil
	}
	return &protocoltypes.LLMResponse{
		Content:   p.successResp,
		ToolCalls: []protocoltypes.ToolCall{},
	}, nil
}

func (p *emptyThenSuccessProvider) GetDefaultModel() string {
	return "test-model"
}

// alwaysEmptyProvider 始终返回空响应的 mock provider
type alwaysEmptyProvider struct {
	mu    sync.Mutex
	calls int
}

func (p *alwaysEmptyProvider) Chat(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	opts map[string]any,
) (*protocoltypes.LLMResponse, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	return &protocoltypes.LLMResponse{
		Content:   "[{'type': 'text', 'text': ''}]",
		ToolCalls: []protocoltypes.ToolCall{},
	}, nil
}

func (p *alwaysEmptyProvider) GetDefaultModel() string {
	return "test-model"
}

// newTestAgentLoopWithEmptyRetry 创建带空响应重试配置的测试用 AgentLoop
func newTestAgentLoopWithEmptyRetry(
	t *testing.T,
	provider providers.LLMProvider,
	retryCfg config.EmptyResponseRetryConfig,
) *AgentLoop {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:          tmpDir,
				ModelName:          "test-model",
				MaxTokens:          4096,
				MaxToolIterations:  10,
				EmptyResponseRetry: retryCfg,
			},
		},
		ModelList: []*config.ModelConfig{
			{ModelName: "test-model", Model: "openai/test-model"},
		},
	}
	msgBus := bus.NewMessageBus()
	al := NewAgentLoop(cfg, msgBus, provider)
	al.providerFactory = func(mc *config.ModelConfig) (providers.LLMProvider, string, error) {
		model := provider.GetDefaultModel()
		if mc != nil {
			if _, modelID := providers.ExtractProtocol(mc); modelID != "" {
				model = modelID
			}
		}
		return provider, model, nil
	}
	return al
}

// TestAskSideQuestion_EmptyResponseRetry_SucceedsOnSecondCall 测试空响应重试在第二次调用时成功
func TestAskSideQuestion_EmptyResponseRetry_SucceedsOnSecondCall(t *testing.T) {
	provider := &emptyThenSuccessProvider{
		emptyContent: "[{'type': 'text', 'text': ''}]",
		successResp:  "Here is the answer",
	}

	al := newTestAgentLoopWithEmptyRetry(t, provider, config.EmptyResponseRetryConfig{
		Enabled:    true,
		MaxRetries: 3,
		Patterns:   []string{"[{'type': 'text', 'text': ''}]"},
	})
	defer al.Close()

	agent := al.GetRegistry().GetDefaultAgent()
	if agent == nil {
		t.Fatal("expected default agent")
	}

	result, err := al.askSideQuestion(context.Background(), agent, &processOptions{
		SessionKey: "test-session",
		Channel:    "cli",
		ChatID:     "test-chat",
		NoHistory:  true,
	}, "test question")
	if err != nil {
		t.Fatalf("askSideQuestion() error = %v", err)
	}
	if result != "Here is the answer" {
		t.Errorf("askSideQuestion() = %q, want %q", result, "Here is the answer")
	}
	provider.mu.Lock()
	calls := provider.calls
	provider.mu.Unlock()
	if calls != 2 {
		t.Errorf("provider called %d times, want 2", calls)
	}
}

// TestAskSideQuestion_EmptyResponseRetry_ExhaustsRetries 测试空响应重试耗尽所有重试次数后仍返回空响应
func TestAskSideQuestion_EmptyResponseRetry_ExhaustsRetries(t *testing.T) {
	provider := &alwaysEmptyProvider{}

	al := newTestAgentLoopWithEmptyRetry(t, provider, config.EmptyResponseRetryConfig{
		Enabled:    true,
		MaxRetries: 2,
		Patterns:   []string{"[{'type': 'text', 'text': ''}]"},
	})
	defer al.Close()

	agent := al.GetRegistry().GetDefaultAgent()
	if agent == nil {
		t.Fatal("expected default agent")
	}

	result, err := al.askSideQuestion(context.Background(), agent, &processOptions{
		SessionKey: "test-session",
		Channel:    "cli",
		ChatID:     "test-chat",
		NoHistory:  true,
	}, "test question")
	if err != nil {
		t.Fatalf("askSideQuestion() error = %v", err)
	}
	if result != "[{'type': 'text', 'text': ''}]" {
		t.Errorf("askSideQuestion() = %q, want empty pattern content", result)
	}
	provider.mu.Lock()
	calls := provider.calls
	provider.mu.Unlock()
	if calls != 3 {
		t.Errorf("provider called %d times, want 3 (1 initial + 2 retries)", calls)
	}
}

// TestAskSideQuestion_EmptyResponseRetry_Disabled 测试空响应重试功能关闭时不触发重试
func TestAskSideQuestion_EmptyResponseRetry_Disabled(t *testing.T) {
	provider := &emptyThenSuccessProvider{
		emptyContent: "[{'type': 'text', 'text': ''}]",
		successResp:  "Here is the answer",
	}

	al := newTestAgentLoopWithEmptyRetry(t, provider, config.EmptyResponseRetryConfig{
		Enabled: false,
	})
	defer al.Close()

	agent := al.GetRegistry().GetDefaultAgent()
	if agent == nil {
		t.Fatal("expected default agent")
	}

	result, err := al.askSideQuestion(context.Background(), agent, &processOptions{
		SessionKey: "test-session",
		Channel:    "cli",
		ChatID:     "test-chat",
		NoHistory:  true,
	}, "test question")
	if err != nil {
		t.Fatalf("askSideQuestion() error = %v", err)
	}
	if result != "[{'type': 'text', 'text': ''}]" {
		t.Errorf("askSideQuestion() = %q, want empty pattern content (no retry)", result)
	}
	provider.mu.Lock()
	calls := provider.calls
	provider.mu.Unlock()
	if calls != 1 {
		t.Errorf("provider called %d times, want 1 (no retry)", calls)
	}
}

// TestAskSideQuestion_EmptyResponseRetry_NoPatternsNoRetry 测试未配置匹配模式时不触发重试
func TestAskSideQuestion_EmptyResponseRetry_NoPatternsNoRetry(t *testing.T) {
	provider := &emptyThenSuccessProvider{
		emptyContent: "[{'type': 'text', 'text': ''}]",
		successResp:  "Here is the answer",
	}

	al := newTestAgentLoopWithEmptyRetry(t, provider, config.EmptyResponseRetryConfig{
		Enabled:    true,
		MaxRetries: 3,
		Patterns:   nil,
	})
	defer al.Close()

	agent := al.GetRegistry().GetDefaultAgent()
	if agent == nil {
		t.Fatal("expected default agent")
	}

	result, err := al.askSideQuestion(context.Background(), agent, &processOptions{
		SessionKey: "test-session",
		Channel:    "cli",
		ChatID:     "test-chat",
		NoHistory:  true,
	}, "test question")
	if err != nil {
		t.Fatalf("askSideQuestion() error = %v", err)
	}
	if result != "[{'type': 'text', 'text': ''}]" {
		t.Errorf("askSideQuestion() = %q, want empty pattern content (no patterns = no retry)", result)
	}
	provider.mu.Lock()
	calls := provider.calls
	provider.mu.Unlock()
	if calls != 1 {
		t.Errorf("provider called %d times, want 1 (no patterns configured)", calls)
	}
}

// TestIsEmptyResponse 测试 isEmptyResponse 函数的各种场景
func TestIsEmptyResponse(t *testing.T) {
	tests := []struct {
		name     string
		response *protocoltypes.LLMResponse
		want     bool
	}{
		{
			name:     "nil response",
			response: nil,
			want:     true,
		},
		{
			name: "empty content no tool calls",
			response: &protocoltypes.LLMResponse{
				Content: "",
			},
			want: true,
		},
		{
			name: "whitespace only content",
			response: &protocoltypes.LLMResponse{
				Content: "   ",
			},
			want: true,
		},
		{
			name: "has tool calls with empty content",
			response: &protocoltypes.LLMResponse{
				Content: "",
				ToolCalls: []protocoltypes.ToolCall{
					{ID: "call_1", Name: "exec"},
				},
			},
			want: false,
		},
		{
			name: "normal content",
			response: &protocoltypes.LLMResponse{
				Content: "Hello",
			},
			want: false,
		},
		{
			name: "kimi-k2 style empty structured content",
			response: &protocoltypes.LLMResponse{
				Content: "[{'type': 'text', 'text': ''}]",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmptyResponse(tt.response); got != tt.want {
				t.Errorf("isEmptyResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMatchesEmptyResponsePattern 测试空响应模式匹配的各种场景
func TestMatchesEmptyResponsePattern(t *testing.T) {
	kimiSubstring := "[{'type': 'text', 'text': ''}]"
	kimiRegex := `re:^\[\s*\{\s*['"]type['"]\s*:\s*['"]text['"]\s*,\s*['"]text['"]\s*:\s*['"]['"]\s*\}\s*\]\s*$`

	tests := []struct {
		name     string
		content  string
		patterns []string
		want     bool
	}{
		{
			name:     "empty string",
			content:  "",
			patterns: []string{kimiSubstring},
			want:     true,
		},
		{
			name:     "whitespace only",
			content:  "   ",
			patterns: []string{kimiSubstring},
			want:     true,
		},
		{
			name:     "kimi-k2 single quotes substring match",
			content:  "[{'type': 'text', 'text': ''}]",
			patterns: []string{kimiSubstring},
			want:     true,
		},
		{
			name:     "kimi-k2 double quotes no match with single quote pattern",
			content:  `[{"type": "text", "text": ""}]`,
			patterns: []string{kimiSubstring},
			want:     false,
		},
		{
			name:     "kimi-k2 double quotes substring match",
			content:  `[{"type": "text", "text": ""}]`,
			patterns: []string{`[{"type": "text", "text": ""}]`},
			want:     true,
		},
		{
			name:     "normal text no match",
			content:  "Hello, how can I help?",
			patterns: []string{kimiSubstring},
			want:     false,
		},
		{
			name:     "structured but with content no match",
			content:  `[{"type": "text", "text": "Hello"}]`,
			patterns: []string{kimiSubstring},
			want:     false,
		},
		{
			name:     "regex pattern with re prefix",
			content:  "[{'type': 'text', 'text': ''}]",
			patterns: []string{kimiRegex},
			want:     true,
		},
		{
			name:     "regex double quotes match",
			content:  `[{"type": "text", "text": ""}]`,
			patterns: []string{kimiRegex},
			want:     true,
		},
		{
			name:     "regex with spaces match",
			content:  `[{"type": "text", "text": ""} ]`,
			patterns: []string{kimiRegex},
			want:     true,
		},
		{
			name:     "regex normal text no match",
			content:  "Hello, how can I help?",
			patterns: []string{kimiRegex},
			want:     false,
		},
		{
			name:     "regex structured with content no match",
			content:  `[{"type": "text", "text": "Hello"}]`,
			patterns: []string{kimiRegex},
			want:     false,
		},
		{
			name:     "custom substring pattern",
			content:  "EMPTY_RESPONSE",
			patterns: []string{"EMPTY"},
			want:     true,
		},
		{
			name:     "custom regex pattern",
			content:  "EMPTY_RESPONSE",
			patterns: []string{"re:^EMPTY_RESPONSE$"},
			want:     true,
		},
		{
			name:     "invalid regex pattern skipped",
			content:  "test",
			patterns: []string{"re:[invalid("},
			want:     false,
		},
		{
			name:     "no patterns configured",
			content:  "[{'type': 'text', 'text': ''}]",
			patterns: nil,
			want:     false,
		},
		{
			name:     "mixed patterns substring and regex",
			content:  "[{'type': 'text', 'text': ''}]",
			patterns: []string{"something_else", kimiSubstring},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesEmptyResponsePattern(tt.content, tt.patterns); got != tt.want {
				t.Errorf("matchesEmptyResponsePattern(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

// TestIsEmptyResponseRetryEnabled 测试空响应重试启用的判断逻辑
func TestIsEmptyResponseRetryEnabled(t *testing.T) {
	tests := []struct {
		name   string
		config config.AgentDefaults
		want   bool
	}{
		{
			name:   "disabled by default",
			config: config.AgentDefaults{},
			want:   false,
		},
		{
			name: "enabled without patterns",
			config: config.AgentDefaults{
				EmptyResponseRetry: config.EmptyResponseRetryConfig{
					Enabled: true,
				},
			},
			want: false,
		},
		{
			name: "enabled with patterns",
			config: config.AgentDefaults{
				EmptyResponseRetry: config.EmptyResponseRetryConfig{
					Enabled:  true,
					Patterns: []string{`^test$`},
				},
			},
			want: true,
		},
		{
			name: "patterns but not enabled",
			config: config.AgentDefaults{
				EmptyResponseRetry: config.EmptyResponseRetryConfig{
					Enabled:  false,
					Patterns: []string{`^test$`},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.IsEmptyResponseRetryEnabled(); got != tt.want {
				t.Errorf("IsEmptyResponseRetryEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
