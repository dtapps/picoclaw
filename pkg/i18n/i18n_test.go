package i18n

import (
	"os"
	"strings"
	"sync"
	"testing"
)

// TestInit 测试 i18n 初始化
func TestInit(t *testing.T) {
	// 重置状态以允许重新初始化
	bundle = nil
	localizer = nil
	once = sync.Once{}
	initErr = nil

	err := Init()
	if err != nil {
		t.Errorf("Init() 失败: %v", err)
	}

	// 验证初始化后 localizer 不为 nil
	if localizer == nil {
		t.Error("Init() 后 localizer 为 nil")
	}
}

// TestT 测试基本翻译功能
func TestT(t *testing.T) {
	// 确保已初始化
	if localizer == nil {
		err := Init()
		if err != nil {
			t.Fatalf("Init() 失败: %v", err)
		}
	}

	// 测试英文翻译
	SetLanguage("en")
	result := T("commands_start_description")
	if result != "Start the bot" {
		t.Errorf("英文翻译失败: 期望 'Start the bot', 得到 '%s'", result)
	}

	// 测试中文翻译
	SetLanguage("zh")
	result = T("commands_start_description")
	if result != "启动机器人" {
		t.Errorf("中文翻译失败: 期望 '启动机器人', 得到 '%s'", result)
	}
}

// TestTf 测试带模板数据的翻译
func TestTf(t *testing.T) {
	if localizer == nil {
		err := Init()
		if err != nil {
			t.Fatalf("Init() 失败: %v", err)
		}
	}

	SetLanguage("en")
	result := Tf("commands_switch_model_success", map[string]any{
		"OldModel": "gpt-3.5",
		"NewModel": "gpt-4",
	})
	expected := "Switched model from gpt-3.5 to gpt-4"
	if result != expected {
		t.Errorf("模板翻译失败: 期望 '%s', 得到 '%s'", expected, result)
	}
}

// TestDetectLanguage 测试语言检测功能
func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		langEnv  string
		expected string
	}{
		{"中文环境", "zh_CN.UTF-8", "zh"},
		{"中文环境（小写）", "zh_cn.utf-8", "zh"},
		{"中文环境（简写）", "zh", "zh"},
		{"中文环境（台湾）", "zh_TW.UTF-8", "zh"},
		{"英文环境", "en_US.UTF-8", "en"},
		{"英文环境（简写）", "en", "en"},
		{"德文环境（默认英文）", "de_DE.UTF-8", "en"},
		{"空环境变量", "", "en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 保存原始值
			originalLang := os.Getenv("LANG")
			defer os.Setenv("LANG", originalLang)

			// 设置测试值
			os.Setenv("LANG", tt.langEnv)

			result := detectLanguage()
			if result != tt.expected {
				t.Errorf("detectLanguage() = %s, 期望 %s", result, tt.expected)
			}
		})
	}
}

// TestSetLanguage 测试手动设置语言
func TestSetLanguage(t *testing.T) {
	if localizer == nil {
		err := Init()
		if err != nil {
			t.Fatalf("Init() 失败: %v", err)
		}
	}

	// 设置为中文并验证
	SetLanguage("zh")
	result := T("commands_start_description")
	if !strings.Contains(result, "启动") {
		t.Errorf("SetLanguage('zh') 失败: 得到 '%s'", result)
	}

	// 设置为英文并验证
	SetLanguage("en")
	result = T("commands_start_description")
	if result != "Start the bot" {
		t.Errorf("SetLanguage('en') 失败: 得到 '%s'", result)
	}
}

// TestEnsureInitialized 测试自动初始化
func TestEnsureInitialized(t *testing.T) {
	// 重置状态
	bundle = nil
	localizer = nil
	once = sync.Once{}
	initErr = nil

	// 直接调用 T()，应该自动初始化
	result := T("commands_start_description")
	if result == "" || result == "commands_start_description" {
		t.Error("ensureInitialized() 没有正确自动初始化")
	}
}

// TestT_UnknownMessageID 测试未知消息 ID
func TestT_UnknownMessageID(t *testing.T) {
	if localizer == nil {
		err := Init()
		if err != nil {
			t.Fatalf("Init() 失败: %v", err)
		}
	}

	// 未知的消息 ID 应该返回消息 ID 本身
	unknownID := "unknown_message_id_xyz"
	result := T(unknownID)
	if result != unknownID {
		t.Errorf("未知消息 ID 应该返回原字符串: 期望 '%s', 得到 '%s'", unknownID, result)
	}
}
