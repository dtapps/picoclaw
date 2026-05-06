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

// MCP config types
type mcpServerConfig struct {
	Name    string            `json:"name"`
	Enabled bool              `json:"enabled"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	EnvFile string            `json:"env_file,omitempty"`
	Type    string            `json:"type,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type mcpDiscoveryConfig struct {
	Enabled          bool `json:"enabled"`
	TTL              int  `json:"ttl"`
	MaxSearchResults int  `json:"max_search_results"`
	UseBM25          bool `json:"use_bm25"`
	UseRegex         bool `json:"use_regex"`
}

type mcpConfigResponse struct {
	Enabled            bool               `json:"enabled"`
	MaxInlineTextChars int                `json:"max_inline_text_chars"`
	Discovery          mcpDiscoveryConfig `json:"discovery"`
	Servers            []mcpServerConfig  `json:"servers"`
}

type mcpConfigRequest struct {
	Enabled            bool               `json:"enabled"`
	MaxInlineTextChars int                `json:"max_inline_text_chars"`
	Discovery          mcpDiscoveryConfig `json:"discovery"`
	Servers            []mcpServerConfig  `json:"servers"`
}

// MCP server details types
type mcpToolParameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type mcpTool struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Parameters  []mcpToolParameter `json:"parameters"`
}

type mcpPrompt struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type mcpResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mime_type,omitempty"`
}

type mcpServerDetailsResponse struct {
	ServerName string        `json:"server_name"`
	Connected  bool          `json:"connected"`
	Error      string        `json:"error,omitempty"`
	Tools      []mcpTool     `json:"tools"`
	Prompts    []mcpPrompt   `json:"prompts"`
	Resources  []mcpResource `json:"resources"`
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

func (h *Handler) handleGetMCPServerDetails(w http.ResponseWriter, r *http.Request) {
	serverName := r.PathValue("name")
	if serverName == "" {
		http.Error(w, "Server name is required", http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	serverConfig, ok := cfg.Tools.MCP.Servers[serverName]
	if !ok {
		http.Error(w, fmt.Sprintf("Server %q not found", serverName), http.StatusNotFound)
		return
	}

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

	details := probeMCPServerDetails(r.Context(), serverName, serverConfig, cfg.WorkspacePath())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(details)
}

func probeMCPServerDetails(
	ctx context.Context,
	name string,
	server config.MCPServerConfig,
	workspacePath string,
) mcpServerDetailsResponse {
	mgr := mcp.NewManager()
	defer func() { _ = mgr.Close() }()

	server.Enabled = true
	mcpCfg := config.MCPConfig{
		ToolConfig: config.ToolConfig{Enabled: true},
		Servers: map[string]config.MCPServerConfig{
			name: server,
		},
	}

	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

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

func extractMCPParameters(schema any) []mcpToolParameter {
	schemaMap, ok := schema.(map[string]any)
	if !ok {
		return nil
	}

	properties, ok := schemaMap["properties"].(map[string]any)
	if !ok || len(properties) == 0 {
		return nil
	}

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

	params := make([]mcpToolParameter, 0, len(properties))
	for paramName, prop := range properties {
		param := mcpToolParameter{
			Name:     paramName,
			Required: false,
		}

		if _, ok := required[paramName]; ok {
			param.Required = true
		}

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
