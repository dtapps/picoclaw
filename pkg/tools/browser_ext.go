package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	toolshared "github.com/sipeed/picoclaw/pkg/tools/shared"
)

// BrowserExtActionCallback 发送操作到浏览器插件并返回结果的回调函数类型
type BrowserExtActionCallback func(ctx context.Context, chatID string, action string, params map[string]any) (map[string]any, error)

// BrowserExtTool 通过浏览器插件执行浏览器操作的 Agent 工具
// 与 CDP 版本的 BrowserTool 不同，此工具将操作请求通过 WebSocket
// 转发给浏览器插件，由插件在用户浏览器中执行
type BrowserExtTool struct {
	actionCallback BrowserExtActionCallback
}

// NewBrowserExtTool 创建浏览器插件操作工具
func NewBrowserExtTool() *BrowserExtTool {
	return &BrowserExtTool{}
}

// SetActionCallback 设置操作执行回调，由 browser channel 初始化时注入
func (t *BrowserExtTool) SetActionCallback(callback BrowserExtActionCallback) {
	t.actionCallback = callback
}

// Name 返回工具名称
func (t *BrowserExtTool) Name() string { return "browser_ext" }

// Description 返回工具描述
func (t *BrowserExtTool) Description() string {
	return `Browser automation via the connected browser extension. Actions: navigate, get_page_info, click, type, fill, scroll, screenshot, get_text, select_option, hover, focus, clear.
Workflow: get_page_info → click/type/fill → get_page_info (verify).
The extension operates on the user's actual browser tab. Always call get_page_info first to understand the current page.`
}

// Parameters 返回工具参数定义（OpenAI function calling 格式）
func (t *BrowserExtTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "The browser action to execute",
				"enum": []string{
					"navigate", "get_page_info", "click", "type", "fill",
					"scroll", "screenshot", "get_text", "select_option",
					"hover", "focus", "clear",
				},
			},
			"url": map[string]any{
				"type":        "string",
				"description": "URL to navigate to (for navigate action)",
			},
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS selector of the target element (for click, type, fill, get_text, select_option, hover, focus, clear actions)",
			},
			"index": map[string]any{
				"type":        "integer",
				"description": "Index of the element if multiple match the selector (0-based)",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "Text to type or fill (for type/fill actions), or option value (for select_option action)",
			},
			"direction": map[string]any{
				"type":        "string",
				"description": "Scroll direction: up, down, top, bottom (for scroll action)",
				"enum":        []string{"up", "down", "top", "bottom"},
			},
		},
		"required": []string{"action"},
	}
}

// Execute 执行浏览器操作
func (t *BrowserExtTool) Execute(ctx context.Context, args map[string]any) *toolshared.ToolResult {
	// 只允许在 browser 频道中使用
	channel := toolshared.ToolChannel(ctx)
	if channel != "browser" {
		return &toolshared.ToolResult{
			ForLLM:  "browser_ext tool is only available in the browser channel",
			IsError: true,
			Err:     fmt.Errorf("browser_ext tool not available in channel %q (only browser)", channel),
		}
	}

	if t.actionCallback == nil {
		return &toolshared.ToolResult{
			ForLLM:  "browser extension not connected — no action callback available",
			IsError: true,
			Err:     fmt.Errorf("browser extension action callback not configured"),
		}
	}

	action, _ := args["action"].(string)
	if action == "" {
		return &toolshared.ToolResult{
			ForLLM:  "action parameter is required",
			IsError: true,
			Err:     fmt.Errorf("missing action parameter"),
		}
	}

	// 从 context 获取 chatID
	chatID := toolshared.ToolChatID(ctx)
	if chatID == "" {
		return &toolshared.ToolResult{
			ForLLM:  "no active browser session",
			IsError: true,
			Err:     fmt.Errorf("no chatID in context"),
		}
	}

	// 构建操作参数
	params := buildBrowserExtParams(args)

	// 设置超时
	timeout := 30 * time.Second
	if action == "screenshot" {
		timeout = 60 * time.Second
	}
	actionCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 调用回调执行操作
	result, err := t.actionCallback(actionCtx, chatID, action, params)
	if err != nil {
		return &toolshared.ToolResult{
			ForLLM:  fmt.Sprintf("browser action '%s' failed: %s", action, err.Error()),
			IsError: true,
			Err:     err,
		}
	}

	// 格式化结果
	output := formatBrowserExtResult(action, result)
	return &toolshared.ToolResult{
		ForLLM: output,
	}
}

// buildBrowserExtParams 从工具参数构建发送给插件的操作参数
func buildBrowserExtParams(args map[string]any) map[string]any {
	params := make(map[string]any)

	if v, ok := args["url"].(string); ok && v != "" {
		params["url"] = v
	}
	if v, ok := args["selector"].(string); ok && v != "" {
		params["selector"] = v
	}
	if v, ok := args["text"].(string); ok && v != "" {
		params["text"] = v
	}
	if v, ok := args["direction"].(string); ok && v != "" {
		params["direction"] = v
	}
	if v, ok := args["index"]; ok {
		switch n := v.(type) {
		case float64:
			params["index"] = int(n)
		case int:
			params["index"] = n
		}
	}

	return params
}

// formatBrowserExtResult 将操作结果格式化为可读文本
func formatBrowserExtResult(action string, result map[string]any) string {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("action '%s' completed (result serialization failed)", action)
	}
	return fmt.Sprintf("action '%s' result:\n%s", action, string(data))
}
