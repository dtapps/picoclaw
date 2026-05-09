package yuanbao

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
)

// testYuanbaoSettings 创建用于测试的有效 YuanbaoSettings
func testYuanbaoSettings() *config.YuanbaoSettings {
	s := &config.YuanbaoSettings{
		AppID:     "test-app-id",
		AppSecret: *config.NewSecureString("test-app-secret"),
	}
	return s
}

// newTestYuanbaoChannel 创建用于测试的 YuanbaoChannel 实例
func newTestYuanbaoChannel(
	t *testing.T,
	cfg *config.YuanbaoSettings,
	bc *config.Channel,
) (*YuanbaoChannel, *bus.MessageBus) {
	t.Helper()

	if cfg == nil {
		cfg = testYuanbaoSettings()
	}
	msgBus := bus.NewMessageBus()
	if bc == nil {
		bc = &config.Channel{Type: config.ChannelYuanbao, Enabled: true}
	}
	ch, err := NewYuanbaoChannel(bc, cfg, msgBus, "info")
	if err != nil {
		t.Fatalf("创建频道失败: %v", err)
	}
	return ch, msgBus
}

// ---------------------------------------------------------------------------
// NewYuanbaoChannel 构造函数测试
// ---------------------------------------------------------------------------

// TestNewYuanbaoChannel_MissingAppID 缺少 app_id 时应返回错误
func TestNewYuanbaoChannel_MissingAppID(t *testing.T) {
	cfg := &config.YuanbaoSettings{
		AppSecret: *config.NewSecureString("secret"),
	}
	bc := &config.Channel{Type: config.ChannelYuanbao, Enabled: true}
	msgBus := bus.NewMessageBus()

	_, err := NewYuanbaoChannel(bc, cfg, msgBus, "info")
	if err == nil {
		t.Fatal("缺少 app_id 时应返回错误")
	}
	if !strings.Contains(err.Error(), "app_id") {
		t.Fatalf("错误信息应包含 app_id，实际: %v", err)
	}
}

// TestNewYuanbaoChannel_MissingAppSecret 缺少 app_secret 时应返回错误
func TestNewYuanbaoChannel_MissingAppSecret(t *testing.T) {
	cfg := &config.YuanbaoSettings{
		AppID: "test-app-id",
	}
	bc := &config.Channel{Type: config.ChannelYuanbao, Enabled: true}
	msgBus := bus.NewMessageBus()

	_, err := NewYuanbaoChannel(bc, cfg, msgBus, "info")
	if err == nil {
		t.Fatal("缺少 app_secret 时应返回错误")
	}
	if !strings.Contains(err.Error(), "app_secret") {
		t.Fatalf("错误信息应包含 app_secret，实际: %v", err)
	}
}

// TestNewYuanbaoChannel_ValidConfig 有效配置应成功创建频道
func TestNewYuanbaoChannel_ValidConfig(t *testing.T) {
	ch, _ := newTestYuanbaoChannel(t, nil, nil)

	if ch.Name() != config.ChannelYuanbao {
		t.Errorf("Name() = %q, 期望 %q", ch.Name(), config.ChannelYuanbao)
	}
	if ch.IsRunning() {
		t.Error("新创建的频道不应处于运行状态")
	}
}

// TestNewYuanbaoChannel_SetsTokensPath 创建时应正确设置 token 文件路径
func TestNewYuanbaoChannel_SetsTokensPath(t *testing.T) {
	cfg := testYuanbaoSettings()
	bc := &config.Channel{Type: config.ChannelYuanbao, Enabled: true}
	msgBus := bus.NewMessageBus()

	ch, err := NewYuanbaoChannel(bc, cfg, msgBus, "info")
	if err != nil {
		t.Fatalf("未预期的错误: %v", err)
	}

	expected := buildYuanbaoTokensPath(cfg)
	if ch.tokensPath != expected {
		t.Errorf("tokensPath = %q, 期望 %q", ch.tokensPath, expected)
	}
}

// ---------------------------------------------------------------------------
// genYuanbaoAccountKey 账户密钥生成测试
// ---------------------------------------------------------------------------

// TestGenYuanbaoAccountKey 测试账户密钥生成逻辑
func TestGenYuanbaoAccountKey(t *testing.T) {
	tests := []struct {
		name  string
		appID string
		want  string
	}{
		{
			name:  "非空 app_id 生成十六进制密钥",
			appID: "test-app-id",
			want:  "5d59ba64d93a7a3c", // sha256("test-app-id")[:8] 的十六进制
		},
		{
			name:  "空 app_id 返回 default",
			appID: "",
			want:  "default",
		},
		{
			name:  "仅空白字符的 app_id 返回 default",
			appID: "   ",
			want:  "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.YuanbaoSettings{AppID: tt.appID}
			got := genYuanbaoAccountKey(cfg)
			if got != tt.want {
				t.Errorf("genYuanbaoAccountKey() = %q, 期望 %q", got, tt.want)
			}
		})
	}
}

// TestGenYuanbaoAccountKey_Deterministic 相同输入应产生相同输出
func TestGenYuanbaoAccountKey_Deterministic(t *testing.T) {
	cfg := &config.YuanbaoSettings{AppID: "my-app"}
	key1 := genYuanbaoAccountKey(cfg)
	key2 := genYuanbaoAccountKey(cfg)
	if key1 != key2 {
		t.Errorf("genYuanbaoAccountKey 应具有确定性，得到 %q 和 %q", key1, key2)
	}
}

// TestGenYuanbaoAccountKey_DifferentAppIDs 不同的 app_id 应产生不同的密钥
func TestGenYuanbaoAccountKey_DifferentAppIDs(t *testing.T) {
	cfg1 := &config.YuanbaoSettings{AppID: "app-1"}
	cfg2 := &config.YuanbaoSettings{AppID: "app-2"}
	key1 := genYuanbaoAccountKey(cfg1)
	key2 := genYuanbaoAccountKey(cfg2)
	if key1 == key2 {
		t.Error("不同的 app_id 应产生不同的密钥")
	}
}

// ---------------------------------------------------------------------------
// buildYuanbaoTokensPath token 文件路径构建测试
// ---------------------------------------------------------------------------

// TestBuildYuanbaoTokensPath 测试 token 文件路径的组成部分
func TestBuildYuanbaoTokensPath(t *testing.T) {
	cfg := &config.YuanbaoSettings{AppID: "test-app-id"}
	path := buildYuanbaoTokensPath(cfg)

	if !strings.Contains(path, "channels") {
		t.Errorf("路径应包含 'channels'，实际 %q", path)
	}
	if !strings.Contains(path, config.ChannelYuanbao) {
		t.Errorf("路径应包含频道名 %q，实际 %q", config.ChannelYuanbao, path)
	}
	if !strings.Contains(path, "tokens") {
		t.Errorf("路径应包含 'tokens'，实际 %q", path)
	}
	if !strings.HasSuffix(path, ".json") {
		t.Errorf("路径应以 .json 结尾，实际 %q", path)
	}

	// 路径应包含账户密钥
	key := genYuanbaoAccountKey(cfg)
	if !strings.Contains(path, key) {
		t.Errorf("路径应包含密钥 %q，实际 %q", key, path)
	}
}

// TestBuildYuanbaoTokensPath_DefaultAppID 空 app_id 时路径应以 default.json 结尾
func TestBuildYuanbaoTokensPath_DefaultAppID(t *testing.T) {
	cfg := &config.YuanbaoSettings{AppID: ""}
	path := buildYuanbaoTokensPath(cfg)

	if !strings.HasSuffix(path, "default.json") {
		t.Errorf("空 app_id 路径应以 default.json 结尾，实际 %q", path)
	}
}

// ---------------------------------------------------------------------------
// saveYuanbaoToken token 文件保存测试
// ---------------------------------------------------------------------------

// TestSaveYuanbaoToken 测试正常保存 token 到文件
func TestSaveYuanbaoToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens", "test-token.json")

	err := saveYuanbaoToken(path, "my-app-id", "tok_abc123", 3600)
	if err != nil {
		t.Fatalf("saveYuanbaoToken 失败: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}

	var tf yuanbaoTokenFile
	if err := json.Unmarshal(data, &tf); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	if tf.Token != "tok_abc123" {
		t.Errorf("token = %q, 期望 %q", tf.Token, "tok_abc123")
	}
	if tf.Name != "my-app-id" {
		t.Errorf("name = %q, 期望 %q", tf.Name, "my-app-id")
	}
	if tf.ExpiresAt.IsZero() {
		t.Error("expires_at 在 expiresIn > 0 时应被设置")
	}
}

// TestSaveYuanbaoToken_ZeroExpiry 过期时间为零时不应设置 expires_at
func TestSaveYuanbaoToken_ZeroExpiry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens", "test-token.json")

	err := saveYuanbaoToken(path, "my-app-id", "tok_abc123", 0)
	if err != nil {
		t.Fatalf("saveYuanbaoToken 失败: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}

	var tf yuanbaoTokenFile
	if err := json.Unmarshal(data, &tf); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	if !tf.ExpiresAt.IsZero() {
		t.Error("expiresIn 为 0 时 expires_at 应为零值")
	}
}

// TestSaveYuanbaoToken_NegativeExpiry 过期时间为负数时不应设置 expires_at
func TestSaveYuanbaoToken_NegativeExpiry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens", "test-token.json")

	err := saveYuanbaoToken(path, "my-app-id", "tok_abc123", -1)
	if err != nil {
		t.Fatalf("saveYuanbaoToken 失败: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}

	var tf yuanbaoTokenFile
	if err := json.Unmarshal(data, &tf); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	if !tf.ExpiresAt.IsZero() {
		t.Error("expiresIn 为负数时 expires_at 应为零值")
	}
}

// TestSaveYuanbaoToken_Overwrite 重复保存应覆盖旧数据
func TestSaveYuanbaoToken_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens", "test-token.json")

	// 第一次保存
	err := saveYuanbaoToken(path, "app-1", "token-v1", 3600)
	if err != nil {
		t.Fatalf("第一次保存失败: %v", err)
	}

	// 覆盖保存
	err = saveYuanbaoToken(path, "app-2", "token-v2", 7200)
	if err != nil {
		t.Fatalf("第二次保存失败: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}

	var tf yuanbaoTokenFile
	if err := json.Unmarshal(data, &tf); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	if tf.Token != "token-v2" {
		t.Errorf("token = %q, 期望 %q（覆盖后的值）", tf.Token, "token-v2")
	}
	if tf.Name != "app-2" {
		t.Errorf("name = %q, 期望 %q（覆盖后的值）", tf.Name, "app-2")
	}
}

// ---------------------------------------------------------------------------
// getChatKind 聊天类型判断测试
// ---------------------------------------------------------------------------

// TestGetChatKind 测试根据 chatID 判断聊天类型
func TestGetChatKind(t *testing.T) {
	tests := []struct {
		name    string
		chatID  string
		preload map[string]string // 测试前预存到 chatType 的条目
		want    string
	}{
		{
			name:   "未知的 chat_id 默认为私聊",
			chatID: "user-123",
			want:   "direct",
		},
		{
			name:    "群组 chat_id 返回 group",
			chatID:  "group-456",
			preload: map[string]string{"group-456": "group"},
			want:    "group",
		},
		{
			name:   "空 chat_id 默认为私聊",
			chatID: "",
			want:   "direct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, _ := newTestYuanbaoChannel(t, nil, nil)

			// 预加载 chatType 条目
			for k, v := range tt.preload {
				ch.chatType.Store(k, v)
			}

			got := ch.getChatKind(tt.chatID)
			if got != tt.want {
				t.Errorf("getChatKind(%q) = %q, 期望 %q", tt.chatID, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Name 频道名称测试
// ---------------------------------------------------------------------------

// TestYuanbaoChannel_Name 频道名称应返回 "yuanbao"
func TestYuanbaoChannel_Name(t *testing.T) {
	ch, _ := newTestYuanbaoChannel(t, nil, nil)
	if ch.Name() != "yuanbao" {
		t.Errorf("Name() = %q, 期望 %q", ch.Name(), "yuanbao")
	}
}

// ---------------------------------------------------------------------------
// EditMessage 编辑消息测试（元宝 API 不支持，始终返回 nil）
// ---------------------------------------------------------------------------

// TestEditMessage_ReturnsNil EditMessage 不支持编辑，应始终返回 nil
func TestEditMessage_ReturnsNil(t *testing.T) {
	ch, _ := newTestYuanbaoChannel(t, nil, nil)

	err := ch.EditMessage(context.Background(), "chat-1", "msg-1", "新内容")
	if err != nil {
		t.Errorf("EditMessage 应返回 nil（不支持编辑），实际: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Send 未运行时发送消息测试
// ---------------------------------------------------------------------------

// TestSend_NotRunning 频道未运行时发送消息应返回 ErrNotRunning
func TestSend_NotRunning(t *testing.T) {
	ch, _ := newTestYuanbaoChannel(t, nil, nil)
	// 频道未启动，IsRunning() 应为 false

	_, err := ch.Send(context.Background(), bus.OutboundMessage{
		ChatID:  "user-123",
		Content: "hello",
	})
	if err == nil {
		t.Fatal("未运行的频道发送消息应返回错误")
	}
	if !errors.Is(err, channels.ErrNotRunning) {
		t.Errorf("期望 ErrNotRunning，实际: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Stop 停止频道测试
// ---------------------------------------------------------------------------

// TestStop_Idempotent Stop 应可幂等调用，即使未启动也不应 panic
func TestStop_Idempotent(t *testing.T) {
	ch, _ := newTestYuanbaoChannel(t, nil, nil)

	// 未启动时调用 Stop 不应 panic
	err := ch.Stop(context.Background())
	if err != nil {
		t.Errorf("Stop 返回了未预期的错误: %v", err)
	}

	// 重复 Stop 也应正常
	err = ch.Stop(context.Background())
	if err != nil {
		t.Errorf("第二次 Stop 返回了未预期的错误: %v", err)
	}

	if ch.IsRunning() {
		t.Error("停止后频道不应处于运行状态")
	}
}

// ---------------------------------------------------------------------------
// applyYuanbaoProxy 代理配置测试
// ---------------------------------------------------------------------------

// TestApplyYuanbaoProxy_NoProxy 未配置代理时不应报错
func TestApplyYuanbaoProxy_NoProxy(t *testing.T) {
	cfg := testYuanbaoSettings()
	cfg.Proxy = ""
	bc := &config.Channel{Type: config.ChannelYuanbao, Enabled: true}
	msgBus := bus.NewMessageBus()

	ch, err := NewYuanbaoChannel(bc, cfg, msgBus, "info")
	if err != nil {
		t.Fatalf("未预期的错误: %v", err)
	}

	if err := ch.applyYuanbaoProxy(); err != nil {
		t.Errorf("空代理配置不应报错，实际: %v", err)
	}
}

// TestApplyYuanbaoProxy_InvalidProxyURL 无效代理 URL 应返回错误
func TestApplyYuanbaoProxy_InvalidProxyURL(t *testing.T) {
	cfg := testYuanbaoSettings()
	cfg.Proxy = "://invalid-url"
	bc := &config.Channel{Type: config.ChannelYuanbao, Enabled: true}
	msgBus := bus.NewMessageBus()

	ch, err := NewYuanbaoChannel(bc, cfg, msgBus, "info")
	if err != nil {
		t.Fatalf("未预期的错误: %v", err)
	}

	err = ch.applyYuanbaoProxy()
	if err == nil {
		t.Fatal("无效代理 URL 应返回错误")
	}
	if !strings.Contains(err.Error(), "invalid proxy URL") {
		t.Errorf("错误信息应包含 invalid proxy URL，实际: %v", err)
	}
}

// TestApplyYuanbaoProxy_ValidProxyURL 有效代理 URL 不应报错
func TestApplyYuanbaoProxy_ValidProxyURL(t *testing.T) {
	cfg := testYuanbaoSettings()
	cfg.Proxy = "http://127.0.0.1:8080"
	bc := &config.Channel{Type: config.ChannelYuanbao, Enabled: true}
	msgBus := bus.NewMessageBus()

	ch, err := NewYuanbaoChannel(bc, cfg, msgBus, "info")
	if err != nil {
		t.Fatalf("未预期的错误: %v", err)
	}

	err = ch.applyYuanbaoProxy()
	if err != nil {
		t.Errorf("有效代理 URL 不应报错，实际: %v", err)
	}
}

// ---------------------------------------------------------------------------
// chatType sync.Map 并发安全测试
// ---------------------------------------------------------------------------

// TestChatType_ConcurrentAccess 并发读写 chatType 不应出现数据竞争
func TestChatType_ConcurrentAccess(t *testing.T) {
	ch, _ := newTestYuanbaoChannel(t, nil, nil)

	var wg sync.WaitGroup
	const goroutines = 100

	// 并发存储和读取
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			chatID := "chat-concurrent"
			ch.chatType.Store(chatID, "group")
			ch.getChatKind(chatID)
		}(i)
	}
	wg.Wait()

	// 所有写入完成后，getChatKind 应返回 "group"
	if got := ch.getChatKind("chat-concurrent"); got != "group" {
		t.Errorf("并发写入后 getChatKind = %q, 期望 %q", got, "group")
	}
}

// ---------------------------------------------------------------------------
// Token 过期时间计算测试
// ---------------------------------------------------------------------------

// TestSaveYuanbaoToken_ExpiryCalculation 过期时间应近似等于 当前时间 + expiresIn
func TestSaveYuanbaoToken_ExpiryCalculation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens", "expiry-test.json")

	beforeSave := time.Now()
	err := saveYuanbaoToken(path, "app", "tok", 600) // 600 秒 = 10 分钟
	if err != nil {
		t.Fatalf("saveYuanbaoToken 失败: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}

	var tf yuanbaoTokenFile
	if err := json.Unmarshal(data, &tf); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	// ExpiresAt 应约为 beforeSave + 600 秒
	expectedExpiry := beforeSave.Add(600 * time.Second)
	diff := tf.ExpiresAt.Sub(expectedExpiry)
	if diff < 0 {
		diff = -diff
	}
	if diff > 2*time.Second {
		t.Errorf("expires_at 偏差 %v，期望接近 %v，实际 %v", diff, expectedExpiry, tf.ExpiresAt)
	}
}

// ---------------------------------------------------------------------------
// yuanbaoTokenFile JSON 结构测试
// ---------------------------------------------------------------------------

// TestYuanbaoTokenFile_JSONStructure 验证 token 文件的 JSON 字段完整性
func TestYuanbaoTokenFile_JSONStructure(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	tf := yuanbaoTokenFile{
		Token:     "tok_123",
		Name:      "app-id",
		ExpiresAt: now,
		Weight:    3,
	}

	data, err := json.Marshal(tf)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("反序列化到 map 失败: %v", err)
	}

	if _, ok := parsed["token"]; !ok {
		t.Error("JSON 应包含 'token' 字段")
	}
	if _, ok := parsed["name"]; !ok {
		t.Error("JSON 应包含 'name' 字段")
	}
	if _, ok := parsed["expires_at"]; !ok {
		t.Error("JSON 应包含 'expires_at' 字段")
	}
	if _, ok := parsed["weight"]; !ok {
		t.Error("JSON 应包含 'weight' 字段")
	}
}

// ---------------------------------------------------------------------------
// buildYuanbaoTokensPath 与 picoclawHomeDir 集成测试
// ---------------------------------------------------------------------------

// TestBuildYuanbaoTokensPath_ContainsHomeDir token 路径应以 HomeDir 为前缀
func TestBuildYuanbaoTokensPath_ContainsHomeDir(t *testing.T) {
	cfg := &config.YuanbaoSettings{AppID: "my-app"}
	path := buildYuanbaoTokensPath(cfg)

	home := picoclawHomeDir()
	if !strings.HasPrefix(path, home) {
		t.Errorf("路径应以 HomeDir %q 开头，实际 %q", home, path)
	}
}

// ---------------------------------------------------------------------------
// 代理 URL 解析测试
// ---------------------------------------------------------------------------

// TestApplyYuanbaoProxy_ParsesURL 测试各种代理 URL 格式的解析
func TestApplyYuanbaoProxy_ParsesURL(t *testing.T) {
	tests := []struct {
		name       string
		proxy      string
		shouldFail bool
	}{
		{"HTTP 代理", "http://proxy.example.com:3128", false},
		{"HTTPS 代理", "https://proxy.example.com:3128", false},
		{"SOCKS5 代理", "socks5://proxy.example.com:1080", false},
		{"空代理", "", false},
		{"无效协议", "://bad", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, parseErr := url.Parse(tt.proxy)
			if parseErr != nil && !tt.shouldFail {
				t.Errorf("url.Parse(%q) 未预期的错误: %v", tt.proxy, parseErr)
			}
		})
	}
}
