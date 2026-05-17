package workflow

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEvaluateCondition(t *testing.T) {
	tests := []struct {
		name          string
		when          string
		prevStepState *StepState
		stepOutputs   map[string]map[string]any
		want          bool
	}{
		{
			name:          "empty when with nil prev always true",
			when:          "",
			prevStepState: nil,
			want:          true,
		},
		{
			name:          "empty when with completed prev",
			when:          "",
			prevStepState: &StepState{Status: StatusCompleted},
			want:          true,
		},
		{
			name:          "empty when with failed prev",
			when:          "",
			prevStepState: &StepState{Status: StatusFailed},
			want:          false,
		},
		{
			name:          "on_success with completed prev",
			when:          "on_success",
			prevStepState: &StepState{Status: StatusCompleted},
			want:          true,
		},
		{
			name:          "on_success with failed prev",
			when:          "on_success",
			prevStepState: &StepState{Status: StatusFailed},
			want:          false,
		},
		{
			name:          "on_error with failed prev",
			when:          "on_error",
			prevStepState: &StepState{Status: StatusFailed},
			want:          true,
		},
		{
			name:          "on_error with completed prev",
			when:          "on_error",
			prevStepState: &StepState{Status: StatusCompleted},
			want:          false,
		},
		{
			name:          "on_error with nil prev",
			when:          "on_error",
			prevStepState: nil,
			want:          false,
		},
		{
			name: "template comparison match",
			when: "{{.check.status}} == ok",
			stepOutputs: map[string]map[string]any{
				"check": {"status": "ok"},
			},
			want: true,
		},
		{
			name: "template comparison no match",
			when: "{{.check.status}} == fail",
			stepOutputs: map[string]map[string]any{
				"check": {"status": "ok"},
			},
			want: false,
		},
		{
			name:        "template comparison missing step",
			when:        "{{.missing.key}} == val",
			stepOutputs: map[string]map[string]any{},
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.when, tt.prevStepState, tt.stepOutputs)
			if got != tt.want {
				t.Fatalf("EvaluateCondition(%q, ...) = %v, want %v", tt.when, got, tt.want)
			}
		})
	}
}

func TestResolveTemplate(t *testing.T) {
	outputs := map[string]map[string]any{
		"fetch": {"weather": "sunny", "temp": 25},
		"vars":  {"project_dir": "/tmp/work", "site_url": "https://example.com"},
	}

	tests := []struct {
		name    string
		tmpl    string
		outputs map[string]map[string]any
		want    string
	}{
		{
			name:    "resolve step output",
			tmpl:    "{{.fetch.weather}}",
			outputs: outputs,
			want:    "sunny",
		},
		{
			name:    "resolve vars key",
			tmpl:    "{{.vars.project_dir}}",
			outputs: outputs,
			want:    "/tmp/work",
		},
		{
			name:    "non-template string",
			tmpl:    "plain text",
			outputs: outputs,
			want:    "plain text",
		},
		{
			name:    "missing step",
			tmpl:    "{{.missing.key}}",
			outputs: outputs,
			want:    "{{.missing.key}}",
		},
		{
			name:    "missing key in step",
			tmpl:    "{{.fetch.nonexistent}}",
			outputs: outputs,
			want:    "{{.fetch.nonexistent}}",
		},
		{
			name:    "too short for template",
			tmpl:    "{{.}}",
			outputs: outputs,
			want:    "{{.}}",
		},
		{
			name:    "malformed template no closing",
			tmpl:    "{{.fetch.weather",
			outputs: outputs,
			want:    "{{.fetch.weather",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveTemplate(tt.tmpl, tt.outputs)
			if got != tt.want {
				t.Fatalf("resolveTemplate(%q) = %q, want %q", tt.tmpl, got, tt.want)
			}
		})
	}
}

func TestResolveStepTemplates(t *testing.T) {
	outputs := map[string]map[string]any{
		"fetch": {"weather": "sunny"},
		"vars":  {"dir": "/tmp"},
	}

	tests := []struct {
		name    string
		input   string
		outputs map[string]map[string]any
		want    string
	}{
		{
			name:    "single template",
			input:   "Weather: {{.fetch.weather}}",
			outputs: outputs,
			want:    "Weather: sunny",
		},
		{
			name:    "multiple templates",
			input:   "{{.fetch.weather}} and {{.vars.dir}}",
			outputs: outputs,
			want:    "sunny and /tmp",
		},
		{
			name:    "no templates",
			input:   "no templates here",
			outputs: outputs,
			want:    "no templates here",
		},
		{
			name:    "unresolvable template kept",
			input:   "{{.missing.key}} + {{.fetch.weather}}",
			outputs: outputs,
			want:    "{{.missing.key}} + sunny",
		},
		{
			name:    "empty string",
			input:   "",
			outputs: outputs,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveStepTemplates(tt.input, tt.outputs)
			if got != tt.want {
				t.Fatalf("ResolveStepTemplates(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValueToString(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"string", "hello", "hello"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"int", 42, "42"},
		{"float64", 3.14, "3.14"},
		{"nil", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valueToString(tt.input)
			if got != tt.want {
				t.Fatalf("valueToString(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveFuncTemplate(t *testing.T) {
	now := time.Now().UTC()
	dateStr := now.Format("2006-01-02")
	timeStr := now.Format("2006-01-02")

	tests := []struct {
		name  string
		tmpl  string
		check func(t *testing.T, got string)
	}{
		{
			name: "fn.now returns UTC datetime",
			tmpl: "{{.fn.now}}",
			check: func(t *testing.T, got string) {
				if !strings.HasPrefix(got, dateStr) {
					t.Fatalf("fn.now = %q, expected to start with %q", got, dateStr)
				}
			},
		},
		{
			name: "fn.date returns UTC date",
			tmpl: "{{.fn.date}}",
			check: func(t *testing.T, got string) {
				if got != dateStr {
					t.Fatalf("fn.date = %q, want %q", got, dateStr)
				}
			},
		},
		{
			name: "fn.unix returns timestamp",
			tmpl: "{{.fn.unix}}",
			check: func(t *testing.T, got string) {
				if len(got) < 10 {
					t.Fatalf("fn.unix = %q, expected numeric timestamp", got)
				}
			},
		},
		{
			name: "fn.now_tz with timezone",
			tmpl: `{{.fn.now_tz "Asia/Shanghai"}}`,
			check: func(t *testing.T, got string) {
				shanghaiTime := now.In(time.FixedZone("CST", 8*3600))
				expected := shanghaiTime.Format("2006-01-02")
				if !strings.HasPrefix(got, expected) {
					t.Fatalf("fn.now_tz = %q, expected to start with %q", got, expected)
				}
			},
		},
		{
			name: "fn.date_tz with timezone",
			tmpl: `{{.fn.date_tz "Asia/Shanghai"}}`,
			check: func(t *testing.T, got string) {
				shanghaiTime := now.In(time.FixedZone("CST", 8*3600))
				expected := shanghaiTime.Format("2006-01-02")
				if got != expected {
					t.Fatalf("fn.date_tz = %q, want %q", got, expected)
				}
			},
		},
		{
			name: "fn.env with existing var",
			tmpl: "{{.fn.env \"HOME\"}}",
			check: func(t *testing.T, got string) {
				home := os.Getenv("HOME")
				if got != home {
					t.Fatalf("fn.env HOME = %q, want %q", got, home)
				}
			},
		},
		{
			name: "fn.env with non-existing var returns original",
			tmpl: `{{.fn.env "NONEXISTENT_VAR_XYZ_123"}}`,
			check: func(t *testing.T, got string) {
				if got != `{{.fn.env "NONEXISTENT_VAR_XYZ_123"}}` {
					t.Fatalf("fn.env non-existent = %q, want original template", got)
				}
			},
		},
		{
			name: "fn.now_tz without arg returns UTC",
			tmpl: "{{.fn.now_tz}}",
			check: func(t *testing.T, got string) {
				if !strings.HasPrefix(got, timeStr) {
					t.Fatalf("fn.now_tz (no arg) = %q, expected to start with %q", got, timeStr)
				}
			},
		},
		{
			name: "fn with invalid timezone returns original",
			tmpl: `{{.fn.now_tz "Invalid/Zone"}}`,
			check: func(t *testing.T, got string) {
				if got != `{{.fn.now_tz "Invalid/Zone"}}` {
					t.Fatalf("fn.now_tz invalid = %q, want original template", got)
				}
			},
		},
		{
			name: "fn with unknown function returns original",
			tmpl: "{{.fn.unknown_func}}",
			check: func(t *testing.T, got string) {
				if got != "{{.fn.unknown_func}}" {
					t.Fatalf("fn.unknown = %q, want original template", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveStepTemplates(tt.tmpl, nil)
			tt.check(t, got)
		})
	}
}

func TestResolveStepTemplatesWithFuncs(t *testing.T) {
	outputs := map[string]map[string]any{
		"fetch": {"weather": "sunny"},
		"vars":  {"dir": "/tmp"},
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "fn mixed with step ref",
			input: "Weather: {{.fetch.weather}}, Date: {{.fn.date}}",
			want:  "Weather: sunny, Date: " + time.Now().UTC().Format("2006-01-02"),
		},
		{
			name:  "fn only",
			input: "Today is {{.fn.date}}",
			want:  "Today is " + time.Now().UTC().Format("2006-01-02"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveStepTemplates(tt.input, outputs)
			if got != tt.want {
				t.Fatalf("ResolveStepTemplates(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestResolveStepTemplatesWithTimeFuncs 测试新增的时间函数
func TestResolveStepTemplatesWithTimeFuncs(t *testing.T) {
	outputs := map[string]map[string]any{}
	now := time.Now().UTC()

	tests := []struct {
		name     string
		input    string
		validate func(result string) bool
		desc     string
	}{
		{
			name:  "days_ago",
			input: "{{.fn.days_ago 7}}",
			validate: func(result string) bool {
				expected := now.AddDate(0, 0, -7).Format("2006-01-02")
				return result == expected
			},
			desc: "7天前应该是 " + now.AddDate(0, 0, -7).Format("2006-01-02"),
		},
		{
			name:  "days_from_now",
			input: "{{.fn.days_from_now 3}}",
			validate: func(result string) bool {
				expected := now.AddDate(0, 0, 3).Format("2006-01-02")
				return result == expected
			},
			desc: "3天后应该是 " + now.AddDate(0, 0, 3).Format("2006-01-02"),
		},
		{
			name:  "hours_ago",
			input: "{{.fn.hours_ago 24}}",
			validate: func(result string) bool {
				expected := now.Add(-24 * time.Hour).Format("2006-01-02 15:04:05")
				return result == expected
			},
			desc: "24小时前应该匹配",
		},
		{
			name:  "hours_from_now",
			input: "{{.fn.hours_from_now 2}}",
			validate: func(result string) bool {
				expected := now.Add(2 * time.Hour).Format("2006-01-02 15:04:05")
				return result == expected
			},
			desc: "2小时后应该匹配",
		},
		{
			name:  "minutes_ago",
			input: "{{.fn.minutes_ago 30}}",
			validate: func(result string) bool {
				expected := now.Add(-30 * time.Minute).Format("2006-01-02 15:04:05")
				return result == expected
			},
			desc: "30分钟前应该匹配",
		},
		{
			name:  "weeks_ago",
			input: "{{.fn.weeks_ago 2}}",
			validate: func(result string) bool {
				expected := now.AddDate(0, 0, -14).Format("2006-01-02")
				return result == expected
			},
			desc: "2周前应该是 " + now.AddDate(0, 0, -14).Format("2006-01-02"),
		},
		{
			name:  "day_of_week",
			input: "{{.fn.day_of_week}}",
			validate: func(result string) bool {
				expected := fmt.Sprintf("%d", now.Weekday()+1)
				return result == expected
			},
			desc: fmt.Sprintf("今天是星期%d", now.Weekday()+1),
		},
		{
			name:  "format_time",
			input: `{{.fn.format_time "2006/01/02"}}`,
			validate: func(result string) bool {
				expected := now.Format("2006/01/02")
				return result == expected
			},
			desc: "自定义格式应该正确",
		},
		{
			name:  "mixed_functions",
			input: "Today: {{.fn.date}}, 7 days ago: {{.fn.days_ago 7}}, Next week: {{.fn.days_from_now 7}}",
			validate: func(result string) bool {
				return strings.Contains(result, now.Format("2006-01-02")) &&
					strings.Contains(result, now.AddDate(0, 0, -7).Format("2006-01-02")) &&
					strings.Contains(result, now.AddDate(0, 0, 7).Format("2006-01-02"))
			},
			desc: "混合使用多个时间函数",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveStepTemplates(tt.input, outputs)
			if !tt.validate(got) {
				t.Errorf("ResolveStepTemplates(%q) = %q\n%s", tt.input, got, tt.desc)
			}
		})
	}
}
