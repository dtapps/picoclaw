package workflow

import (
	"encoding/json"
	"strings"
)

// EvaluateCondition 根据 when 子句判断步骤是否应该执行。
//
// 支持的条件类型：
//   - "" (空): 总是执行（默认行为）
//   - "on_success": 上一步成功时执行（等同于默认）
//   - "on_error": 上一步失败时执行
//   - "{{.step_id.key}} == value": 模板变量等值比较
func EvaluateCondition(when string, prevStepState *StepState, stepOutputs map[string]map[string]any) bool {
	if when == "" || when == "on_success" {
		// 默认：上一步成功或没有上一步时执行
		if prevStepState == nil {
			return true
		}
		return prevStepState.Status == StatusCompleted
	}

	if when == "on_error" {
		// 上一步失败时执行
		if prevStepState == nil {
			return false
		}
		return prevStepState.Status == StatusFailed
	}

	// 模板比较表达式
	return evaluateComparison(when, stepOutputs)
}

// evaluateComparison 处理 "{{.step_id.output_key}} == value" 形式的比较表达式。
// 先解析左边的模板引用，再与右边的值进行字符串等值比较。
func evaluateComparison(expr string, outputs map[string]map[string]any) bool {
	parts := strings.SplitN(expr, "==", 2)
	if len(parts) != 2 {
		return false
	}

	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])

	// 解析左边的模板引用
	resolved := resolveTemplate(left, outputs)
	return resolved == right
}

// resolveTemplate 解析单个 {{.step_id.key}} 或 {{.vars.key}} 模板引用。
// 从 stepOutputs 中查找对应步骤的输出值；"vars" 是工作流全局变量的特殊键。
// 如果引用无法解析（步骤不存在、键不存在），返回原始模板字符串。
func resolveTemplate(tmpl string, outputs map[string]map[string]any) string {
	// 检查是否是模板格式 {{.xxx.yyy}}
	if len(tmpl) < 5 || !strings.HasPrefix(tmpl, "{{.") || !strings.HasSuffix(tmpl, "}}") {
		return tmpl
	}

	// 提取路径：{{.step_id.key}} -> step_id.key
	inner := tmpl[3 : len(tmpl)-2]
	parts := strings.SplitN(inner, ".", 2)
	if len(parts) != 2 {
		return tmpl
	}

	stepID := parts[0]
	key := parts[1]

	stepOut, ok := outputs[stepID]
	if !ok {
		return tmpl
	}

	val, ok := stepOut[key]
	if !ok {
		return tmpl
	}

	return valueToString(val)
}

// ResolveStepTemplates 替换字符串中所有 {{.step_id.key}} 和 {{.vars.key}} 模板引用。
// 遍历输入字符串，找到每个 {{...}} 模式并替换为对应值。
func ResolveStepTemplates(input string, outputs map[string]map[string]any) string {
	var result strings.Builder
	i := 0
	for i < len(input) {
		// 查找下一个模板起始位置
		start := strings.Index(input[i:], "{{.")
		if start == -1 {
			result.WriteString(input[i:])
			break
		}
		// 写入模板前的普通文本
		result.WriteString(input[i : i+start])

		// 查找模板结束位置
		end := strings.Index(input[i+start:], "}}")
		if end == -1 {
			result.WriteString(input[i+start:])
			break
		}

		// 提取并解析模板
		tmpl := input[i+start : i+start+end+2]
		resolved := resolveTemplate(tmpl, outputs)
		result.WriteString(resolved)
		i = i + start + end + 2
	}
	return result.String()
}

// valueToString 将任意值转换为字符串用于条件比较。
func valueToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64, float32, int, int64, int32:
		b, _ := json.Marshal(val)
		return string(b)
	case nil:
		return ""
	default:
		b, _ := json.Marshal(val)
		return string(b)
	}
}
