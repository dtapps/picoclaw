package workflow

import (
	"strings"
	"testing"
)

func TestPromptUsesLiteralStyle(t *testing.T) {
	wf := &Workflow{
		Name: "test",
		Steps: []Step{
			{
				ID:     "s1",
				Action: "agent_prompt",
				Prompt: "第一行\n第二行\n第三行",
				Skills: StepTurnProfileBlock{Mode: "default"},
				Tools:  StepTurnProfileBlock{Mode: "default"},
			},
			{
				ID:     "s2",
				Action: "agent_prompt",
				Prompt: "纯文本",
				Skills: StepTurnProfileBlock{Mode: "off"},
				Tools:  StepTurnProfileBlock{Mode: "off"},
			},
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
	if !strings.Contains(out, "skills:") || !strings.Contains(out, "mode: default") {
		t.Fatal("需要 skills:\n  mode: default")
	}
	// off 是 YAML 布尔值关键字，所以会被加上引号
	if !strings.Contains(out, `mode: "off"`) {
		t.Fatal(`需要 mode: "off"`)
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
