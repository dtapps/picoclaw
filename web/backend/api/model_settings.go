package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sipeed/picoclaw/pkg/config"
)

// registerModelSettingsRoutes 注册智能体模型设置相关的 API 端点。
// 这些端点与模型列表的 CRUD 接口分离，避免与上游代码产生冲突。
func (h *Handler) registerModelSettingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/agent/model-settings", h.handleGetModelSettings)
	mux.HandleFunc("PUT /api/agent/model-settings", h.handleUpdateModelSettings)
}

// handleGetModelSettings 返回当前活动模型和备选模型链。
//
//	GET /api/agent/model-settings
func (h *Handler) handleGetModelSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"model_name":      cfg.Agents.Defaults.ModelName,
		"model_fallbacks": cfg.Agents.Defaults.ModelFallbacks,
	})
}

// handleUpdateModelSettings 更新活动模型和备选模型链。
//
//	PUT /api/agent/model-settings
func (h *Handler) handleUpdateModelSettings(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req struct {
		ModelName      string   `json:"model_name"`
		ModelFallbacks []string `json:"model_fallbacks"`
	}
	if err = json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if req.ModelName == "" {
		http.Error(w, "model_name is required", http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	// 验证 model_name 在 model_list 中存在且不是虚拟模型
	found := false
	isVirtual := false
	for _, m := range cfg.ModelList {
		if m.ModelName == req.ModelName {
			found = true
			isVirtual = m.IsVirtual()
			break
		}
	}
	if !found {
		http.Error(w, fmt.Sprintf("Model %q not found in model_list", req.ModelName), http.StatusNotFound)
		return
	}
	if isVirtual {
		http.Error(w, fmt.Sprintf("Cannot set virtual model %q as default", req.ModelName), http.StatusBadRequest)
		return
	}

	// 验证备选模型名称是否存在
	for _, fbName := range req.ModelFallbacks {
		fbFound := false
		for _, m := range cfg.ModelList {
			if m.ModelName == fbName {
				fbFound = true
				break
			}
		}
		if !fbFound {
			http.Error(w, fmt.Sprintf("Fallback model %q not found in model_list", fbName), http.StatusBadRequest)
			return
		}
	}

	cfg.Agents.Defaults.ModelName = req.ModelName
	cfg.Agents.Defaults.ModelFallbacks = req.ModelFallbacks

	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":          "ok",
		"model_name":      req.ModelName,
		"model_fallbacks": req.ModelFallbacks,
	})
}
