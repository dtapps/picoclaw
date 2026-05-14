package evolution

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/sipeed/picoclaw/pkg/providers"
)

type TaskSuccessDecision struct {
	Success bool
	Reason  string
}

type SuccessJudge interface {
	JudgeTaskRecord(ctx context.Context, record LearningRecord) (TaskSuccessDecision, error)
}

type HeuristicSuccessJudge struct{}

func (j *HeuristicSuccessJudge) JudgeTaskRecord(
	_ context.Context,
	record LearningRecord,
) (TaskSuccessDecision, error) {
	if record.Success == nil || !*record.Success {
		return TaskSuccessDecision{Success: false, Reason: "task not completed"}, nil
	}
	if strings.TrimSpace(record.Summary) == "" {
		return TaskSuccessDecision{Success: false, Reason: "missing summary"}, nil
	}
	if strings.EqualFold(strings.TrimSpace(record.SessionKey), "heartbeat") {
		return TaskSuccessDecision{Success: false, Reason: "heartbeat session"}, nil
	}
	if strings.EqualFold(strings.TrimSpace(record.FinalOutput), "HEARTBEAT_OK") {
		return TaskSuccessDecision{Success: false, Reason: "heartbeat output"}, nil
	}
	if strings.TrimSpace(record.FinalOutput) == "" {
		return TaskSuccessDecision{Success: false, Reason: "missing final output"}, nil
	}
	return TaskSuccessDecision{Success: true, Reason: "heuristic success"}, nil
}

type LLMTaskSuccessJudge struct {
	provider providers.LLMProvider
	model    string
	fallback SuccessJudge
}

type llmTaskSuccessResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
}

func NewLLMTaskSuccessJudge(provider providers.LLMProvider, model string, fallback SuccessJudge) *LLMTaskSuccessJudge {
	if fallback == nil {
		fallback = &HeuristicSuccessJudge{}
	}
	return &LLMTaskSuccessJudge{
		provider: provider,
		model:    strings.TrimSpace(model),
		fallback: fallback,
	}
}

func (j *LLMTaskSuccessJudge) JudgeTaskRecord(
	ctx context.Context,
	record LearningRecord,
) (TaskSuccessDecision, error) {
	if j == nil || j.provider == nil {
		return j.fallbackDecision(ctx, record)
	}

	model := strings.TrimSpace(j.model)
	if model == "" {
		model = strings.TrimSpace(j.provider.GetDefaultModel())
	}
	if model == "" {
		return j.fallbackDecision(ctx, record)
	}

	lang := os.Getenv("LANG")
	isZh := strings.HasPrefix(strings.ToLower(lang), "zh")

	callCtx, cancel := withLLMCallTimeout(ctx, llmTaskSuccessJudgeTimeout)
	defer cancel()

	systemPrompt := "Return exactly one JSON object with fields success:boolean and reason:string. No markdown fences."
	if isZh {
		systemPrompt = "返回一个 JSON 对象，包含 success:boolean 和 reason:string 字段。不要使用 markdown 代码块。"
	}

	resp, err := j.provider.Chat(callCtx, []providers.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: buildTaskSuccessJudgePrompt(record),
		},
	}, nil, model, map[string]any{"temperature": 0})
	if err != nil || resp == nil {
		return j.fallbackDecision(ctx, record)
	}

	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	if content == "" {
		return j.fallbackDecision(ctx, record)
	}

	var payload llmTaskSuccessResponse
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return j.fallbackDecision(ctx, record)
	}
	return TaskSuccessDecision{
		Success: payload.Success,
		Reason:  strings.TrimSpace(payload.Reason),
	}, nil
}

func (j *LLMTaskSuccessJudge) fallbackDecision(
	ctx context.Context,
	record LearningRecord,
) (TaskSuccessDecision, error) {
	if j == nil || j.fallback == nil {
		return TaskSuccessDecision{Success: false, Reason: "no success judge available"}, nil
	}
	return j.fallback.JudgeTaskRecord(ctx, record)
}

func buildTaskSuccessJudgePrompt(record LearningRecord) string {
	lang := os.Getenv("LANG")
	isZh := strings.HasPrefix(strings.ToLower(lang), "zh")

	var lines []string
	if isZh {
		lines = []string{
			"判断此代理任务是否真正实现了用户的目标。",
			"拒绝那些只是部分推理、仅描述未来步骤或明显未完成请求结果的任务。",
			"当最终输出给出具体结果或具体完成的流程时，接受已完成的工作空间技能/定理任务。",
			"",
			"摘要: " + fallbackString(record.Summary, "无"),
			"最终输出: " + fallbackString(record.FinalOutput, "无"),
			"使用的技能: " + joinOrFallback(record.UsedSkillNames, "无"),
		}
	} else {
		lines = []string{
			"Decide whether this agent task truly achieved the user's goal.",
			"Reject tasks that are only partial reasoning, only describe future steps, or obviously did not complete the requested outcome.",
			"Accept completed custom workspace skill/theorem tasks when the final output gives a concrete result or concrete completed procedure.",
			"",
			"Summary: " + fallbackString(record.Summary, "none"),
			"Final output: " + fallbackString(record.FinalOutput, "none"),
			"Used skills: " + joinOrFallback(record.UsedSkillNames, "none"),
		}
	}
	return strings.Join(lines, "\n")
}
