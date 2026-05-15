package i18n

import (
	"embed"
	"os"
	"strings"
	"sync"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/text/language"
)

//go:embed locales/*.toml
var localeFS embed.FS

var (
	bundle    *i18n.Bundle
	localizer *i18n.Localizer
	once      sync.Once
	initErr   error
)

// Init 初始化 i18n 翻译包，加载嵌入的翻译文件。
// 应该在应用程序启动时调用一次。
// 自动从 LANG 环境变量检测语言，默认为英文。
func Init() error {
	once.Do(func() {
		bundle = i18n.NewBundle(language.English)
		bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

		// 加载英文翻译
		if _, err := bundle.LoadMessageFileFS(localeFS, "locales/en.toml"); err != nil {
			initErr = err
			return
		}

		// 加载中文翻译
		if _, err := bundle.LoadMessageFileFS(localeFS, "locales/zh.toml"); err != nil {
			initErr = err
			return
		}

		// 从环境变量检测语言，默认为英文
		lang := detectLanguage()
		localizer = i18n.NewLocalizer(bundle, lang)
	})
	return initErr
}

// detectLanguage 从 LANG 环境变量检测语言
// 返回 "zh" 表示中文，"en" 表示英文（默认）
func detectLanguage() string {
	lang := os.Getenv("LANG")
	if lang == "" {
		return "en"
	}

	// 转换为小写并检查中文标识
	lang = strings.ToLower(lang)
	if strings.HasPrefix(lang, "zh") {
		return "zh"
	}

	return "en"
}

// SetLanguage 设置当前本地化语言。
// 支持的语言："en" (英文), "zh" (中文)
func SetLanguage(lang string) {
	if bundle == nil {
		return
	}
	localizer = i18n.NewLocalizer(bundle, lang)
}

// ensureInitialized 确保 i18n 在使用前已初始化。
// 可以安全地多次调用。
func ensureInitialized() {
	if localizer == nil {
		_ = Init()
	}
}

// T 使用当前本地化器按 ID 翻译消息。
// 如果消息 ID 不存在，返回原消息 ID。
func T(messageID string) string {
	ensureInitialized()
	if localizer == nil {
		return messageID
	}
	result, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID: messageID,
	})
	if err != nil {
		return messageID
	}
	return result
}

// Tf 使用模板数据翻译消息。
// 如果消息 ID 不存在，返回原消息 ID。
func Tf(messageID string, templateData map[string]any) string {
	ensureInitialized()
	if localizer == nil {
		return messageID
	}
	result, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
	})
	if err != nil {
		return messageID
	}
	return result
}

// Tc 使用计数翻译消息，支持复数形式。
// 如果消息 ID 不存在，返回原消息 ID。
func Tc(messageID string, count int) string {
	ensureInitialized()
	if localizer == nil {
		return messageID
	}
	result, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:   messageID,
		PluralCount: count,
	})
	if err != nil {
		return messageID
	}
	return result
}

// Tfc 使用计数和模板数据翻译消息。
// 如果消息 ID 不存在，返回原消息 ID。
func Tfc(messageID string, count int, templateData map[string]any) string {
	ensureInitialized()
	if localizer == nil {
		return messageID
	}
	result, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		PluralCount:  count,
		TemplateData: templateData,
	})
	if err != nil {
		return messageID
	}
	return result
}
