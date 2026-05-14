package evolution

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/skills"
)

type LLMDraftGenerator struct {
	provider providers.LLMProvider
	model    string
	fallback DraftGenerator
}

type llmDraftResponse struct {
	TargetSkillName    string   `json:"target_skill_name"`
	DraftType          string   `json:"draft_type"`
	ChangeKind         string   `json:"change_kind"`
	HumanSummary       string   `json:"human_summary"`
	IntendedUseCases   []string `json:"intended_use_cases"`
	PreferredEntryPath []string `json:"preferred_entry_path"`
	AvoidPatterns      []string `json:"avoid_patterns"`
	BodyOrPatch        string   `json:"body_or_patch"`
}

func NewLLMDraftGenerator(provider providers.LLMProvider, model string, fallback DraftGenerator) *LLMDraftGenerator {
	return &LLMDraftGenerator{
		provider: provider,
		model:    strings.TrimSpace(model),
		fallback: fallback,
	}
}

func (g *LLMDraftGenerator) GenerateDraft(
	ctx context.Context,
	rule LearningRecord,
	matches []skills.SkillInfo,
) (SkillDraft, error) {
	return g.GenerateDraftWithEvidence(ctx, rule, matches, DraftEvidence{})
}

func (g *LLMDraftGenerator) GenerateDraftWithEvidence(
	ctx context.Context,
	rule LearningRecord,
	matches []skills.SkillInfo,
	evidence DraftEvidence,
) (SkillDraft, error) {
	rule = enrichRuleWithDraftEvidence(rule, evidence)
	if g == nil || g.provider == nil {
		return g.generateFallback(ctx, rule, matches, evidence)
	}

	model := g.model
	if model == "" {
		model = strings.TrimSpace(g.provider.GetDefaultModel())
	}
	if model == "" {
		return g.generateFallback(ctx, rule, matches, evidence)
	}

	lang := os.Getenv("LANG")
	isZh := strings.HasPrefix(strings.ToLower(lang), "zh")

	callCtx, cancel := withLLMCallTimeout(ctx, llmDraftGenerationTimeout)
	defer cancel()

	systemPrompt := "Return exactly one JSON object for a skill draft. Do not use markdown fences."
	if isZh {
		systemPrompt = "返回一个技能草稿的 JSON 对象。不要使用 markdown 代码块。"
	}

	resp, err := g.provider.Chat(callCtx, []providers.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: g.buildPrompt(rule, matches, evidence),
		},
	}, nil, model, map[string]any{"temperature": 0.2})
	if err != nil || resp == nil {
		return g.generateFallback(ctx, rule, matches, evidence)
	}

	content := strings.TrimSpace(resp.Content)
	if content == "" {
		return g.generateFallback(ctx, rule, matches, evidence)
	}

	draft, ok := parseLLMDraft(content)
	if !ok || len(ValidateDraft(draft)) > 0 {
		return g.generateFallback(ctx, rule, matches, evidence)
	}

	return draft, nil
}

func (g *LLMDraftGenerator) generateFallback(
	ctx context.Context,
	rule LearningRecord,
	matches []skills.SkillInfo,
	evidence DraftEvidence,
) (SkillDraft, error) {
	if g == nil || g.fallback == nil {
		return SkillDraft{}, nil
	}
	if generator, ok := g.fallback.(EvidenceAwareDraftGenerator); ok {
		return generator.GenerateDraftWithEvidence(ctx, rule, matches, evidence)
	}
	return g.fallback.GenerateDraft(ctx, rule, matches)
}

func (g *LLMDraftGenerator) buildPrompt(
	rule LearningRecord,
	matches []skills.SkillInfo,
	evidence DraftEvidence,
) string {
	lang := os.Getenv("LANG")
	isZh := strings.HasPrefix(strings.ToLower(lang), "zh")

	if isZh {
		return strings.Join([]string{
			"生成一个技能草稿 JSON 对象，包含以下必填字符串字段：",
			`target_skill_name, draft_type, change_kind, human_summary, body_or_patch.`,
			"可选数组字段: intended_use_cases, preferred_entry_path, avoid_patterns.",
			"",
			"允许的值:",
			"- draft_type: workflow | shortcut",
			"- change_kind: create | append | replace | merge",
			"- target_skill_name: 小写连字符格式的技能名称，描述功能用途; 不能仅为数字",
			"",
			"规则摘要: " + strings.TrimSpace(rule.Summary),
			"成功路径: " + joinOrFallback(rule.WinningPath, "无"),
			"后期添加的成功技能: " + joinOrFallback(rule.LateAddedSkills, "无"),
			"最终快照触发器: " + fallbackString(rule.FinalSnapshotTrigger, "无"),
			fmt.Sprintf("事件数量: %d", rule.EventCount),
			fmt.Sprintf("成功率: %.2f", rule.SuccessRate),
			"匹配的技能引用: " + summarizeSkillMatches(matches),
			"匹配的技能名称: " + joinOrFallback(rule.MatchedSkillNames, "无"),
			"源任务证据:",
			summarizeDraftTaskEvidence(evidence),
			"匹配的技能内容摘录:",
			summarizeMatchedSkillExcerpts(matches),
			"",
			combinedSkillGuidance(rule),
			skillDraftPromptText(),
		}, "\n")
	}

	return strings.Join([]string{
		"Generate a skill draft JSON object with these required string fields:",
		`target_skill_name, draft_type, change_kind, human_summary, body_or_patch.`,
		"Optional array fields: intended_use_cases, preferred_entry_path, avoid_patterns.",
		"",
		"Allowed values:",
		"- draft_type: workflow | shortcut",
		"- change_kind: create | append | replace | merge",
		"- target_skill_name: lowercase hyphenated skill name that describes the functional purpose; it must not be numeric-only",
		"",
		"Rule summary: " + strings.TrimSpace(rule.Summary),
		"Winning path: " + joinOrFallback(rule.WinningPath, "none"),
		"Late-added successful skills: " + joinOrFallback(rule.LateAddedSkills, "none"),
		"Final snapshot trigger: " + fallbackString(rule.FinalSnapshotTrigger, "none"),
		fmt.Sprintf("Event count: %d", rule.EventCount),
		fmt.Sprintf("Success rate: %.2f", rule.SuccessRate),
		"Matched skill refs: " + summarizeSkillMatches(matches),
		"Matched skill names: " + joinOrFallback(rule.MatchedSkillNames, "none"),
		"Source task evidence:",
		summarizeDraftTaskEvidence(evidence),
		"Matched skill content excerpts:",
		summarizeMatchedSkillExcerpts(matches),
		"",
		combinedSkillGuidance(rule),
		skillDraftPromptText(),
	}, "\n")
}

func summarizeDraftTaskEvidence(evidence DraftEvidence) string {
	lang := os.Getenv("LANG")
	isZh := strings.HasPrefix(strings.ToLower(lang), "zh")

	if len(evidence.TaskRecords) == 0 {
		if isZh {
			return "无"
		}
		return "none"
	}

	idLabel, summaryLabel, outputLabel, skillsLabel, unknownLabel, noneLabel := "id", "summary", "final_output_excerpt", "used_skill_names", "unknown", "none"
	if isZh {
		idLabel, summaryLabel, outputLabel, skillsLabel, unknownLabel, noneLabel = "ID", "摘要", "最终输出摘录", "使用的技能名称", "未知", "无"
	}

	lines := make([]string, 0, minInt(len(evidence.TaskRecords), 5))
	for i, task := range evidence.TaskRecords {
		if i >= 5 {
			break
		}
		parts := []string{
			"- " + idLabel + ": " + fallbackString(task.ID, unknownLabel),
			"  " + summaryLabel + ": " + fallbackString(task.Summary, noneLabel),
			"  " + outputLabel + ": " + fallbackString(summarizeText(task.FinalOutput, 700), noneLabel),
			"  " + skillsLabel + ": " + joinOrFallback(task.UsedSkillNames, noneLabel),
		}
		lines = append(lines, strings.Join(parts, "\n"))
	}
	return strings.Join(lines, "\n")
}

func combinedSkillGuidance(rule LearningRecord) string {
	lang := os.Getenv("LANG")
	isZh := strings.HasPrefix(strings.ToLower(lang), "zh")

	if target := inferCombinedSkillName(rule); target != "" {
		if isZh {
			return strings.Join([]string{
				"此规则代表一个稳定的多步骤成功路径。",
				"优先创建一个新的组合快捷技能，而不是修改单个组件技能。",
				"建议的目标技能名称: " + target,
			}, "\n")
		}
		return strings.Join([]string{
			"This rule represents a stable multi-step successful path.",
			"Prefer creating a new combined shortcut skill instead of modifying one component skill.",
			"Suggested target skill name: " + target,
		}, "\n")
	}
	if isZh {
		return "仅当学习到的模式明确属于单个技能时，才优先更新现有技能。"
	}
	return "Prefer updating an existing skill only when the learned pattern clearly belongs inside that single skill."
}

func parseLLMDraft(content string) (SkillDraft, bool) {
	normalized := strings.TrimSpace(content)
	normalized = strings.TrimPrefix(normalized, "```json")
	normalized = strings.TrimPrefix(normalized, "```")
	normalized = strings.TrimSuffix(normalized, "```")
	normalized = strings.TrimSpace(normalized)

	var payload llmDraftResponse
	if err := json.Unmarshal([]byte(normalized), &payload); err != nil {
		return SkillDraft{}, false
	}

	draft := SkillDraft{
		TargetSkillName:    strings.TrimSpace(payload.TargetSkillName),
		DraftType:          DraftType(strings.TrimSpace(payload.DraftType)),
		ChangeKind:         ChangeKind(strings.TrimSpace(payload.ChangeKind)),
		HumanSummary:       strings.TrimSpace(payload.HumanSummary),
		IntendedUseCases:   append([]string(nil), payload.IntendedUseCases...),
		PreferredEntryPath: append([]string(nil), payload.PreferredEntryPath...),
		AvoidPatterns:      append([]string(nil), payload.AvoidPatterns...),
		BodyOrPatch:        strings.TrimSpace(payload.BodyOrPatch),
	}
	return draft, true
}

func summarizeSkillMatches(matches []skills.SkillInfo) string {
	if len(matches) == 0 {
		return "none"
	}

	parts := make([]string, 0, len(matches))
	for _, match := range matches {
		part := strings.TrimSpace(match.Name)
		if desc := strings.TrimSpace(match.Description); desc != "" {
			part += ": " + desc
		}
		if path := strings.TrimSpace(match.Path); path != "" {
			part += " (" + path + ")"
		}
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, "; ")
}

func joinOrFallback(parts []string, fallback string) string {
	if len(parts) == 0 {
		return fallback
	}
	return strings.Join(parts, " -> ")
}

func fallbackString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
