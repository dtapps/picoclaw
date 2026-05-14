package evolution

import (
	"fmt"
	"os"
	"strings"

	"github.com/sipeed/picoclaw/pkg/skills"
)

const (
	maxMatchedSkillExcerptCount = 5
	maxMatchedSkillExcerptChars = 1400
	maxComponentGuidanceChars   = 520
)

type matchedSkillExcerpt struct {
	Name        string
	Description string
	Body        string
}

func loadMatchedSkillExcerpts(matches []skills.SkillInfo) []matchedSkillExcerpt {
	excerpts := make([]matchedSkillExcerpt, 0, minInt(len(matches), maxMatchedSkillExcerptCount))
	for _, match := range matches {
		if len(excerpts) >= maxMatchedSkillExcerptCount {
			break
		}
		body := readSkillBodyExcerpt(match.Path)
		if body == "" {
			continue
		}
		excerpts = append(excerpts, matchedSkillExcerpt{
			Name:        strings.TrimSpace(match.Name),
			Description: strings.TrimSpace(match.Description),
			Body:        body,
		})
	}
	return excerpts
}

func readSkillBodyExcerpt(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	body := strings.TrimSpace(stripSkillFrontmatter(string(data)))
	if body == "" {
		return ""
	}
	body = strings.Join(strings.Fields(body), " ")
	if len(body) <= maxMatchedSkillExcerptChars {
		return body
	}
	return strings.TrimSpace(body[:maxMatchedSkillExcerptChars]) + "..."
}

func summarizeMatchedSkillExcerpts(matches []skills.SkillInfo) string {
	lang := os.Getenv("LANG")
	isZh := strings.HasPrefix(strings.ToLower(lang), "zh")

	excerpts := loadMatchedSkillExcerpts(matches)
	if len(excerpts) == 0 {
		if isZh {
			return "无"
		}
		return "none"
	}

	parts := make([]string, 0, len(excerpts))
	for _, excerpt := range excerpts {
		header := excerpt.Name
		if excerpt.Description != "" {
			header += ": " + excerpt.Description
		}
		parts = append(parts, fmt.Sprintf("### %s\n%s", header, excerpt.Body))
	}
	return strings.Join(parts, "\n\n")
}

func synthesizedComponentBreakdown(matches []skills.SkillInfo) string {
	lang := os.Getenv("LANG")
	isZh := strings.HasPrefix(strings.ToLower(lang), "zh")

	excerpts := loadMatchedSkillExcerpts(matches)
	if len(excerpts) == 0 {
		if isZh {
			return "- 生成此快捷方式时没有可用的组件技能内容。"
		}
		return "- No component skill content was available when this shortcut was generated."
	}

	lines := make([]string, 0, len(excerpts))
	for _, excerpt := range excerpts {
		guidance := conciseComponentGuidance(excerpt)
		if guidance == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- `%s`: %s", excerpt.Name, guidance))
	}
	if len(lines) == 0 {
		if isZh {
			return "- 组件技能内容可用，但无法提取简洁的指导。"
		}
		return "- Component skill content was available, but no concise guidance could be extracted."
	}
	return strings.Join(lines, "\n")
}

func conciseComponentGuidance(excerpt matchedSkillExcerpt) string {
	description := strings.TrimSpace(excerpt.Description)
	body := trimComponentGuidance(excerpt.Body)
	switch {
	case description != "" && body != "":
		return trimComponentGuidance(description + " " + body)
	case description != "":
		return trimComponentGuidance(description)
	default:
		return body
	}
}

func trimComponentGuidance(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	content = strings.NewReplacer(
		"#### ", "",
		"### ", "",
		"## ", "",
		"# ", "",
		"**", "",
	).Replace(content)
	content = strings.TrimSpace(content)
	return trimAtReadableBoundary(content, maxComponentGuidanceChars)
}
