package skills

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
)

func TestInjectEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected map[string]string
	}{
		{
			name: "inject enabled variables",
			cfg: &config.Config{
				EnvVars: config.EnvVarsConfig{
					Variables: []config.EnvVarEntry{
						{Key: "TEST_VAR1", Value: "value1", Enabled: true},
						{Key: "TEST_VAR2", Value: "value2", Enabled: true},
						{Key: "DISABLED_VAR", Value: "disabled", Enabled: false},
					},
				},
			},
			expected: map[string]string{
				"TEST_VAR1": "value1",
				"TEST_VAR2": "value2",
			},
		},
		{
			name:     "nil config",
			cfg:      nil,
			expected: map[string]string{
				// Should not panic, just return
			},
		},
		{
			name: "empty variables",
			cfg: &config.Config{
				EnvVars: config.EnvVarsConfig{
					Variables: []config.EnvVarEntry{},
				},
			},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("echo", "test")
			InjectEnvVars(cmd, tt.cfg)

			// Build env map from cmd.Env
			envMap := make(map[string]string)
			for _, e := range cmd.Env {
				if idx := strings.Index(e, "="); idx > 0 {
					envMap[e[:idx]] = e[idx+1:]
				}
			}

			// Check expected variables
			for key, expectedValue := range tt.expected {
				if actualValue, ok := envMap[key]; !ok {
					t.Errorf("Expected env var %s not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("For %s: expected %q, got %q", key, expectedValue, actualValue)
				}
			}

			// Ensure disabled variable is not present
			if tt.cfg != nil {
				for _, v := range tt.cfg.EnvVars.Variables {
					if !v.Enabled {
						if _, ok := envMap[v.Key]; ok {
							t.Errorf("Disabled variable %s should not be in env", v.Key)
						}
					}
				}
			}
		})
	}
}

func TestInjectEnvVars_WithEnvFile(t *testing.T) {
	// Create a temporary .env file
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "test.env")

	envContent := `FILE_VAR1=file_value1
FILE_VAR2=file_value2
SHARED_VAR=from_file`

	if err := os.WriteFile(envFile, []byte(envContent), 0o644); err != nil {
		t.Fatalf("Failed to create test env file: %v", err)
	}

	cfg := &config.Config{
		EnvVars: config.EnvVarsConfig{
			EnvFile: envFile,
			Variables: []config.EnvVarEntry{
				{Key: "CONFIG_VAR", Value: "config_value", Enabled: true},
				{Key: "SHARED_VAR", Value: "from_config", Enabled: true}, // Should override file
			},
		},
	}

	cmd := exec.Command("echo", "test")
	InjectEnvVars(cmd, cfg)

	// Build env map
	envMap := make(map[string]string)
	for _, e := range cmd.Env {
		if idx := strings.Index(e, "="); idx > 0 {
			envMap[e[:idx]] = e[idx+1:]
		}
	}

	// Verify file variables are loaded
	if envMap["FILE_VAR1"] != "file_value1" {
		t.Errorf("Expected FILE_VAR1=file_value1, got %s", envMap["FILE_VAR1"])
	}

	// Verify config variables are loaded
	if envMap["CONFIG_VAR"] != "config_value" {
		t.Errorf("Expected CONFIG_VAR=config_value, got %s", envMap["CONFIG_VAR"])
	}

	// Verify config overrides file
	if envMap["SHARED_VAR"] != "from_config" {
		t.Errorf("Expected SHARED_VAR=from_config (config should override file), got %s", envMap["SHARED_VAR"])
	}
}

func TestGetEnvVars(t *testing.T) {
	cfg := &config.Config{
		EnvVars: config.EnvVarsConfig{
			Variables: []config.EnvVarEntry{
				{Key: "MY_VAR", Value: "my_value", Enabled: true},
				{Key: "DISABLED", Value: "disabled", Enabled: false},
			},
		},
	}

	envVars := GetEnvVars(cfg)

	if envVars["MY_VAR"] != "my_value" {
		t.Errorf("Expected MY_VAR=my_value, got %s", envVars["MY_VAR"])
	}

	if _, ok := envVars["DISABLED"]; ok {
		t.Error("Disabled variable should not be in result")
	}

	// Should include parent environment
	if len(envVars) < 1 {
		t.Error("Expected at least one variable from parent env")
	}
}

func TestGetEnvVars_NilConfig(t *testing.T) {
	envVars := GetEnvVars(nil)
	if envVars != nil {
		t.Error("Expected nil for nil config")
	}
}

func TestEnvVarsIntegration(t *testing.T) {
	// This test verifies that environment variables are actually passed to a command
	cfg := &config.Config{
		EnvVars: config.EnvVarsConfig{
			Variables: []config.EnvVarEntry{
				{Key: "TEST_INTEGRATION", Value: "integration_value", Enabled: true},
			},
		},
	}

	cmd := exec.Command("sh", "-c", "echo $TEST_INTEGRATION")
	InjectEnvVars(cmd, cfg)

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "integration_value" {
		t.Errorf("Expected 'integration_value', got '%s'", result)
	}
}
