package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/isolation"
)

// envVarEntryResponse 表示用于 API 响应的环境变量条目
type envVarEntryResponse struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Enabled   bool   `json:"enabled"`
	Sensitive bool   `json:"sensitive"`
	Note      string `json:"note"`
}

// envVarsConfigResponse 表示环境变量配置响应
type envVarsConfigResponse struct {
	Variables []envVarEntryResponse `json:"variables"`
}

// envVarsUpdateRequest 表示更新环境变量的请求
type envVarsUpdateRequest struct {
	Variables []envVarEntryResponse `json:"variables"`
}

// registerEnvVarsRoutes 注册环境变量 API 路由
func (h *Handler) registerEnvVarsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/env-vars", h.handleGetEnvVars)
	mux.HandleFunc("PUT /api/env-vars", h.handleUpdateEnvVars)
	mux.HandleFunc("POST /api/env-vars/import", h.handleImportEnvVars)
	mux.HandleFunc("GET /api/env-vars/export", h.handleExportEnvVars)
}

// handleGetEnvVars 处理 GET /api/env-vars
func (h *Handler) handleGetEnvVars(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("加载配置失败: %v", err), http.StatusInternalServerError)
		return
	}

	response := envVarsConfigResponse{
		Variables: make([]envVarEntryResponse, 0, len(cfg.EnvVars.Variables)),
	}

	for _, v := range cfg.EnvVars.Variables {
		entry := envVarEntryResponse{
			Key:       v.Key,
			Enabled:   v.Enabled,
			Sensitive: v.Sensitive,
			Note:      v.Note,
		}
		// 敏感变量从 SecureValue 获取值，非敏感变量从 Value 获取
		if v.Sensitive {
			entry.Value = v.SecureValue.String()
		} else {
			entry.Value = v.Value
		}
		response.Variables = append(response.Variables, entry)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("编码响应失败: %v", err), http.StatusInternalServerError)
		return
	}
}

// handleUpdateEnvVars 处理 PUT /api/env-vars
func (h *Handler) handleUpdateEnvVars(w http.ResponseWriter, r *http.Request) {
	var req envVarsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("无效的请求体: %v", err), http.StatusBadRequest)
		return
	}

	// 加载现有配置
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("加载配置失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 验证并转换变量
	variables := make([]config.EnvVarEntry, 0, len(req.Variables))
	for _, v := range req.Variables {
		// 验证键格式
		if !isValidEnvVarKey(v.Key) {
			http.Error(w, fmt.Sprintf("无效的环境变量键: %s", v.Key), http.StatusBadRequest)
			return
		}

		entry := config.EnvVarEntry{
			Key:       v.Key,
			Enabled:   v.Enabled,
			Sensitive: v.Sensitive,
			Note:      v.Note,
		}

		// 如果值被隐藏（********），保留现有值
		if v.Sensitive && v.Value == "********" {
			// 查找现有值
			for _, existing := range cfg.EnvVars.Variables {
				if existing.Key == v.Key {
					entry.SecureValue = existing.SecureValue
					break
				}
			}
		} else if v.Sensitive {
			// 敏感值保存到 SecureValue
			entry.SecureValue = *config.NewSecureString(v.Value)
		} else {
			entry.Value = v.Value
		}

		variables = append(variables, entry)
	}

	// 更新配置
	cfg.EnvVars = config.EnvVarsConfig{
		Variables: variables,
	}

	// 保存配置
	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, fmt.Sprintf("保存配置失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 重新加载配置到 isolation 包，使环境变量立即生效
	isolation.Configure(cfg)

	w.WriteHeader(http.StatusOK)
}

// handleImportEnvVars 处理 POST /api/env-vars/import
func (h *Handler) handleImportEnvVars(w http.ResponseWriter, r *http.Request) {
	// 解析多部分表单
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 最大 10 MB
		http.Error(w, fmt.Sprintf("解析表单失败: %v", err), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("获取文件失败: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 读取文件内容
	content := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, readErr := file.Read(buf)
		if n > 0 {
			content = append(content, buf[:n]...)
		}
		if readErr != nil {
			break
		}
	}

	// 解析 .env 格式
	vars, err := parseEnvFileContent(string(content))
	if err != nil {
		http.Error(w, fmt.Sprintf("解析文件失败: %v", err), http.StatusBadRequest)
		return
	}

	// 转换为响应格式
	response := make([]envVarEntryResponse, 0, len(vars))
	for k, v := range vars {
		response = append(response, envVarEntryResponse{
			Key:       k,
			Value:     v,
			Enabled:   true,
			Sensitive: isLikelySensitive(k),
			Note:      "从文件导入",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("编码响应失败: %v", err), http.StatusInternalServerError)
		return
	}
}

// handleExportEnvVars 处理 GET /api/env-vars/export
func (h *Handler) handleExportEnvVars(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("加载配置失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 构建 .env 格式内容
	var sb strings.Builder
	for _, v := range cfg.EnvVars.Variables {
		if !v.Enabled {
			continue
		}
		sb.WriteString(fmt.Sprintf("# %s\n", v.Note))
		sb.WriteString(fmt.Sprintf("%s=%s\n\n", v.Key, v.Value))
	}

	// 设置文件下载头
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", "attachment; filename=picoclaw.env")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(sb.String()))
}

// isValidEnvVarKey 检查字符串是否为有效的环境变量键
func isValidEnvVarKey(key string) bool {
	if key == "" {
		return false
	}
	// 第一个字符必须是字母或下划线
	if !((key[0] >= 'a' && key[0] <= 'z') || (key[0] >= 'A' && key[0] <= 'Z') || key[0] == '_') {
		return false
	}
	// 剩余字符必须是字母数字或下划线
	for i := 1; i < len(key); i++ {
		c := key[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// isLikelySensitive 检查键是否可能包含敏感信息
func isLikelySensitive(key string) bool {
	lower := strings.ToLower(key)
	sensitiveKeywords := []string{
		"password", "secret", "token", "key", "api_key", "apikey",
		"auth", "credential", "private", "passwd", "pwd",
	}
	for _, keyword := range sensitiveKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

// parseEnvFileContent 解析 .env 文件内容
func parseEnvFileContent(content string) (map[string]string, error) {
	vars := make(map[string]string)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 解析 KEY=VALUE 格式
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			// 如果存在引号则移除
			if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}
			vars[key] = value
		}
	}
	return vars, nil
}
