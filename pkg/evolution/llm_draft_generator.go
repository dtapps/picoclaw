package evolution

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/sipeed/picoclaw/pkg/i18n"
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

	callCtx, cancel := withLLMCallTimeout(ctx, llmDraftGenerationTimeout)
	defer cancel()

	systemPrompt := i18n.T("llm_draft_system_prompt")

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
	return i18n.Tf("llm_draft_build_prompt", map[string]any{
		"Summary":              strings.TrimSpace(rule.Summary),
		"WinningPath":          joinOrFallback(rule.WinningPath, i18n.T("llm_draft_none")),
		"LateAddedSkills":      joinOrFallback(rule.LateAddedSkills, i18n.T("llm_draft_none")),
		"FinalSnapshotTrigger": fallbackString(rule.FinalSnapshotTrigger, i18n.T("llm_draft_none")),
		"EventCount":           rule.EventCount,
		"SuccessRate":          rule.SuccessRate,
		"SkillMatches":         summarizeSkillMatches(matches),
		"MatchedSkillNames":    joinOrFallback(rule.MatchedSkillNames, i18n.T("llm_draft_none")),
		"TaskEvidence":         summarizeDraftTaskEvidence(evidence),
		"SkillExcerpts":        summarizeMatchedSkillExcerpts(matches),
		"CombinedGuidance":     combinedSkillGuidance(rule),
		"DraftInstructions":    skillDraftPromptText(),
	})
}

func summarizeDraftTaskEvidence(evidence DraftEvidence) string {
	if len(evidence.TaskRecords) == 0 {
		return i18n.T("llm_draft_none")
	}

	labels := i18n.T("llm_draft_evidence_labels")
	labelParts := strings.Split(labels, "|")
	if len(labelParts) != 6 {
		labelParts = []string{"id", "summary", "final_output_excerpt", "used_skill_names", "unknown", "none"}
	}
	idLabel, summaryLabel, outputLabel, skillsLabel, unknownLabel, noneLabel := labelParts[0], labelParts[1], labelParts[2], labelParts[3], labelParts[4], labelParts[5]

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
	if target := inferCombinedSkillName(rule); target != "" {
		return i18n.Tf("llm_draft_combined_guidance", map[string]any{
			"Target": target,
		})
	}
	return i18n.T("llm_draft_single_guidance")
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
