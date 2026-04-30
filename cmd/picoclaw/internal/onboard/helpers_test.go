package onboard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyEmbeddedToTargetUsesStructuredAgentFiles(t *testing.T) {
	targetDir := t.TempDir()

	if err := copyEmbeddedToTarget(targetDir); err != nil {
		t.Fatalf("copyEmbeddedToTarget() error = %v", err)
	}

	agentPath := filepath.Join(targetDir, "AGENT.md")
	if _, err := os.Stat(agentPath); err != nil {
		t.Fatalf("expected %s to exist: %v", agentPath, err)
	}

	soulPath := filepath.Join(targetDir, "SOUL.md")
	if _, err := os.Stat(soulPath); err != nil {
		t.Fatalf("expected %s to exist: %v", soulPath, err)
	}

	userPath := filepath.Join(targetDir, "USER.md")
	if _, err := os.Stat(userPath); err != nil {
		t.Fatalf("expected %s to exist: %v", userPath, err)
	}

	for _, legacyName := range []string{"AGENTS.md", "IDENTITY.md"} {
		legacyPath := filepath.Join(targetDir, legacyName)
		if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
			t.Fatalf("expected legacy file %s to be absent, got err=%v", legacyPath, err)
		}
	}
}

func TestCleanupExistingFiles_NonZh_RemovesZhFiles(t *testing.T) {
	targetDir := t.TempDir()

	// 创建测试文件
	os.WriteFile(filepath.Join(targetDir, "AGENT.md"), []byte("agent"), 0o644)
	os.WriteFile(filepath.Join(targetDir, "AGENT.zh.md"), []byte("agent zh"), 0o644)
	os.WriteFile(filepath.Join(targetDir, "SOUL.zh.md"), []byte("soul zh"), 0o644)

	// 非中文环境：删除 .zh.md 文件
	if err := cleanupExistingFiles(targetDir, false); err != nil {
		t.Fatalf("cleanupExistingFiles() error = %v", err)
	}

	// 验证 .md 文件存在
	if _, err := os.Stat(filepath.Join(targetDir, "AGENT.md")); err != nil {
		t.Fatalf("expected AGENT.md to exist: %v", err)
	}

	// 验证 .zh.md 文件被删除
	if _, err := os.Stat(filepath.Join(targetDir, "AGENT.zh.md")); !os.IsNotExist(err) {
		t.Fatalf("expected AGENT.zh.md to be removed")
	}
	if _, err := os.Stat(filepath.Join(targetDir, "SOUL.zh.md")); !os.IsNotExist(err) {
		t.Fatalf("expected SOUL.zh.md to be removed")
	}
}

func TestCleanupExistingFiles_Zh_RenamesZhFiles(t *testing.T) {
	targetDir := t.TempDir()

	// 创建测试文件
	os.WriteFile(filepath.Join(targetDir, "AGENT.md"), []byte("agent old"), 0o644)
	os.WriteFile(filepath.Join(targetDir, "AGENT.zh.md"), []byte("agent zh"), 0o644)

	// 中文环境：将 .zh.md 重命名为 .md
	if err := cleanupExistingFiles(targetDir, true); err != nil {
		t.Fatalf("cleanupExistingFiles() error = %v", err)
	}

	// 验证 .zh.md 文件被重命名（原 .md 被覆盖）
	if _, err := os.Stat(filepath.Join(targetDir, "AGENT.zh.md")); !os.IsNotExist(err) {
		t.Fatalf("expected AGENT.zh.md to be renamed")
	}

	// 验证新的 .md 文件内容是原 .zh.md 的内容
	content, err := os.ReadFile(filepath.Join(targetDir, "AGENT.md"))
	if err != nil {
		t.Fatalf("failed to read AGENT.md: %v", err)
	}
	if string(content) != "agent zh" {
		t.Fatalf("expected AGENT.md content to be 'agent zh', got %q", string(content))
	}
}

func TestCopyEmbeddedToTarget_ZhEnvironment_CreatesMdFiles(t *testing.T) {
	// 保存原始 LANG 环境变量
	origLang := os.Getenv("LANG")
	defer os.Setenv("LANG", origLang)

	// 设置为中文环境
	os.Setenv("LANG", "zh_CN.UTF-8")

	targetDir := t.TempDir()

	if err := copyEmbeddedToTarget(targetDir); err != nil {
		t.Fatalf("copyEmbeddedToTarget() error = %v", err)
	}

	// 验证 .md 文件被创建（从 .zh.md 复制并重命名）
	agentPath := filepath.Join(targetDir, "AGENT.md")
	if _, err := os.Stat(agentPath); err != nil {
		t.Fatalf("expected %s to exist: %v", agentPath, err)
	}

	soulPath := filepath.Join(targetDir, "SOUL.md")
	if _, err := os.Stat(soulPath); err != nil {
		t.Fatalf("expected %s to exist: %v", soulPath, err)
	}

	userPath := filepath.Join(targetDir, "USER.md")
	if _, err := os.Stat(userPath); err != nil {
		t.Fatalf("expected %s to exist: %v", userPath, err)
	}

	// 验证 .zh.md 文件不存在（只保留 .md）
	agentZhPath := filepath.Join(targetDir, "AGENT.zh.md")
	if _, err := os.Stat(agentZhPath); !os.IsNotExist(err) {
		t.Fatalf("expected %s to not exist in zh environment (should be renamed to .md)", agentZhPath)
	}
}
