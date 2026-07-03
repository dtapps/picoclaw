package yuanbao

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/fileutil"
)

func picoclawHomeDir() string {
	return config.GetHome()
}

func genYuanbaoAccountKey(cfg *config.YuanbaoSettings) string {
	AppID := strings.TrimSpace(cfg.AppID)
	if AppID == "" {
		return "default"
	}
	sum := sha256.Sum256([]byte(AppID))
	return hex.EncodeToString(sum[:8])
}

func buildYuanbaoTokensPath(cfg *config.YuanbaoSettings) string {
	return filepath.Join(
		picoclawHomeDir(),
		"channels",
		config.ChannelYuanbao,
		"tokens",
		genYuanbaoAccountKey(cfg)+".json",
	)
}

// yuanbaoTokenFile 定义 token 文件的 JSON 结构
type yuanbaoTokenFile struct {
	Token     string    `json:"token"`            // Token 值
	Name      string    `json:"name"`             // Token 名称/标识（使用 AppID）
	ExpiresAt time.Time `json:"expires_at"`       // 过期时间（可选）
	Weight    int       `json:"weight,omitempty"` // 权重，用于随机选择（默认 1）
}

// saveYuanbaoToken 保存 Yuanbao token 到文件
func saveYuanbaoToken(path string, appID string, token string, expiresIn int64) error {
	t := yuanbaoTokenFile{
		Name:  appID,
		Token: token,
	}
	// 仅在 expiresIn > 0 时设置过期时间
	if expiresIn > 0 {
		expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
		t.ExpiresAt = expiresAt
	}
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(path, data, 0o600)
}

// buildYuanbaoChatIDsPath 构建 group chatID 文件路径
func buildYuanbaoChatIDsPath(cfg *config.YuanbaoSettings) string {
	return filepath.Join(
		picoclawHomeDir(),
		"channels",
		config.ChannelYuanbao,
		"chatids",
		genYuanbaoAccountKey(cfg)+".json",
	)
}

// yuanbaoChatIDsFile 定义 chatID 文件的 JSON 结构
type yuanbaoChatIDsFile struct {
	GroupChatIDs []string `json:"group_chat_ids"` // Group 类型的 chatID 列表
}

// saveYuanbaoGroupChatID 保存 group chatID 到文件
func saveYuanbaoGroupChatID(path string, chatID string) error {
	// 读取现有文件
	var data yuanbaoChatIDsFile
	if _, err := os.Stat(path); err == nil {
		content, err := os.ReadFile(path)
		if err == nil {
			_ = json.Unmarshal(content, &data)
		}
	}

	// 检查是否已存在
	for _, id := range data.GroupChatIDs {
		if id == chatID {
			return nil // 已存在，无需保存
		}
	}

	// 添加新的 chatID
	data.GroupChatIDs = append(data.GroupChatIDs, chatID)

	// 保存到文件
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(path, jsonData, 0o600)
}

// loadYuanbaoGroupChatIDs 从文件加载 group chatID 列表
func loadYuanbaoGroupChatIDs(path string) ([]string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []string{}, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data yuanbaoChatIDsFile
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, err
	}

	return data.GroupChatIDs, nil
}
