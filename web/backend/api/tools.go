package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
	picotools "github.com/sipeed/picoclaw/pkg/tools"
)

type toolCatalogEntry struct {
	Name        string
	Description string
	Category    string
	ConfigKey   string
	Parameters  map[string]any // JSON Schema
}

type toolSupportItem struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	ConfigKey   string         `json:"config_key"`
	Status      string         `json:"status"`
	ReasonCode  string         `json:"reason_code,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type toolSupportResponse struct {
	Tools []toolSupportItem `json:"tools"`
}

type toolStateRequest struct {
	Enabled bool `json:"enabled"`
}

type webSearchProviderOption struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Configured   bool   `json:"configured"`
	Current      bool   `json:"current"`
	RequiresAuth bool   `json:"requires_auth"`
}

type webSearchProviderConfig struct {
	Enabled    bool     `json:"enabled"`
	MaxResults int      `json:"max_results"`
	BaseURL    string   `json:"base_url,omitempty"`
	APIKey     string   `json:"api_key,omitempty"`
	APIKeys    []string `json:"api_keys,omitempty"`
	Model      string   `json:"model,omitempty"`
	APIKeySet  bool     `json:"api_key_set,omitempty"`
}

type webSearchConfigResponse struct {
	Provider       string                             `json:"provider"`
	CurrentService string                             `json:"current_service"`
	PreferNative   bool                               `json:"prefer_native"`
	Proxy          string                             `json:"proxy,omitempty"`
	Providers      []webSearchProviderOption          `json:"providers"`
	Settings       map[string]webSearchProviderConfig `json:"settings"`
}

type webSearchConfigRequest struct {
	Provider     string                             `json:"provider"`
	PreferNative bool                               `json:"prefer_native"`
	Proxy        string                             `json:"proxy"`
	Settings     map[string]webSearchProviderConfig `json:"settings"`
}

var toolCatalog = []toolCatalogEntry{
	{
		Name:        "read_file",
		Description: "Read file content from the workspace or explicitly allowed paths.",
		Category:    "filesystem",
		ConfigKey:   "read_file",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":   map[string]any{"type": "string", "description": "Path to the file to read."},
				"offset": map[string]any{"type": "integer", "description": "Byte offset to start reading from."},
				"length": map[string]any{"type": "integer", "description": "Maximum number of bytes to read."},
			},
			"required": []string{"path"},
		},
	},
	{
		Name:        "write_file",
		Description: "Create or overwrite files within the writable workspace scope.",
		Category:    "filesystem",
		ConfigKey:   "write_file",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "Path to the file to write"},
				"content": map[string]any{"type": "string", "description": "Content to write to the file"},
				"overwrite": map[string]any{
					"type":        "boolean",
					"description": "Must be true to overwrite an existing file.",
				},
			},
			"required": []string{"path", "content"},
		},
	},
	{
		Name:        "list_dir",
		Description: "Inspect directories and enumerate files available to the agent.",
		Category:    "filesystem",
		ConfigKey:   "list_dir",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Path to list"},
			},
			"required": []string{"path"},
		},
	},
	{
		Name:        "edit_file",
		Description: "Apply targeted edits to existing files without rewriting everything.",
		Category:    "filesystem",
		ConfigKey:   "edit_file",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":     map[string]any{"type": "string", "description": "The file path to edit"},
				"old_text": map[string]any{"type": "string", "description": "The exact text to find and replace"},
				"new_text": map[string]any{"type": "string", "description": "The text to replace with"},
			},
			"required": []string{"path", "old_text", "new_text"},
		},
	},
	{
		Name:        "append_file",
		Description: "Append content to the end of an existing file.",
		Category:    "filesystem",
		ConfigKey:   "append_file",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "The file path to append to"},
				"content": map[string]any{"type": "string", "description": "The content to append"},
			},
			"required": []string{"path", "content"},
		},
	},
	{
		Name:        "exec",
		Description: "Run shell commands inside the configured workspace sandbox.",
		Category:    "filesystem",
		ConfigKey:   "exec",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"description": "Action: run, list, poll, read, write, kill, send-keys",
					"enum":        []string{"run", "list", "poll", "read", "write", "kill", "send-keys"},
				},
				"command": map[string]any{
					"type":        "string",
					"description": "Shell command to execute (required for run)",
				},
				"sessionId": map[string]any{
					"type":        "string",
					"description": "Session ID (required for poll/read/write/kill/send-keys)",
				},
			},
			"required": []string{"action"},
		},
	},
	{
		Name:        "cron",
		Description: "Schedule one-time or recurring reminders, jobs, and shell commands.",
		Category:    "automation",
		ConfigKey:   "cron",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"description": "Action: add, list, remove, enable, disable",
					"enum":        []string{"add", "list", "remove", "enable", "disable"},
				},
				"message": map[string]any{"type": "string", "description": "The reminder/task message"},
				"cron":    map[string]any{"type": "string", "description": "Cron expression (e.g., '0 9 * * *')"},
			},
			"required": []string{"action"},
		},
	},
	{
		Name:        "web_search",
		Description: "Search the web using the configured providers.",
		Category:    "web",
		ConfigKey:   "web",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "Search query"},
				"count": map[string]any{"type": "integer", "description": "Number of results (default: 10)"},
			},
			"required": []string{"query"},
		},
	},
	{
		Name:        "web_fetch",
		Description: "Fetch and summarize the contents of a webpage.",
		Category:    "web",
		ConfigKey:   "web_fetch",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url":      map[string]any{"type": "string", "description": "URL to fetch"},
				"maxChars": map[string]any{"type": "integer", "description": "Maximum characters to extract"},
			},
			"required": []string{"url"},
		},
	},
	{
		Name:        "message",
		Description: "Send a follow-up message back to the active user or chat.",
		Category:    "communication",
		ConfigKey:   "message",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"content": map[string]any{"type": "string", "description": "The message content to send"},
				"channel": map[string]any{"type": "string", "description": "Target channel (telegram, whatsapp, etc.)"},
				"chat_id": map[string]any{"type": "string", "description": "Target chat/user ID"},
			},
			"required": []string{"content"},
		},
	},
	{
		Name:        "send_file",
		Description: "Send an outbound file or media attachment to the active chat.",
		Category:    "communication",
		ConfigKey:   "send_file",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":     map[string]any{"type": "string", "description": "Path to the local file"},
				"filename": map[string]any{"type": "string", "description": "Optional display filename"},
			},
			"required": []string{"path"},
		},
	},
	{
		Name:        "find_skills",
		Description: "Search external skill registries for installable skills.",
		Category:    "skills",
		ConfigKey:   "find_skills",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query describing the desired skill capability",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of results (1-20, default 5)",
				},
			},
			"required": []string{"query"},
		},
	},
	{
		Name:        "install_skill",
		Description: "Install a skill into the current workspace from a registry.",
		Category:    "skills",
		ConfigKey:   "install_skill",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"slug":    map[string]any{"type": "string", "description": "The unique slug of the skill to install"},
				"version": map[string]any{"type": "string", "description": "Specific version to install (optional)"},
			},
			"required": []string{"slug"},
		},
	},
	{
		Name:        "spawn",
		Description: "Launch a background subagent for long-running or delegated work.",
		Category:    "agents",
		ConfigKey:   "spawn",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task":     map[string]any{"type": "string", "description": "The task for subagent to complete"},
				"label":    map[string]any{"type": "string", "description": "Optional short label for the task"},
				"agent_id": map[string]any{"type": "string", "description": "Optional target agent ID"},
			},
			"required": []string{"task"},
		},
	},
	{
		Name:        "spawn_status",
		Description: "Query the status of spawned subagents.",
		Category:    "agents",
		ConfigKey:   "spawn_status",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "Optional task ID to inspect a specific subagent",
				},
			},
		},
	},
	{
		Name:        "i2c",
		Description: "Interact with I2C hardware devices exposed on the host.",
		Category:    "hardware",
		ConfigKey:   "i2c",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"description": "Action: detect, scan, read, write",
					"enum":        []string{"detect", "scan", "read", "write"},
				},
				"bus":     map[string]any{"type": "string", "description": "I2C bus number"},
				"address": map[string]any{"type": "integer", "description": "7-bit I2C device address"},
			},
			"required": []string{"action"},
		},
	},
	{
		Name:        "spi",
		Description: "Interact with SPI hardware devices exposed on the host.",
		Category:    "hardware",
		ConfigKey:   "spi",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"description": "Action: list, transfer, read",
					"enum":        []string{"list", "transfer", "read"},
				},
				"device": map[string]any{"type": "string", "description": "SPI device identifier"},
			},
			"required": []string{"action"},
		},
	},
	{
		Name:        "serial",
		Description: "Interact with serial ports exposed on the host.",
		Category:    "hardware",
		ConfigKey:   "serial",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"description": "Action: list, read, write",
					"enum":        []string{"list", "read", "write"},
				},
				"port": map[string]any{"type": "string", "description": "Serial port path"},
				"baud": map[string]any{"type": "integer", "description": "Baud rate (default: 115200)"},
			},
			"required": []string{"action"},
		},
	},
	{
		Name:        "get_current_time",
		Description: "Get the current time, date, or both in various formats and timezones.",
		Category:    "utility",
		ConfigKey:   "get_current_time",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"format": map[string]any{
					"type":        "string",
					"description": "Output format: iso, time, date, datetime, unix",
					"enum":        []string{"iso", "time", "date", "datetime", "unix"},
				},
				"timezone": map[string]any{"type": "string", "description": "Timezone name (e.g., Asia/Shanghai)"},
			},
		},
	},
	{
		Name:        "browser_ext",
		Description: "Control the user's browser via the browser extension (click, type, navigate, etc.).",
		Category:    "browser",
		ConfigKey:   "browser_ext",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"description": "The browser action to execute",
					"enum": []string{
						"navigate",
						"get_page_info",
						"click",
						"type",
						"fill",
						"scroll",
						"screenshot",
						"get_text",
						"select_option",
						"hover",
						"focus",
						"clear",
					},
				},
				"url":      map[string]any{"type": "string", "description": "URL to navigate to"},
				"selector": map[string]any{"type": "string", "description": "CSS selector of the target element"},
			},
			"required": []string{"action"},
		},
	},
	{
		Name:        "tool_search_tool_regex",
		Description: "Discover hidden MCP tools by regex search when tool discovery is enabled.",
		Category:    "discovery",
		ConfigKey:   "mcp.discovery.use_regex",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Regex pattern to match tool name or description",
				},
			},
			"required": []string{"pattern"},
		},
	},
	{
		Name:        "tool_search_tool_bm25",
		Description: "Discover hidden MCP tools by semantic ranking when tool discovery is enabled.",
		Category:    "discovery",
		ConfigKey:   "mcp.discovery.use_bm25",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "Search query"},
			},
			"required": []string{"query"},
		},
	},
}

func (h *Handler) registerToolRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tools", h.handleListTools)
	mux.HandleFunc("PUT /api/tools/{name}/state", h.handleUpdateToolState)
	mux.HandleFunc("GET /api/tools/web-search-config", h.handleGetWebSearchConfig)
	mux.HandleFunc("PUT /api/tools/web-search-config", h.handleUpdateWebSearchConfig)
}

func (h *Handler) handleListTools(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toolSupportResponse{
		Tools: buildToolSupport(cfg),
	})
}

func (h *Handler) handleUpdateToolState(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	var req toolStateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if err := applyToolState(cfg, r.PathValue("name"), req.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func buildToolSupport(cfg *config.Config) []toolSupportItem {
	items := make([]toolSupportItem, 0, len(toolCatalog))
	for _, entry := range toolCatalog {
		status := "disabled"
		reasonCode := ""

		switch entry.Name {
		case "find_skills", "install_skill":
			if cfg.Tools.IsToolEnabled(entry.ConfigKey) {
				if cfg.Tools.IsToolEnabled("skills") {
					status = "enabled"
				} else {
					status = "blocked"
					reasonCode = "requires_skills"
				}
			}
		case "spawn", "spawn_status":
			if cfg.Tools.IsToolEnabled(entry.ConfigKey) {
				if cfg.Tools.IsToolEnabled("subagent") {
					status = "enabled"
				} else {
					status = "blocked"
					reasonCode = "requires_subagent"
				}
			}
		case "tool_search_tool_regex":
			status, reasonCode = resolveDiscoveryToolSupport(cfg, cfg.Tools.MCP.Discovery.UseRegex)
		case "tool_search_tool_bm25":
			status, reasonCode = resolveDiscoveryToolSupport(cfg, cfg.Tools.MCP.Discovery.UseBM25)
		case "web_search":
			status, reasonCode = resolveWebSearchToolSupport(cfg)
		case "i2c", "spi":
			status, reasonCode = resolveHardwareToolSupport(cfg.Tools.IsToolEnabled(entry.ConfigKey))
		case "browser_ext":
			if cfg.Tools.IsToolEnabled(entry.ConfigKey) {
				if ch := cfg.Channels.Get("browser"); ch != nil && ch.Enabled {
					status = "enabled"
				} else {
					status = "blocked"
					reasonCode = "requires_browser_channel"
				}
			}
		case "serial":
			status, reasonCode = resolveSerialToolSupport(cfg.Tools.IsToolEnabled(entry.ConfigKey))
		default:
			if cfg.Tools.IsToolEnabled(entry.ConfigKey) {
				status = "enabled"
			}
		}

		items = append(items, toolSupportItem{
			Name:        entry.Name,
			Description: entry.Description,
			Category:    entry.Category,
			ConfigKey:   entry.ConfigKey,
			Status:      status,
			ReasonCode:  reasonCode,
			Parameters:  entry.Parameters,
		})
	}
	return items
}

func resolveHardwareToolSupport(enabled bool) (string, string) {
	if !enabled {
		return "disabled", ""
	}
	if runtime.GOOS != "linux" {
		return "blocked", "requires_linux"
	}
	return "enabled", ""
}

func resolveSerialToolSupport(enabled bool) (string, string) {
	if !enabled {
		return "disabled", ""
	}
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		return "enabled", ""
	default:
		return "blocked", "requires_serial_platform"
	}
}

func resolveDiscoveryToolSupport(cfg *config.Config, methodEnabled bool) (string, string) {
	if !cfg.Tools.IsToolEnabled("mcp") {
		return "disabled", ""
	}
	if !cfg.Tools.MCP.Discovery.Enabled {
		return "blocked", "requires_mcp_discovery"
	}
	if !methodEnabled {
		return "disabled", ""
	}
	return "enabled", ""
}

func resolveWebSearchToolSupport(cfg *config.Config) (string, string) {
	if !cfg.Tools.IsToolEnabled("web") {
		return "disabled", ""
	}
	return "enabled", ""
}

func applyToolState(cfg *config.Config, toolName string, enabled bool) error {
	switch toolName {
	case "read_file":
		cfg.Tools.ReadFile.Enabled = enabled
	case "write_file":
		cfg.Tools.WriteFile.Enabled = enabled
	case "list_dir":
		cfg.Tools.ListDir.Enabled = enabled
	case "edit_file":
		cfg.Tools.EditFile.Enabled = enabled
	case "append_file":
		cfg.Tools.AppendFile.Enabled = enabled
	case "exec":
		cfg.Tools.Exec.Enabled = enabled
	case "cron":
		cfg.Tools.Cron.Enabled = enabled
	case "web_search":
		cfg.Tools.Web.Enabled = enabled
	case "web_fetch":
		cfg.Tools.WebFetch.Enabled = enabled
	case "message":
		cfg.Tools.Message.Enabled = enabled
	case "send_file":
		cfg.Tools.SendFile.Enabled = enabled
	case "find_skills":
		cfg.Tools.FindSkills.Enabled = enabled
		if enabled {
			cfg.Tools.Skills.Enabled = true
		}
	case "install_skill":
		cfg.Tools.InstallSkill.Enabled = enabled
		if enabled {
			cfg.Tools.Skills.Enabled = true
		}
	case "spawn":
		cfg.Tools.Spawn.Enabled = enabled
		if enabled {
			cfg.Tools.Subagent.Enabled = true
		}
	case "spawn_status":
		cfg.Tools.SpawnStatus.Enabled = enabled
		if enabled {
			cfg.Tools.Spawn.Enabled = true
			cfg.Tools.Subagent.Enabled = true
		}
	case "i2c":
		cfg.Tools.I2C.Enabled = enabled
	case "spi":
		cfg.Tools.SPI.Enabled = enabled
	case "serial":
		cfg.Tools.Serial.Enabled = enabled
	case "get_current_time":
		cfg.Tools.GetCurrentTime.Enabled = enabled
	case "browser_ext":
		cfg.Tools.BrowserExt.Enabled = enabled
	case "tool_search_tool_regex":
		cfg.Tools.MCP.Discovery.UseRegex = enabled
		if enabled {
			cfg.Tools.MCP.Enabled = true
			cfg.Tools.MCP.Discovery.Enabled = true
		}
	case "tool_search_tool_bm25":
		cfg.Tools.MCP.Discovery.UseBM25 = enabled
		if enabled {
			cfg.Tools.MCP.Enabled = true
			cfg.Tools.MCP.Discovery.Enabled = true
		}
	default:
		return fmt.Errorf("tool %q cannot be updated", toolName)
	}
	return nil
}

func (h *Handler) handleGetWebSearchConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(buildWebSearchConfigResponse(cfg)); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleUpdateWebSearchConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	var req webSearchConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	provider := normalizeWebSearchProvider(req.Provider)
	if provider == "" {
		http.Error(w, "invalid web search provider", http.StatusBadRequest)
		return
	}

	cfg.Tools.Web.Provider = provider
	cfg.Tools.Web.PreferNative = req.PreferNative
	cfg.Tools.Web.Proxy = strings.TrimSpace(req.Proxy)

	if settings, ok := req.Settings["sogou"]; ok {
		cfg.Tools.Web.Sogou.Enabled = settings.Enabled
		cfg.Tools.Web.Sogou.MaxResults = settings.MaxResults
	}
	if settings, ok := req.Settings["duckduckgo"]; ok {
		cfg.Tools.Web.DuckDuckGo.Enabled = settings.Enabled
		cfg.Tools.Web.DuckDuckGo.MaxResults = settings.MaxResults
	}
	if settings, ok := req.Settings["gemini"]; ok {
		cfg.Tools.Web.Gemini.Enabled = settings.Enabled
		cfg.Tools.Web.Gemini.MaxResults = settings.MaxResults
		cfg.Tools.Web.Gemini.Model = strings.TrimSpace(settings.Model)
		if key := strings.TrimSpace(settings.APIKey); key != "" {
			cfg.Tools.Web.Gemini.APIKey = *config.NewSecureString(key)
		}
	}
	if settings, ok := req.Settings["brave"]; ok {
		cfg.Tools.Web.Brave.Enabled = settings.Enabled
		cfg.Tools.Web.Brave.MaxResults = settings.MaxResults
		if keys, ok := normalizeWebSearchAPIKeys(settings.APIKeys, settings.APIKey); ok {
			cfg.Tools.Web.Brave.SetAPIKeys(keys)
		}
	}
	if settings, ok := req.Settings["tavily"]; ok {
		cfg.Tools.Web.Tavily.Enabled = settings.Enabled
		cfg.Tools.Web.Tavily.MaxResults = settings.MaxResults
		cfg.Tools.Web.Tavily.BaseURL = strings.TrimSpace(settings.BaseURL)
		if keys, ok := normalizeWebSearchAPIKeys(settings.APIKeys, settings.APIKey); ok {
			cfg.Tools.Web.Tavily.SetAPIKeys(keys)
		}
	}
	if settings, ok := req.Settings["kagi"]; ok {
		cfg.Tools.Web.Kagi.Enabled = settings.Enabled
		cfg.Tools.Web.Kagi.MaxResults = settings.MaxResults
		cfg.Tools.Web.Kagi.BaseURL = strings.TrimSpace(settings.BaseURL)
		if keys, ok := normalizeWebSearchAPIKeys(settings.APIKeys, settings.APIKey); ok {
			cfg.Tools.Web.Kagi.SetAPIKeys(keys)
		}
	}
	if settings, ok := req.Settings["perplexity"]; ok {
		cfg.Tools.Web.Perplexity.Enabled = settings.Enabled
		cfg.Tools.Web.Perplexity.MaxResults = settings.MaxResults
		if keys, ok := normalizeWebSearchAPIKeys(settings.APIKeys, settings.APIKey); ok {
			cfg.Tools.Web.Perplexity.APIKeys = config.SimpleSecureStrings(keys...)
		}
	}
	if settings, ok := req.Settings["searxng"]; ok {
		cfg.Tools.Web.SearXNG.Enabled = settings.Enabled
		cfg.Tools.Web.SearXNG.MaxResults = settings.MaxResults
		cfg.Tools.Web.SearXNG.BaseURL = strings.TrimSpace(settings.BaseURL)
	}
	if settings, ok := req.Settings["glm_search"]; ok {
		cfg.Tools.Web.GLMSearch.Enabled = settings.Enabled
		cfg.Tools.Web.GLMSearch.MaxResults = settings.MaxResults
		cfg.Tools.Web.GLMSearch.BaseURL = strings.TrimSpace(settings.BaseURL)
		if key := strings.TrimSpace(settings.APIKey); key != "" {
			cfg.Tools.Web.GLMSearch.APIKey = *config.NewSecureString(key)
		}
	}
	if settings, ok := req.Settings["baidu_search"]; ok {
		cfg.Tools.Web.BaiduSearch.Enabled = settings.Enabled
		cfg.Tools.Web.BaiduSearch.MaxResults = settings.MaxResults
		cfg.Tools.Web.BaiduSearch.BaseURL = strings.TrimSpace(settings.BaseURL)
		if key := strings.TrimSpace(settings.APIKey); key != "" {
			cfg.Tools.Web.BaiduSearch.APIKey = *config.NewSecureString(key)
		}
	}

	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(buildWebSearchConfigResponse(cfg)); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func normalizeWebSearchProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "auto":
		return "auto"
	case "sogou",
		"brave",
		"tavily",
		"kagi",
		"duckduckgo",
		"gemini",
		"perplexity",
		"searxng",
		"glm_search",
		"baidu_search":
		return strings.ToLower(strings.TrimSpace(provider))
	default:
		return ""
	}
}

func normalizeWebSearchAPIKeys(apiKeys []string, apiKey string) ([]string, bool) {
	if apiKeys != nil {
		keys := make([]string, 0, len(apiKeys))
		seen := make(map[string]struct{}, len(apiKeys))
		for _, key := range apiKeys {
			trimmed := strings.TrimSpace(key)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			keys = append(keys, trimmed)
		}
		return keys, true
	}

	if trimmed := strings.TrimSpace(apiKey); trimmed != "" {
		return []string{trimmed}, true
	}

	return nil, false
}

func buildWebSearchConfigResponse(cfg *config.Config) webSearchConfigResponse {
	opts := picotools.WebSearchToolOptionsFromConfig(cfg)
	current := resolveCurrentWebSearchProvider(cfg)
	settings := map[string]webSearchProviderConfig{
		"sogou": {
			Enabled:    cfg.Tools.Web.Sogou.Enabled,
			MaxResults: cfg.Tools.Web.Sogou.MaxResults,
		},
		"duckduckgo": {
			Enabled:    cfg.Tools.Web.DuckDuckGo.Enabled,
			MaxResults: cfg.Tools.Web.DuckDuckGo.MaxResults,
		},
		"gemini": {
			Enabled:    cfg.Tools.Web.Gemini.Enabled,
			MaxResults: cfg.Tools.Web.Gemini.MaxResults,
			Model:      cfg.Tools.Web.Gemini.Model,
			APIKeySet:  cfg.Tools.Web.Gemini.APIKey.String() != "",
		},
		"brave": {
			Enabled:    cfg.Tools.Web.Brave.Enabled,
			MaxResults: cfg.Tools.Web.Brave.MaxResults,
			APIKeySet:  len(cfg.Tools.Web.Brave.APIKeys.Values()) > 0,
		},
		"tavily": {
			Enabled:    cfg.Tools.Web.Tavily.Enabled,
			MaxResults: cfg.Tools.Web.Tavily.MaxResults,
			BaseURL:    cfg.Tools.Web.Tavily.BaseURL,
			APIKeySet:  len(cfg.Tools.Web.Tavily.APIKeys.Values()) > 0,
		},
		"kagi": {
			Enabled:    cfg.Tools.Web.Kagi.Enabled,
			MaxResults: cfg.Tools.Web.Kagi.MaxResults,
			BaseURL:    cfg.Tools.Web.Kagi.BaseURL,
			APIKeySet:  len(cfg.Tools.Web.Kagi.APIKeys.Values()) > 0,
		},
		"perplexity": {
			Enabled:    cfg.Tools.Web.Perplexity.Enabled,
			MaxResults: cfg.Tools.Web.Perplexity.MaxResults,
			APIKeySet:  len(cfg.Tools.Web.Perplexity.APIKeys.Values()) > 0,
		},
		"searxng": {
			Enabled:    cfg.Tools.Web.SearXNG.Enabled,
			MaxResults: cfg.Tools.Web.SearXNG.MaxResults,
			BaseURL:    cfg.Tools.Web.SearXNG.BaseURL,
		},
		"glm_search": {
			Enabled:    cfg.Tools.Web.GLMSearch.Enabled,
			MaxResults: cfg.Tools.Web.GLMSearch.MaxResults,
			BaseURL:    cfg.Tools.Web.GLMSearch.BaseURL,
			APIKeySet:  cfg.Tools.Web.GLMSearch.APIKey.String() != "",
		},
		"baidu_search": {
			Enabled:    cfg.Tools.Web.BaiduSearch.Enabled,
			MaxResults: cfg.Tools.Web.BaiduSearch.MaxResults,
			BaseURL:    cfg.Tools.Web.BaiduSearch.BaseURL,
			APIKeySet:  cfg.Tools.Web.BaiduSearch.APIKey.String() != "",
		},
	}

	providers := []webSearchProviderOption{
		{
			ID:         "auto",
			Label:      "Auto",
			Configured: current != "",
			Current: cfg.Tools.Web.Provider == "" ||
				cfg.Tools.Web.Provider == "auto",
		},
		{
			ID:         "sogou",
			Label:      "Sogou",
			Configured: picotools.WebSearchProviderReady(opts, "sogou"),
			Current:    current == "sogou",
		},
		{
			ID:         "duckduckgo",
			Label:      "DuckDuckGo",
			Configured: picotools.WebSearchProviderReady(opts, "duckduckgo"),
			Current:    current == "duckduckgo",
		},
		{
			ID:           "gemini",
			Label:        "Gemini (Google Search)",
			Configured:   picotools.WebSearchProviderReady(opts, "gemini"),
			Current:      current == "gemini",
			RequiresAuth: true,
		},
		{
			ID:           "brave",
			Label:        "Brave Search",
			Configured:   picotools.WebSearchProviderReady(opts, "brave"),
			Current:      current == "brave",
			RequiresAuth: true,
		},
		{
			ID:           "tavily",
			Label:        "Tavily",
			Configured:   picotools.WebSearchProviderReady(opts, "tavily"),
			Current:      current == "tavily",
			RequiresAuth: true,
		},
		{
			ID:           "kagi",
			Label:        "Kagi Search",
			Configured:   picotools.WebSearchProviderReady(opts, "kagi"),
			Current:      current == "kagi",
			RequiresAuth: true,
		},
		{
			ID:           "perplexity",
			Label:        "Perplexity",
			Configured:   picotools.WebSearchProviderReady(opts, "perplexity"),
			Current:      current == "perplexity",
			RequiresAuth: true,
		},
		{
			ID:         "searxng",
			Label:      "SearXNG",
			Configured: picotools.WebSearchProviderReady(opts, "searxng"),
			Current:    current == "searxng",
		},
		{
			ID:           "glm_search",
			Label:        "GLM Search",
			Configured:   picotools.WebSearchProviderReady(opts, "glm_search"),
			Current:      current == "glm_search",
			RequiresAuth: true,
		},
		{
			ID:           "baidu_search",
			Label:        "Baidu Search",
			Configured:   picotools.WebSearchProviderReady(opts, "baidu_search"),
			Current:      current == "baidu_search",
			RequiresAuth: true,
		},
	}

	provider := cfg.Tools.Web.Provider
	if provider == "" {
		provider = "auto"
	}

	return webSearchConfigResponse{
		Provider:       provider,
		CurrentService: current,
		PreferNative:   cfg.Tools.Web.PreferNative,
		Proxy:          cfg.Tools.Web.Proxy,
		Providers:      providers,
		Settings:       settings,
	}
}

func resolveCurrentWebSearchProvider(cfg *config.Config) string {
	if cfg == nil || !cfg.Tools.IsToolEnabled("web") {
		return ""
	}
	selected, err := picotools.ResolveWebSearchProviderName(picotools.WebSearchToolOptionsFromConfig(cfg), "")
	if err != nil {
		return ""
	}
	return selected
}
