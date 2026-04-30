package common

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

const (
	// inlineToolUseMarker 是部分模型（如 kimi-k2）在 content 文本中
	// 标记内联工具调用的前缀
	inlineToolUseMarker = "[tool_use: "

	// inlineToolArgsSeparator 是内联工具调用中名称与参数的分隔符
	inlineToolArgsSeparator = ", args: "

	// inlineToolCallEndToken 和 inlineToolCallsSectionEndToken 是部分模型
	// 在内联工具调用后附加的特殊 token
	inlineToolCallEndToken         = "<|tool_call_end|>"
	inlineToolCallsSectionEndToken = "<|tool_calls_section_end|>"
)

// ExtractInlineToolCalls 扫描 content 中的内联工具调用模式，如：
//
//	[tool_use: name, args: {"key": "value"}]
//
// 返回提取的 ToolCall 列表以及去除工具调用和关联特殊 token 后的内容。
//
// 用于处理 kimi-k2 等模型将工具调用以文本形式写在 content 字段、
// 而非使用标准 tool_calls 响应字段的情况。
func ExtractInlineToolCalls(content string) ([]ToolCall, string) {
	if !strings.Contains(content, inlineToolUseMarker) {
		return nil, content
	}

	var toolCalls []ToolCall
	var cleanContent strings.Builder
	pos := 0

	for pos < len(content) {
		idx := strings.Index(content[pos:], inlineToolUseMarker)
		if idx == -1 {
			cleanContent.WriteString(content[pos:])
			break
		}
		absIdx := pos + idx

		// 写入工具调用标记之前的文本内容
		cleanContent.WriteString(content[pos:absIdx])

		// 尝试从 absIdx 处解析内联工具调用
		tc, endPos, ok := parseInlineToolCall(content, absIdx)
		if !ok {
			// 不是有效的内联工具调用；写入标记文本并继续
			cleanContent.WriteString(content[absIdx : absIdx+len(inlineToolUseMarker)])
			pos = absIdx + len(inlineToolUseMarker)
			continue
		}

		toolCalls = append(toolCalls, tc)
		pos = endPos
	}

	cleaned := stripInlineToolTokens(cleanContent.String())
	return toolCalls, cleaned
}

// parseInlineToolCall 从 content 的 start 位置解析单个内联工具调用。
// content[start] 必须以 inlineToolUseMarker 开头。
// 返回解析后的 ToolCall、工具调用结束位置、以及是否解析成功。
func parseInlineToolCall(content string, start int) (ToolCall, int, bool) {
	// 格式：[tool_use: NAME, args: JSON_OBJECT]
	afterMarker := content[start+len(inlineToolUseMarker):]

	// 查找参数分隔符
	argsIdx := strings.Index(afterMarker, inlineToolArgsSeparator)
	if argsIdx == -1 {
		return ToolCall{}, start, false
	}

	name := strings.TrimSpace(afterMarker[:argsIdx])
	if name == "" {
		return ToolCall{}, start, false
	}

	afterArgs := afterMarker[argsIdx+len(inlineToolArgsSeparator):]

	// 查找 JSON 对象起始位置
	jsonStart := strings.Index(afterArgs, "{")
	if jsonStart == -1 {
		return ToolCall{}, start, false
	}

	// 查找与起始花括号匹配的结束花括号
	jsonEnd := findMatchingBraceInString(afterArgs, jsonStart)
	if jsonEnd == -1 {
		return ToolCall{}, start, false
	}

	jsonStr := afterArgs[jsonStart : jsonEnd+1]

	// 解析 JSON 参数
	var args map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &args); err != nil {
		log.Printf("common: 解析内联工具调用参数失败，工具 %q: %v", name, err)
		return ToolCall{}, start, false
	}

	// 跳过 [tool_use: ...] 的结束方括号 ']'
	remaining := afterArgs[jsonEnd+1:]
	closeBracket := strings.Index(remaining, "]")
	var endPos int
	if closeBracket != -1 {
		endPos = start + len(inlineToolUseMarker) + len(afterMarker[:argsIdx]) +
			len(inlineToolArgsSeparator) + len(afterArgs[:jsonEnd+1]) + closeBracket + 1
	} else {
		// 没有结束方括号；消耗到 JSON 对象末尾
		endPos = start + len(inlineToolUseMarker) + len(afterMarker[:argsIdx]) +
			len(inlineToolArgsSeparator) + len(afterArgs[:jsonEnd+1])
	}

	argsJSON := jsonStr
	return ToolCall{
		ID:        fmt.Sprintf("inline_%s_%d", name, start),
		Type:      "function",
		Name:      name,
		Arguments: args,
		Function: &FunctionCall{
			Name:      name,
			Arguments: argsJSON,
		},
	}, endPos, true
}

// findMatchingBraceInString 从 s 的 start 位置开始，查找与起始花括号匹配的
// 结束花括号位置。正确处理字符串内的花括号和转义字符。
// 未找到匹配时返回 -1。
func findMatchingBraceInString(s string, start int) int {
	if start >= len(s) || s[start] != '{' {
		return -1
	}

	depth := 0
	inString := false
	escape := false

	for i := start; i < len(s); i++ {
		ch := s[i]

		if escape {
			escape = false
			continue
		}

		if ch == '\\' && inString {
			escape = true
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// stripInlineToolTokens 清除与内联工具调用关联的特殊 token（如
// <|tool_call_end|>、<|tool_calls_section_end|>），并清理周围的空白和
// Anthropic 风格包装模式。
func stripInlineToolTokens(content string) string {
	// 移除特殊 token
	content = strings.ReplaceAll(content, inlineToolCallEndToken, "")
	content = strings.ReplaceAll(content, inlineToolCallsSectionEndToken, "")

	// 移除 kimi-k2 等模型在工具调用外层附加的 Anthropic 风格包装
	content = stripAnthropicStyleWrapper(content)

	return strings.TrimSpace(content)
}

// stripAnthropicStyleWrapper 移除内容中空文本的 Anthropic 风格包装模式，
// 如 [{'type': 'text', 'text': ”}]。
func stripAnthropicStyleWrapper(content string) string {
	// 模式：[{'type': 'text', 'text': ''}]（空文本包装）
	// kimi-k2 在工具调用输出外层附加这种 Anthropic 风格的内容块
	trimmed := strings.TrimSpace(content)

	// 检查整个内容是否为空文本的 Anthropic 风格包装
	// 如 "[{'type': 'text', 'text': ''}]"
	if isEmptyAnthropicWrapper(trimmed) {
		return ""
	}

	return content
}

// isEmptyAnthropicWrapper 检查内容是否为文本值为空的 Anthropic 风格包装，
// 如 [{'type': 'text', 'text': ”}]
func isEmptyAnthropicWrapper(content string) bool {
	if len(content) == 0 {
		return false
	}

	// 必须以 [{' 开头、以 '}]' 结尾
	if !strings.HasPrefix(content, "[{") || !strings.HasSuffix(content, "}]") {
		return false
	}

	// 必须包含 'type' 和 'text' 键
	if !strings.Contains(content, "'type'") || !strings.Contains(content, "'text'") {
		return false
	}

	// 提取 'text': ' 和 '} 之间的文本值
	textIdx := strings.Index(content, "'text': '")
	if textIdx == -1 {
		// 尝试双引号格式："text": "..."
		textIdx = strings.Index(content, `"text": "`)
		if textIdx == -1 {
			return false
		}
	}

	// 文本值为空或仅空白则判定为空包装
	inner := content[1 : len(content)-1] // 去除外层 [ 和 ]
	// 查找文本值
	if idx := strings.Index(inner, "'text': '"); idx != -1 {
		start := idx + len("'text': '")
		end := strings.Index(inner[start:], "'}")
		if end != -1 {
			textValue := inner[start : start+end]
			return strings.TrimSpace(textValue) == ""
		}
	}
	// 尝试双引号格式："text": "..."
	if idx := strings.Index(inner, `"text": "`); idx != -1 {
		start := idx + len(`"text": "`)
		end := strings.Index(inner[start:], `"}`)
		if end != -1 {
			textValue := inner[start : start+end]
			return strings.TrimSpace(textValue) == ""
		}
	}

	return false
}
