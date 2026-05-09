package weibo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/fileutil"
)

func picoclawHomeDir() string {
	return config.GetHome()
}

func genWeiboAccountKey(cfg *config.WeiboSettings) string {
	AppID := strings.TrimSpace(cfg.AppID)
	if AppID == "" {
		return "default"
	}
	sum := sha256.Sum256([]byte(AppID))
	return hex.EncodeToString(sum[:8])
}

func buildWeiboTokensPath(cfg *config.WeiboSettings) string {
	return filepath.Join(
		picoclawHomeDir(),
		"channels",
		config.ChannelWeibo,
		"tokens",
		genWeiboAccountKey(cfg)+".json",
	)
}

// weiboTokenFile 定义 token 文件的 JSON 结构
type weiboTokenFile struct {
	Token     string    `json:"token"`            // Token 值
	Name      string    `json:"name"`             // Token 名称/标识（使用 AppID）
	ExpiresAt time.Time `json:"expires_at"`       // 过期时间（可选）
	Weight    int       `json:"weight,omitempty"` // 权重，用于随机选择（默认 1）
}

// saveWeiboToken 保存 Weibo token 到文件
func saveWeiboToken(path string, appID string, token string, expiresIn int64) error {
	t := weiboTokenFile{
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
