package evolution

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/sipeed/picoclaw/pkg/i18n"
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

	callCtx, cancel := withLLMCallTimeout(ctx, llmTaskSuccessJudgeTimeout)
	defer cancel()

	systemPrompt := i18n.T("success_judge_system_prompt")

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
	return i18n.Tf("success_judge_prompt", map[string]any{
		"Summary":        fallbackString(record.Summary, i18n.T("success_judge_none")),
		"FinalOutput":    fallbackString(record.FinalOutput, i18n.T("success_judge_none")),
		"UsedSkillNames": joinOrFallback(record.UsedSkillNames, i18n.T("success_judge_none")),
	})
}
