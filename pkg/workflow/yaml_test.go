package workflow

import (
	"strings"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func TestPromptUsesLiteralStyle(t *testing.T) {
	wf := &Workflow{
		Name: "test",
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "第一行\n第二行\n第三行", SendTools: boolPtr(true)},
			{ID: "s2", Action: "agent_prompt", Prompt: "纯文本", SendTools: boolPtr(false)},
			{ID: "s3", Action: "notify", Message: "通知\n换行"},
			{ID: "s4", Action: "tool_call", Tool: "x"},
		},
	}
	data, _ := renderYAMLWorkflow(wf)
	out := string(data)
	t.Log(out)

	if !strings.Contains(out, "prompt: |-") {
		t.Fatal("需要 prompt: |-")
	}
	if strings.Contains(out, "\\n") {
		t.Fatal("不应有 \\n")
	}
	if !strings.Contains(out, "send_tools: true") {
		t.Fatal("需要 send_tools: true")
	}
	if !strings.Contains(out, "send_tools: false") {
		t.Fatal("需要 send_tools: false")
	}
}

func TestEmojiNotEscaped(t *testing.T) {
	wf := &Workflow{
		Name: "emoji",
		Steps: []Step{
			{ID: "e1", Action: "agent_prompt", Prompt: "🌍 三、标题\n📝 四、内容"},
		},
	}
	data, _ := renderYAMLWorkflow(wf)
	out := string(data)
	t.Log(out)

	if !strings.Contains(out, "prompt: |-") {
		t.Fatal("需要 |- 格式")
	}
	if strings.Contains(out, `\U000`) {
		t.Fatal("emoji 不应被转义为 \\U0001F30D")
	}
}
