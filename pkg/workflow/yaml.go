package workflow

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseYAMLWorkflow 将 YAML 字节流反序列化为 Workflow 结构体。
func ParseYAMLWorkflow(data []byte) (*Workflow, error) {
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("解析工作流 YAML 失败: %w", err)
	}
	return &wf, nil
}

// renderYAMLWorkflow 将 Workflow 结构体序列化为 YAML 字节流。
// 后处理：prompt / message 字段含换行时转为 |- 块标量格式
func renderYAMLWorkflow(wf *Workflow) ([]byte, error) {
	data, err := yaml.Marshal(wf)
	if err != nil {
		return nil, fmt.Errorf("生成工作流 YAML 失败: %w", err)
	}
	data = fixBlockScalar(data)
	return data, nil
}

// fixBlockScalar 把 prompt:/message: 的 "...\n..." 转义格式转为 |-
var reBlockField = regexp.MustCompile(`(?m)^(\s*)(prompt|message):\s*"((?:[^"\\]|\\.)*)"$`)

func fixBlockScalar(data []byte) []byte {
	s := string(data)
	s = reBlockField.ReplaceAllStringFunc(s, func(match string) string {
		m := reBlockField.FindStringSubmatch(match)
		indent := m[1]
		field := m[2]
		quotedValue := m[3]

		content := unescape(quotedValue)
		if !strings.ContainsRune(content, '\n') {
			return match
		}

		lines := strings.Split(content, "\n")
		prefix := indent + "  "
		result := indent + field + ": |-\n"
		for _, line := range lines {
			result += prefix + line + "\n"
		}
		return result
	})
	return []byte(s)
}

// unescape 解析 yaml 双引号字符串中的转义序列：
// \n \t \" \\ 以及 \UXXXXXXXX (unicode emoji 等)
func unescape(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		switch s[i+1] {
		case 'n':
			b.WriteByte('\n')
			i++
		case 't':
			b.WriteByte('\t')
			i++
		case '"', '\\':
			b.WriteByte(s[i+1])
			i++
		case 'U': // \U0001F30D 格式的 unicode 转义
			if i+9 < len(s) {
				if codePoint, err := strconv.ParseInt(s[i+2:i+10], 16, 32); err == nil {
					b.WriteRune(rune(codePoint))
					i += 9
					break
				}
			}
			fallthrough
		default:
			b.WriteByte('\\')
		}
	}
	return b.String()
}
