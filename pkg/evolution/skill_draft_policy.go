package evolution

import (
	"strings"

	"github.com/sipeed/picoclaw/pkg/i18n"
)

func skillDraftPromptInstructions() []string {
	return strings.Split(i18n.T("skill_draft_instructions"), "\n")
}

func skillDraftPromptText() string {
	return strings.Join(skillDraftPromptInstructions(), "\n")
}

func learningTraceReplacer() *strings.Replacer {
	pairs := i18n.T("learning_trace_replacements")
	var replacements []string
	for _, line := range strings.Split(pairs, "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 {
			replacements = append(replacements, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
	return strings.NewReplacer(replacements...)
}

func renderDeployableSkillBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return body
	}
	frontmatter, markdownBody := splitSkillFrontmatter(body)
	if frontmatter != "" {
		body = "---\n" + frontmatter + "\n---\n" + learningTraceReplacer().Replace(strings.TrimLeft(markdownBody, "\n"))
	} else {
		body = learningTraceReplacer().Replace(body)
	}
	body = normalizeDeployableDescription(body)
	return removeDeployOnlyProvenanceLines(body)
}

func normalizeDeployableDescription(body string) string {
	lines := strings.Split(body, "\n")
	inFrontmatter := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if i == 0 && trimmed == "---" {
			inFrontmatter = true
			continue
		}
		if inFrontmatter && trimmed == "---" {
			break
		}
		if !inFrontmatter || !strings.HasPrefix(trimmed, "description:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
		value = cleanDeployableDescription(value)
		lines[i] = "description: " + value
		break
	}
	return strings.Join(lines, "\n")
}

func cleanDeployableDescription(description string) string {
	description = strings.TrimSpace(strings.Trim(description, `"'`))
	for _, marker := range []string{
		" for: ",
		" from learned pattern: ",
		" for learned pattern: ",
	} {
		if idx := strings.Index(strings.ToLower(description), marker); idx >= 0 {
			description = strings.TrimSpace(description[idx+len(marker):])
			break
		}
	}
	description = strings.TrimPrefix(description, "Create combined shortcut ")
	description = strings.TrimPrefix(description, "Refresh combined shortcut ")
	description = strings.TrimPrefix(description, "Create shortcut ")
	description = strings.TrimPrefix(description, "Refresh shortcut ")
	description = strings.TrimSpace(description)
	if description == "" {
		return "Use this skill when the task matches its documented workflow."
	}
	return description
}

func sentenceFragment(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "complete the documented workflow"
	}
	runes := []rune(text)
	if len(runes) > 0 && runes[0] >= 'A' && runes[0] <= 'Z' {
		runes[0] = runes[0] + ('a' - 'A')
	}
	return string(runes)
}

func trimAtReadableBoundary(content string, maxLen int) string {
	content = strings.TrimSpace(content)
	runes := []rune(content)
	if content == "" || maxLen <= 0 || len(runes) <= maxLen {
		return content
	}

	cut := maxLen
	searchStart := maxLen - minInt(maxLen/2, 240)
	if searchStart < 0 {
		searchStart = 0
	}
	for i := maxLen; i >= searchStart; i-- {
		switch runes[i-1] {
		case '\n', '.', '!', '?', ';', ':', '。', '！', '？', '；', '：':
			cut = i
			goto done
		}
	}
	for i := maxLen; i >= searchStart; i-- {
		if runes[i-1] == ' ' || runes[i-1] == '\t' {
			cut = i
			goto done
		}
	}

done:
	return strings.TrimRight(strings.TrimSpace(string(runes[:cut])), ".,;:，。；：") + "..."
}

func removeDeployOnlyProvenanceLines(body string) string {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "- evidence:") {
			continue
		}
		if strings.HasPrefix(lower, "- validated examples:") {
			continue
		}
		if strings.Contains(lower, "source_record_id") || strings.Contains(lower, "source record") {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
