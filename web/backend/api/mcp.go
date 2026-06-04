package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/mcp"
)

// ==================== MCP 配置类型 ====================

// mcpServerConfig 表示单个 MCP 服务器的配置
type mcpServerConfig struct {
	Name    string            `json:"name"`               // 服务器名称
	Enabled bool              `json:"enabled"`            // 是否启用
	Command string            `json:"command,omitempty"`  // 启动命令（stdio 类型）
	Args    []string          `json:"args,omitempty"`     // 命令参数
	Env     map[string]string `json:"env,omitempty"`      // 环境变量
	EnvFile string            `json:"env_file,omitempty"` // 环境变量文件路径
	Type    string            `json:"type,omitempty"`     // 连接类型：stdio | sse | http
	URL     string            `json:"url,omitempty"`      // 服务器 URL
	Headers map[string]string `json:"headers,omitempty"`  // HTTP 请求头
}

// mcpDiscoveryConfig 表示 MCP 发现模式的配置
type mcpDiscoveryConfig struct {
	Enabled          bool `json:"enabled"`            // 是否启用发现模式
	TTL              int  `json:"ttl"`                // 工具解锁状态的存活轮数
	MaxSearchResults int  `json:"max_search_results"` // 每次搜索返回的最大工具数
	UseBM25          bool `json:"use_bm25"`           // 是否使用 BM25 搜索
	UseRegex         bool `json:"use_regex"`          // 是否使用正则搜索
}

// mcpConfigResponse MCP 配置响应结构
type mcpConfigResponse struct {
	Enabled            bool               `json:"enabled"`               // MCP 总开关
	MaxInlineTextChars int                `json:"max_inline_text_chars"` // 最大内联文本字符数
	Discovery          mcpDiscoveryConfig `json:"discovery"`             // 发现模式配置
	Servers            []mcpServerConfig  `json:"servers"`               // 服务器列表
}

// mcpConfigRequest MCP 配置请求结构
type mcpConfigRequest struct {
	Enabled            bool               `json:"enabled"`
	MaxInlineTextChars int                `json:"max_inline_text_chars"`
	Discovery          mcpDiscoveryConfig `json:"discovery"`
	Servers            []mcpServerConfig  `json:"servers"`
}

// ==================== MCP 服务器详情类型 ====================

// mcpToolParameter 表示工具参数定义
type mcpToolParameter struct {
	Name        string `json:"name"`        // 参数名称
	Type        string `json:"type"`        // 参数类型
	Description string `json:"description"` // 参数描述
	Required    bool   `json:"required"`    // 是否必填
}

// mcpTool 表示 MCP 工具定义
type mcpTool struct {
	Name        string             `json:"name"`        // 工具名称
	Description string             `json:"description"` // 工具描述
	Parameters  []mcpToolParameter `json:"parameters"`  // 参数列表
}

// mcpPrompt 表示 MCP 提示模板（预留）
type mcpPrompt struct {
	Name        string `json:"name"`        // 提示名称
	Description string `json:"description"` // 提示描述
}

// mcpResource 表示 MCP 资源（预留）
type mcpResource struct {
	URI         string `json:"uri"`                   // 资源 URI
	Name        string `json:"name"`                  // 资源名称
	Description string `json:"description,omitempty"` // 资源描述
	MimeType    string `json:"mime_type,omitempty"`   // MIME 类型
}

// mcpServerDetailsResponse MCP 服务器详情响应
type mcpServerDetailsResponse struct {
	ServerName string        `json:"server_name"`     // 服务器名称
	Connected  bool          `json:"connected"`       // 连接状态
	Error      string        `json:"error,omitempty"` // 错误信息
	Tools      []mcpTool     `json:"tools"`           // 工具列表
	Prompts    []mcpPrompt   `json:"prompts"`         // 提示列表（预留）
	Resources  []mcpResource `json:"resources"`       // 资源列表（预留）
}

func (h *Handler) registerMCPRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mcp/config", h.handleGetMCPConfig)
	mux.HandleFunc("PUT /api/mcp/config", h.handleUpdateMCPConfig)
	mux.HandleFunc("GET /api/mcp/servers/{name}/details", h.handleGetMCPServerDetails)
}

func (h *Handler) handleGetMCPConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(buildMCPConfigResponse(cfg)); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleUpdateMCPConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	var req mcpConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	cfg.Tools.MCP.Enabled = req.Enabled
	if req.MaxInlineTextChars > 0 {
		cfg.Tools.MCP.MaxInlineTextChars = req.MaxInlineTextChars
	}

	cfg.Tools.MCP.Discovery.Enabled = req.Discovery.Enabled
	if req.Discovery.TTL > 0 {
		cfg.Tools.MCP.Discovery.TTL = req.Discovery.TTL
	}
	if req.Discovery.MaxSearchResults > 0 {
		cfg.Tools.MCP.Discovery.MaxSearchResults = req.Discovery.MaxSearchResults
	}
	cfg.Tools.MCP.Discovery.UseBM25 = req.Discovery.UseBM25
	cfg.Tools.MCP.Discovery.UseRegex = req.Discovery.UseRegex

	cfg.Tools.MCP.Servers = make(map[string]config.MCPServerConfig)
	for _, server := range req.Servers {
		name := strings.TrimSpace(server.Name)
		if name == "" {
			continue
		}
		cfg.Tools.MCP.Servers[name] = config.MCPServerConfig{
			Enabled: server.Enabled,
			Command: server.Command,
			Args:    server.Args,
			Env:     server.Env,
			EnvFile: server.EnvFile,
			Type:    server.Type,
			URL:     server.URL,
			Headers: server.Headers,
		}
	}

	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(buildMCPConfigResponse(cfg)); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func buildMCPConfigResponse(cfg *config.Config) mcpConfigResponse {
	servers := make([]mcpServerConfig, 0, len(cfg.Tools.MCP.Servers))
	for name, server := range cfg.Tools.MCP.Servers {
		servers = append(servers, mcpServerConfig{
			Name:    name,
			Enabled: server.Enabled,
			Command: server.Command,
			Args:    server.Args,
			Env:     server.Env,
			EnvFile: server.EnvFile,
			Type:    server.Type,
			URL:     server.URL,
			Headers: server.Headers,
		})
	}

	discovery := mcpDiscoveryConfig{
		Enabled:          cfg.Tools.MCP.Discovery.Enabled,
		TTL:              cfg.Tools.MCP.Discovery.TTL,
		MaxSearchResults: cfg.Tools.MCP.Discovery.MaxSearchResults,
		UseBM25:          cfg.Tools.MCP.Discovery.UseBM25,
		UseRegex:         cfg.Tools.MCP.Discovery.UseRegex,
	}
	if discovery.TTL == 0 {
		discovery.TTL = 5
	}
	if discovery.MaxSearchResults == 0 {
		discovery.MaxSearchResults = 5
	}

	return mcpConfigResponse{
		Enabled:            cfg.Tools.MCP.Enabled,
		MaxInlineTextChars: cfg.Tools.MCP.MaxInlineTextChars,
		Discovery:          discovery,
		Servers:            servers,
	}
}

// handleGetMCPServerDetails 处理获取 MCP 服务器详情的请求
// GET /api/mcp/servers/{name}/details
func (h *Handler) handleGetMCPServerDetails(w http.ResponseWriter, r *http.Request) {
	serverName := r.PathValue("name")
	if serverName == "" {
		http.Error(w, "Server name is required", http.StatusBadRequest)
		return
	}

	// 加载配置
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	// 查找服务器配置
	serverConfig, ok := cfg.Tools.MCP.Servers[serverName]
	if !ok {
		http.Error(w, fmt.Sprintf("Server %q not found", serverName), http.StatusNotFound)
		return
	}

	// 如果 MCP 或服务器被禁用，返回禁用状态
	if !cfg.Tools.MCP.Enabled || !serverConfig.Enabled {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mcpServerDetailsResponse{
			ServerName: serverName,
			Connected:  false,
			Error:      "MCP or server is disabled",
			Tools:      []mcpTool{},
			Prompts:    []mcpPrompt{},
			Resources:  []mcpResource{},
		})
		return
	}

	// 探测服务器详情
	details := probeMCPServerDetails(r.Context(), serverName, serverConfig, cfg.WorkspacePath())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(details)
}

// probeMCPServerDetails 探测 MCP 服务器的详细信息
// 临时连接到服务器并获取其工具、提示和资源列表
// 参数:
//   - ctx: 上下文
//   - name: 服务器名称
//   - server: 服务器配置
//   - workspacePath: 工作目录路径
//
// 返回:
//   - 服务器详情响应
func probeMCPServerDetails(
	ctx context.Context,
	name string,
	server config.MCPServerConfig,
	workspacePath string,
) mcpServerDetailsResponse {
	// 创建临时 MCP 管理器
	mgr := mcp.NewManager()
	defer func() { _ = mgr.Close() }()

	// 启用服务器并创建配置
	server.Enabled = true
	mcpCfg := config.MCPConfig{
		ToolConfig: config.ToolConfig{Enabled: true},
		Servers: map[string]config.MCPServerConfig{
			name: server,
		},
	}

	// 设置 30 秒超时
	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 加载服务器配置
	if err := mgr.LoadFromMCPConfig(probeCtx, mcpCfg, workspacePath); err != nil {
		return mcpServerDetailsResponse{
			ServerName: name,
			Connected:  false,
			Error:      err.Error(),
			Tools:      []mcpTool{},
			Prompts:    []mcpPrompt{},
			Resources:  []mcpResource{},
		}
	}

	// 获取服务器连接
	conn, ok := mgr.GetServer(name)
	if !ok {
		return mcpServerDetailsResponse{
			ServerName: name,
			Connected:  false,
			Error:      "server did not register a connection",
			Tools:      []mcpTool{},
			Prompts:    []mcpPrompt{},
			Resources:  []mcpResource{},
		}
	}

	// 转换工具列表
	tools := make([]mcpTool, 0, len(conn.Tools))
	for _, tool := range conn.Tools {
		tools = append(tools, mcpTool{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  extractMCPParameters(tool.InputSchema),
		})
	}

	return mcpServerDetailsResponse{
		ServerName: name,
		Connected:  true,
		Tools:      tools,
		Prompts:    []mcpPrompt{},
		Resources:  []mcpResource{},
	}
}

// extractMCPParameters 从 JSON Schema 中提取工具参数定义
// 解析 schema 的 properties 和 required 字段
// 参数:
//   - schema: 工具的 inputSchema（JSON Schema 格式）
//
// 返回:
//   - 参数列表
func extractMCPParameters(schema any) []mcpToolParameter {
	schemaMap, ok := schema.(map[string]any)
	if !ok {
		return nil
	}

	// 提取 properties
	properties, ok := schemaMap["properties"].(map[string]any)
	if !ok || len(properties) == 0 {
		return nil
	}

	// 提取 required 字段
	required := make(map[string]struct{})
	switch raw := schemaMap["required"].(type) {
	case []string:
		for _, name := range raw {
			required[name] = struct{}{}
		}
	case []any:
		for _, value := range raw {
			if name, ok := value.(string); ok {
				required[name] = struct{}{}
			}
		}
	}

	// 构建参数列表
	params := make([]mcpToolParameter, 0, len(properties))
	for paramName, prop := range properties {
		param := mcpToolParameter{
			Name:     paramName,
			Required: false,
		}

		// 标记必填参数
		if _, ok := required[paramName]; ok {
			param.Required = true
		}

		// 提取参数描述和类型
		if propMap, ok := prop.(map[string]any); ok {
			if desc, ok := propMap["description"].(string); ok {
				param.Description = desc
			}
			if typ, ok := propMap["type"].(string); ok {
				param.Type = typ
			}
		}

		params = append(params, param)
	}

	return params
}
