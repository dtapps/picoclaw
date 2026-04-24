package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
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

func (h *Handler) registerMCPRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mcp/config", h.handleGetMCPConfig)
	mux.HandleFunc("PUT /api/mcp/config", h.handleUpdateMCPConfig)
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
