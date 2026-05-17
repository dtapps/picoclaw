package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
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

// resolveTemplate 解析单个 {{.step_id.key}}、{{.vars.key}} 或 {{.fn.xxx}} 模板引用。
// 从 stepOutputs 中查找对应步骤的输出值；"vars" 是工作流全局变量的特殊键；
// "fn" 是内置模板函数（如 now、now_tz、date、date_tz、unix、env）。
// 如果引用无法解析（步骤不存在、键不存在），返回原始模板字符串。
func resolveTemplate(tmpl string, outputs map[string]map[string]any) string {
	if len(tmpl) < 5 || !strings.HasPrefix(tmpl, "{{.") || !strings.HasSuffix(tmpl, "}}") {
		return tmpl
	}

	inner := tmpl[3 : len(tmpl)-2]

	if strings.HasPrefix(inner, "fn.") {
		return resolveFuncTemplate(inner[3:], tmpl)
	}

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

// resolveFuncTemplate 解析内置模板函数调用。
// 支持的函数：
//   - now: 当前 UTC 时间，格式 "2006-01-02 15:04:05"
//   - now_tz "timezone": 指定时区的当前时间
//   - date: 当前 UTC 日期，格式 "2006-01-02"
//   - date_tz "timezone": 指定时区的当前日期
//   - unix: 当前 Unix 时间戳
//   - env "key": 获取环境变量值
//   - days_ago N: N 天前的日期，格式 "2006-01-02"
//   - days_from_now N: N 天后的日期，格式 "2006-01-02"
//   - hours_ago N: N 小时前的时间，格式 "2006-01-02 15:04:05"
//   - hours_from_now N: N 小时后的时间，格式 "2006-01-02 15:04:05"
//   - minutes_ago N: N 分钟前的时间，格式 "2006-01-02 15:04:05"
//   - minutes_from_now N: N 分钟后的时间，格式 "2006-01-02 15:04:05"
//   - weeks_ago N: N 周前的日期，格式 "2006-01-02"
//   - day_of_week: 当前星期几（1=Monday, 7=Sunday）
//   - format_time "format": 按指定格式格式化当前时间
func resolveFuncTemplate(funcExpr string, originalTmpl string) string {
	now := time.Now().UTC()

	if funcExpr == "now" {
		return now.Format("2006-01-02 15:04:05")
	}
	if funcExpr == "date" {
		return now.Format("2006-01-02")
	}
	if funcExpr == "unix" {
		return fmt.Sprintf("%d", now.Unix())
	}

	if strings.HasPrefix(funcExpr, "now_tz") {
		tz := extractQuotedArg(funcExpr[len("now_tz"):])
		if tz == "" {
			return now.Format("2006-01-02 15:04:05")
		}
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return originalTmpl
		}
		return now.In(loc).Format("2006-01-02 15:04:05")
	}

	if strings.HasPrefix(funcExpr, "date_tz") {
		tz := extractQuotedArg(funcExpr[len("date_tz"):])
		if tz == "" {
			return now.Format("2006-01-02")
		}
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return originalTmpl
		}
		return now.In(loc).Format("2006-01-02")
	}

	if strings.HasPrefix(funcExpr, "env") {
		key := extractQuotedArg(funcExpr[len("env"):])
		if key == "" {
			return originalTmpl
		}
		val, ok := os.LookupEnv(key)
		if !ok {
			return originalTmpl
		}
		return val
	}

	// 时间计算函数：days_ago, days_from_now
	if strings.HasPrefix(funcExpr, "days_ago ") {
		n := parseNumberArg(funcExpr[len("days_ago "):])
		if n < 0 {
			return originalTmpl
		}
		target := now.AddDate(0, 0, -n)
		return target.Format("2006-01-02")
	}
	if strings.HasPrefix(funcExpr, "days_from_now ") {
		n := parseNumberArg(funcExpr[len("days_from_now "):])
		if n < 0 {
			return originalTmpl
		}
		target := now.AddDate(0, 0, n)
		return target.Format("2006-01-02")
	}

	// 时间计算函数：hours_ago, hours_from_now
	if strings.HasPrefix(funcExpr, "hours_ago ") {
		n := parseNumberArg(funcExpr[len("hours_ago "):])
		if n < 0 {
			return originalTmpl
		}
		target := now.Add(-time.Duration(n) * time.Hour)
		return target.Format("2006-01-02 15:04:05")
	}
	if strings.HasPrefix(funcExpr, "hours_from_now ") {
		n := parseNumberArg(funcExpr[len("hours_from_now "):])
		if n < 0 {
			return originalTmpl
		}
		target := now.Add(time.Duration(n) * time.Hour)
		return target.Format("2006-01-02 15:04:05")
	}

	// 时间计算函数：minutes_ago, minutes_from_now
	if strings.HasPrefix(funcExpr, "minutes_ago ") {
		n := parseNumberArg(funcExpr[len("minutes_ago "):])
		if n < 0 {
			return originalTmpl
		}
		target := now.Add(-time.Duration(n) * time.Minute)
		return target.Format("2006-01-02 15:04:05")
	}
	if strings.HasPrefix(funcExpr, "minutes_from_now ") {
		n := parseNumberArg(funcExpr[len("minutes_from_now "):])
		if n < 0 {
			return originalTmpl
		}
		target := now.Add(time.Duration(n) * time.Minute)
		return target.Format("2006-01-02 15:04:05")
	}

	// 时间计算函数：weeks_ago
	if strings.HasPrefix(funcExpr, "weeks_ago ") {
		n := parseNumberArg(funcExpr[len("weeks_ago "):])
		if n < 0 {
			return originalTmpl
		}
		target := now.AddDate(0, 0, -n*7)
		return target.Format("2006-01-02")
	}

	// day_of_week: 返回星期几（1=Monday, 7=Sunday）
	if funcExpr == "day_of_week" {
		return fmt.Sprintf("%d", now.Weekday()+1)
	}

	// format_time: 自定义格式
	if strings.HasPrefix(funcExpr, "format_time ") {
		format := extractQuotedArg(funcExpr[len("format_time "):])
		if format == "" {
			return originalTmpl
		}
		return now.Format(format)
	}

	return originalTmpl
}

// extractQuotedArg 从函数参数表达式中提取引号内的字符串。
// 例如：` "Asia/Shanghai"` → `Asia/Shanghai`
func extractQuotedArg(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return ""
	}
	if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return ""
}

// parseNumberArg 从函敳参数表达式中提取数字。
// 例如：` 7` → `7`, ` 30` → `30`
func parseNumberArg(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return -1
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
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
