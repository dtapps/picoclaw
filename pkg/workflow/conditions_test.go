package workflow

import "testing"

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
