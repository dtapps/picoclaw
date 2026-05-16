package workflow

import (
	"testing"
)

// TestMigrateToV2_SingleChannel 测试 v1 单频道配置迁移到 v2
func TestMigrateToV2_SingleChannel(t *testing.T) {
	wf := &Workflow{
		Name: "test-workflow",
		Config: WorkflowConfig{
			NotifyChannel: "dingtalk",
			NotifyChatID:  "cid123",
		},
	}

	// 执行迁移
	migrated := wf.MigrateToV2()

	if !migrated {
		t.Error("Expected migration to occur, but it didn't")
	}

	if wf.Version != 2 {
		t.Errorf("Expected version 2, got %d", wf.Version)
	}

	if len(wf.Config.NotifyChannels) != 1 {
		t.Fatalf("Expected 1 notify channel, got %d", len(wf.Config.NotifyChannels))
	}

	if wf.Config.NotifyChannels[0].Channel != "dingtalk" {
		t.Errorf("Expected channel 'dingtalk', got '%s'", wf.Config.NotifyChannels[0].Channel)
	}

	if wf.Config.NotifyChannels[0].ChatID != "cid123" {
		t.Errorf("Expected chat_id 'cid123', got '%s'", wf.Config.NotifyChannels[0].ChatID)
	}
}

// TestMigrateToV2_AlreadyV2 测试 v2 配置不需要迁移
func TestMigrateToV2_AlreadyV2(t *testing.T) {
	wf := &Workflow{
		Version: 2,
		Name:    "test-workflow",
		Config: WorkflowConfig{
			NotifyChannels: []NotifyTarget{
				{Channel: "dingtalk", ChatID: "cid123"},
			},
		},
	}

	// 执行迁移
	migrated := wf.MigrateToV2()

	if migrated {
		t.Error("Expected no migration for v2 workflow, but migration occurred")
	}

	if wf.Version != 2 {
		t.Errorf("Expected version 2, got %d", wf.Version)
	}
}

// TestMigrateToV2_MultiChannel 测试 v2 多频道配置保持不变
func TestMigrateToV2_MultiChannel(t *testing.T) {
	wf := &Workflow{
		Version: 2,
		Name:    "test-workflow",
		Config: WorkflowConfig{
			NotifyChannels: []NotifyTarget{
				{Channel: "dingtalk", ChatID: "cid1"},
				{Channel: "telegram", ChatID: "cid2"},
			},
		},
	}

	// 执行迁移
	migrated := wf.MigrateToV2()

	if migrated {
		t.Error("Expected no migration for v2 multi-channel workflow")
	}

	if len(wf.Config.NotifyChannels) != 2 {
		t.Errorf("Expected 2 notify channels, got %d", len(wf.Config.NotifyChannels))
	}
}

// TestMigrateToV2_NoOldFields 测试没有旧字段时不迁移
func TestMigrateToV2_NoOldFields(t *testing.T) {
	wf := &Workflow{
		Name:   "test-workflow",
		Config: WorkflowConfig{},
	}

	// 执行迁移
	migrated := wf.MigrateToV2()

	if migrated {
		t.Error("Expected no migration when no old fields present")
	}

	if wf.Version != 0 {
		t.Errorf("Expected version 0 (unset), got %d", wf.Version)
	}
}

// TestGetNotifyTargets_V1Format 测试从 v1 格式获取通知目标
func TestGetNotifyTargets_V1Format(t *testing.T) {
	config := &WorkflowConfig{
		NotifyChannel: "dingtalk",
		NotifyChatID:  "cid123",
	}

	targets := config.GetNotifyTargets()

	if len(targets) != 1 {
		t.Fatalf("Expected 1 target, got %d", len(targets))
	}

	if targets[0].Channel != "dingtalk" {
		t.Errorf("Expected channel 'dingtalk', got '%s'", targets[0].Channel)
	}

	if targets[0].ChatID != "cid123" {
		t.Errorf("Expected chat_id 'cid123', got '%s'", targets[0].ChatID)
	}
}

// TestGetNotifyTargets_V2Format 测试从 v2 格式获取通知目标
func TestGetNotifyTargets_V2Format(t *testing.T) {
	config := &WorkflowConfig{
		NotifyChannels: []NotifyTarget{
			{Channel: "dingtalk", ChatID: "cid1"},
			{Channel: "telegram", ChatID: "cid2"},
		},
	}

	targets := config.GetNotifyTargets()

	if len(targets) != 2 {
		t.Fatalf("Expected 2 targets, got %d", len(targets))
	}

	if targets[0].Channel != "dingtalk" {
		t.Errorf("Expected first channel 'dingtalk', got '%s'", targets[0].Channel)
	}

	if targets[1].Channel != "telegram" {
		t.Errorf("Expected second channel 'telegram', got '%s'", targets[1].Channel)
	}
}

// TestGetNotifyTargets_PreferV2 测试优先使用 v2 格式
func TestGetNotifyTargets_PreferV2(t *testing.T) {
	config := &WorkflowConfig{
		NotifyChannels: []NotifyTarget{
			{Channel: "dingtalk", ChatID: "cid-new"},
		},
		NotifyChannel: "telegram", // 旧字段
		NotifyChatID:  "cid-old",  // 旧字段
	}

	targets := config.GetNotifyTargets()

	if len(targets) != 1 {
		t.Fatalf("Expected 1 target (from v2 format), got %d", len(targets))
	}

	// 应该使用新格式，而不是旧字段
	if targets[0].Channel != "dingtalk" {
		t.Errorf("Expected channel 'dingtalk' (v2), got '%s'", targets[0].Channel)
	}

	if targets[0].ChatID != "cid-new" {
		t.Errorf("Expected chat_id 'cid-new' (v2), got '%s'", targets[0].ChatID)
	}
}
