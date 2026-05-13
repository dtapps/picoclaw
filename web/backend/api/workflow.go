package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/workflow"
)

// registerWorkflowRoutes 注册所有工作流 API 端点。
// 路由设计遵循 RESTful 风格，支持 CRUD、执行/停止、启用/禁用、实例查询等操作。
// CRUD 操作直接读写文件系统，执行/停止/实例查询通过反向代理转发到网关进程。
func (h *Handler) registerWorkflowRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/workflows", h.handleListWorkflows)                               // 列表
	mux.HandleFunc("POST /api/workflows", h.handleCreateWorkflow)                             // 创建
	mux.HandleFunc("GET /api/workflows/{name}", h.handleGetWorkflow)                          // 详情
	mux.HandleFunc("PUT /api/workflows/{name}", h.handleUpdateWorkflow)                       // 更新
	mux.HandleFunc("DELETE /api/workflows/{name}", h.handleDeleteWorkflow)                    // 删除
	mux.HandleFunc("POST /api/workflows/{name}/run", h.handleRunWorkflow)                     // 执行
	mux.HandleFunc("POST /api/workflows/{name}/stop", h.handleStopWorkflow)                   // 停止
	mux.HandleFunc("POST /api/workflows/{name}/toggle", h.handleToggleWorkflow)               // 启用/禁用
	mux.HandleFunc("GET /api/workflows/{name}/instances", h.handleListInstances)              // 实例列表
	mux.HandleFunc("GET /api/workflows/{name}/instances/{id}", h.handleGetInstance)           // 实例详情
	mux.HandleFunc("GET /api/workflows/{name}/instances/{id}/stream", h.handleStreamInstance) // 实例SSE流
	mux.HandleFunc("DELETE /api/workflows/{name}/instances/{id}", h.handleDeleteInstance)     // 删除实例
	mux.HandleFunc("POST /api/workflows/import", h.handleImportWorkflow)                      // 导入
	mux.HandleFunc("GET /api/workflow/cron-tasks", h.handleCronTasks)                         // Cron 调度列表
}

// getWorkflowStore 从配置创建持久化存储实例（懒初始化 + 缓存，避免每次请求重复读配置文件）。
// CRUD 操作不依赖运行中的网关进程，可直接通过文件系统读写工作流定义。
func (h *Handler) getWorkflowStore() (*workflow.PersistStore, error) {
	h.workflowStoreOnce.Do(func() {
		cfg, err := config.LoadConfig(h.configPath)
		if err != nil {
			h.workflowStoreErr = err
			return
		}
		workspace := cfg.WorkspacePath()
		if workspace == "" {
			workspace = ".picoclaw/workspace"
		}
		h.workflowStore = workflow.NewPersistStore(workspace)
	})
	return h.workflowStore, h.workflowStoreErr
}

// gatewayAvailableForWorkflow 检查网关是否运行且可用于代理请求。
func (h *Handler) gatewayAvailableForWorkflow() bool {
	return h.gatewayAvailableForProxy()
}

// proxyToGateway 向网关内部端点发送请求并转发响应。
func (h *Handler) proxyToGateway(w http.ResponseWriter, method, path string, body io.Reader) {
	target := h.gatewayProxyURL()
	url := target.Scheme + "://" + target.Host + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create proxy request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Workflow engine not running (gateway not started)", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read gateway response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

// proxyToGatewayFast 向网关内部端点发送请求（1秒超时），用于实例查询等对延迟敏感的接口。
// 返回 (响应体, 是否成功)。失败时快速降级，不阻塞调用方。
func (h *Handler) proxyToGatewayFast(method, path string, body io.Reader) ([]byte, bool) {
	if !h.gatewayAvailableForWorkflow() {
		return nil, false
	}
	target := h.gatewayProxyURL()
	url := target.Scheme + "://" + target.Host + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, false
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, false
	}
	return respBody, true
}

// fallbackLoadInstances 从文件系统加载实例列表（网关代理降级路径）。
func (h *Handler) fallbackLoadInstances(w http.ResponseWriter, workflowName string) {
	store, err := h.getWorkflowStore()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"instances": []any{}})
		return
	}
	instances, err := store.LoadInstances(workflowName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"instances": []any{}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"instances": instances})
}

// triggerGatewayReload 通知网关进程重新加载工作流定义。
func (h *Handler) triggerGatewayReload() {
	target := h.gatewayProxyURL()
	url := target.Scheme + "://" + target.Host + "/reload"

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return
	}

	// 使用 PID 文件中的 token 进行认证
	gateway.mu.Lock()
	token := gateway.picoToken
	gateway.mu.Unlock()

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// 3 秒超时，避免阻塞
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// triggerClearWorkflowCache 通知网关进程清除工作流缓存。
// Web 修改工作流后调用，确保触发器使用最新定义。
func (h *Handler) triggerClearWorkflowCache() {
	target := h.gatewayProxyURL()
	url := target.Scheme + "://" + target.Host + "/internal/workflow/clear_cache"

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return
	}

	// 3 秒超时，避免阻塞
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// handleListWorkflows 获取所有工作流列表。
func (h *Handler) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	store, err := h.getWorkflowStore()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to initialize workflow store: %v", err), http.StatusInternalServerError)
		return
	}

	workflows, err := store.LoadAllWorkflows()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load workflows: %v", err), http.StatusInternalServerError)
		return
	}

	items := make([]workflowListItemResponse, 0, len(workflows))
	for _, wf := range workflows {
		items = append(items, workflowListItemResponse{
			Name:        wf.Name,
			Description: wf.Description,
			Enabled:     wf.Enabled,
			TriggerType: describeTriggerType(wf.Triggers),
			Triggers:    wf.Triggers,
			StepCount:   len(wf.Steps),
			Vars:        wf.Vars,
		})
	}
	slices.SortFunc(items, func(a, b workflowListItemResponse) int {
		return strings.Compare(a.Name, b.Name)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"workflows": items,
	})
}

// handleGetWorkflow 获取指定工作流的详细信息。
func (h *Handler) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	store, err := h.getWorkflowStore()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to initialize workflow store: %v", err), http.StatusInternalServerError)
		return
	}

	workflows, err := store.LoadAllWorkflows()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load workflows: %v", err), http.StatusInternalServerError)
		return
	}

	name := r.PathValue("name")
	wf, ok := workflows[name]
	if !ok {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(makeDetailResponse(wf))
}

// handleCreateWorkflow 创建新工作流。
func (h *Handler) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var req createWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	wf := &workflow.Workflow{
		Name:        req.Name,
		Description: req.Description,
		Triggers:    req.Triggers,
		Vars:        req.Vars,
		Steps:       req.Steps,
		Config:      req.Config,
		Enabled:     false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 校验工作流定义的合法性
	if err := wf.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
		return
	}

	store, err := h.getWorkflowStore()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to initialize workflow store: %v", err), http.StatusInternalServerError)
		return
	}

	if store.WorkflowExists(wf.Name) {
		http.Error(w, fmt.Sprintf("Workflow %q already exists", wf.Name), http.StatusConflict)
		return
	}

	if err := store.SaveWorkflow(wf); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create workflow: %v", err), http.StatusInternalServerError)
		return
	}

	// 默认禁用：创建 .disabled 标记文件，确保重新加载后 Enabled 仍为 false
	if err := store.SetEnabled(wf.Name, false); err != nil {
		http.Error(w, fmt.Sprintf("Failed to set workflow disabled: %v", err), http.StatusInternalServerError)
		return
	}

	// 通知网关进程重新加载工作流定义并清除缓存
	go h.triggerGatewayReload()
	go h.triggerClearWorkflowCache()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(makeDetailResponse(wf))
}

// handleImportWorkflow 从 YAML 文本导入工作流。
// 请求体为原始 YAML 文本（Content-Type: text/yaml 或 application/octet-stream）。
func (h *Handler) handleImportWorkflow(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
		return
	}

	wf, err := workflow.ParseYAMLWorkflow(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid YAML: %v", err), http.StatusBadRequest)
		return
	}

	// 设置运行时字段（默认禁用，用户确认后再启用）
	wf.Enabled = false
	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()

	if err = wf.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
		return
	}

	store, oerr := h.getWorkflowStore()
	if oerr != nil {
		http.Error(w, fmt.Sprintf("Failed to initialize workflow store: %v", oerr), http.StatusInternalServerError)
		return
	}

	if store.WorkflowExists(wf.Name) {
		http.Error(w, fmt.Sprintf("Workflow %q already exists", wf.Name), http.StatusConflict)
		return
	}

	if err := store.SaveWorkflow(wf); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save workflow: %v", err), http.StatusInternalServerError)
		return
	}

	// 默认禁用：创建 .disabled 标记文件，确保重新加载后 Enabled 仍为 false
	if err := store.SetEnabled(wf.Name, false); err != nil {
		http.Error(w, fmt.Sprintf("Failed to set workflow disabled: %v", err), http.StatusInternalServerError)
		return
	}

	go h.triggerGatewayReload()
	go h.triggerClearWorkflowCache()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(makeDetailResponse(wf))
}

// handleCronTasks 获取所有启用的工作流 cron 调度列表（下次运行时间）。
// 通过反向代理转发到网关进程的内部 API；网关不可用时返回空列表。
func (h *Handler) handleCronTasks(w http.ResponseWriter, r *http.Request) {
	if body, ok := h.proxyToGatewayFast(http.MethodGet, "/internal/workflow/cron_tasks", nil); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"tasks": []any{}})
}

// handleUpdateWorkflow 更新已有工作流的定义。
func (h *Handler) handleUpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	store, err := h.getWorkflowStore()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to initialize workflow store: %v", err), http.StatusInternalServerError)
		return
	}

	workflows, err := store.LoadAllWorkflows()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load workflows: %v", err), http.StatusInternalServerError)
		return
	}

	name := r.PathValue("name")
	wf, ok := workflows[name]
	if !ok {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	var req updateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// 仅更新请求中提供的字段
	if req.Description != nil {
		wf.Description = *req.Description
	}
	if req.Triggers != nil {
		wf.Triggers = req.Triggers
	}
	if req.Vars != nil {
		wf.Vars = req.Vars
	}
	if req.Steps != nil {
		wf.Steps = req.Steps
	}
	if req.Config != nil {
		wf.Config = *req.Config
	}
	wf.UpdatedAt = time.Now()

	if err := wf.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
		return
	}

	if err := store.SaveWorkflow(wf); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update workflow: %v", err), http.StatusInternalServerError)
		return
	}

	// 通知网关进程重新加载工作流定义并清除缓存
	go h.triggerGatewayReload()
	go h.triggerClearWorkflowCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(makeDetailResponse(wf))
}

// handleDeleteWorkflow 删除指定工作流。
func (h *Handler) handleDeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	store, err := h.getWorkflowStore()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to initialize workflow store: %v", err), http.StatusInternalServerError)
		return
	}

	name := r.PathValue("name")
	if err := store.DeleteWorkflow(name); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete workflow: %v", err), http.StatusInternalServerError)
		return
	}

	// 通知网关进程重新加载工作流定义并清除缓存
	go h.triggerGatewayReload()
	go h.triggerClearWorkflowCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleRunWorkflow 手动触发工作流执行。
// 通过反向代理转发到网关进程的内部 API。
func (h *Handler) handleRunWorkflow(w http.ResponseWriter, r *http.Request) {
	if !h.gatewayAvailableForWorkflow() {
		http.Error(w, "Workflow engine not running (gateway not started)", http.StatusServiceUnavailable)
		return
	}

	name := r.PathValue("name")
	body := strings.NewReader(fmt.Sprintf(`{"name":%q}`, name))
	h.proxyToGateway(w, http.MethodPost, "/internal/workflow/run", body)
}

// handleStopWorkflow 停止指定工作流的所有运行中实例。
// 通过反向代理转发到网关进程的内部 API。
func (h *Handler) handleStopWorkflow(w http.ResponseWriter, r *http.Request) {
	if !h.gatewayAvailableForWorkflow() {
		http.Error(w, "Workflow engine not running (gateway not started)", http.StatusServiceUnavailable)
		return
	}

	name := r.PathValue("name")
	body := strings.NewReader(fmt.Sprintf(`{"name":%q}`, name))
	h.proxyToGateway(w, http.MethodPost, "/internal/workflow/stop", body)
}

// handleToggleWorkflow 切换工作流的启用/禁用状态。
// 同时更新磁盘文件和通知网关进程重新加载。
func (h *Handler) handleToggleWorkflow(w http.ResponseWriter, r *http.Request) {
	store, err := h.getWorkflowStore()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to initialize workflow store: %v", err), http.StatusInternalServerError)
		return
	}

	name := r.PathValue("name")

	// 检查工作流是否存在
	if !store.WorkflowExists(name) {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// 更新磁盘上的禁用标记文件
	if err := store.SetEnabled(name, req.Enabled); err != nil {
		http.Error(w, fmt.Sprintf("Failed to toggle workflow: %v", err), http.StatusInternalServerError)
		return
	}

	// 通知网关进程重新加载工作流定义并清除缓存
	go h.triggerGatewayReload()
	go h.triggerClearWorkflowCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": req.Enabled})
}

// handleListInstances 获取指定工作流的执行实例列表。
// 优先通过反向代理转发到网关进程的内部 API（1秒超时）；
// 网关不可用或超时时，降级从文件系统读取历史记录。
func (h *Handler) handleListInstances(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// 尝试代理到网关内部端点（短超时，快速降级）
	if body, ok := h.proxyToGatewayFast(http.MethodGet, "/internal/workflow/instances?name="+name, nil); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
		return
	}

	// 降级：直接从文件系统读取实例数据
	h.fallbackLoadInstances(w, name)
}

// handleGetInstance 获取指定工作流实例的详细状态。
// 优先通过反向代理转发到网关进程的内部 API（1秒超时）；
// 网关不可用或超时时，降级从文件系统读取。
func (h *Handler) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id := r.PathValue("id")

	// 尝试代理到网关内部端点
	if body, ok := h.proxyToGatewayFast(http.MethodGet, "/internal/workflow/instance?name="+name+"&id="+id, nil); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
		return
	}

	// 降级：直接从文件系统读取实例数据
	store, err := h.getWorkflowStore()
	if err != nil {
		http.Error(w, "Workflow engine not running", http.StatusServiceUnavailable)
		return
	}

	inst, err := store.LoadInstance(name, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Instance not found: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(inst)
}

// handleDeleteInstance 删除指定工作流实例记录。
// 优先通过反向代理转发到网关进程的内部 API（1秒超时）；
// 网关不可用时，降级从文件系统删除。
func (h *Handler) handleDeleteInstance(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id := r.PathValue("id")

	// 尝试代理到网关内部端点
	body := strings.NewReader(fmt.Sprintf(`{"name":%q,"id":%q}`, name, id))
	if _, ok := h.proxyToGatewayFast(http.MethodPost, "/internal/workflow/delete_instance", body); ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	// 降级：直接从文件系统删除
	store, err := h.getWorkflowStore()
	if err != nil {
		http.Error(w, "Workflow engine not running", http.StatusServiceUnavailable)
		return
	}

	if err := store.DeleteInstance(name, id); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete instance: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleStreamInstance 通过 SSE 实时推送工作流实例的状态变更事件。
// 将客户端的 SSE 连接代理到网关进程的 /internal/workflow/stream 端点。
func (h *Handler) handleStreamInstance(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id := r.PathValue("id")

	if !h.gatewayAvailableForWorkflow() {
		http.Error(w, "Workflow engine not running", http.StatusServiceUnavailable)
		return
	}

	target := h.gatewayProxyURL()
	url := target.Scheme + "://" + target.Host + "/internal/workflow/stream?name=" + name + "&id=" + id

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}
	// 传递客户端断开信号
	req = req.WithContext(r.Context())

	// 使用不支持缓冲的 transport 以获得流式响应
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Workflow engine not available", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		http.Error(w, string(respBody), resp.StatusCode)
		return
	}

	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 流式转发网关的 SSE 响应
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	buf := make([]byte, 4096)
	for {
		select {
		case <-r.Context().Done():
			return
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return
			}
			flusher.Flush()
		}
		if err != nil {
			return
		}
	}
}

// --- 响应类型和辅助函数 ---

// workflowListItemResponse 工作流列表项响应格式。
type workflowListItemResponse struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Enabled     bool               `json:"enabled"`
	TriggerType string             `json:"trigger_type"`
	Triggers    []workflow.Trigger `json:"triggers"`
	StepCount   int                `json:"step_count"`
	Vars        map[string]string  `json:"vars,omitempty"`
}

// workflowDetailResponse 工作流详情响应格式。
type workflowDetailResponse struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Enabled     bool                    `json:"enabled"`
	Triggers    []workflow.Trigger      `json:"triggers"`
	Vars        map[string]string       `json:"vars,omitempty"`
	Steps       []workflow.Step         `json:"steps"`
	Config      workflow.WorkflowConfig `json:"config"`
	CreatedAt   any                     `json:"createdAt"`
	UpdatedAt   any                     `json:"updatedAt"`
}

// createWorkflowRequest 创建工作流请求体。
type createWorkflowRequest struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Triggers    []workflow.Trigger      `json:"triggers"`
	Vars        map[string]string       `json:"vars,omitempty"`
	Steps       []workflow.Step         `json:"steps"`
	Config      workflow.WorkflowConfig `json:"config"`
}

// updateWorkflowRequest 更新工作流请求体。
// 所有字段均为可选，仅更新请求中提供的字段。
type updateWorkflowRequest struct {
	Description *string                  `json:"description,omitempty"`
	Triggers    []workflow.Trigger       `json:"triggers,omitempty"`
	Vars        map[string]string        `json:"vars,omitempty"`
	Steps       []workflow.Step          `json:"steps,omitempty"`
	Config      *workflow.WorkflowConfig `json:"config,omitempty"`
}

// makeDetailResponse 从 Workflow 构造详情响应。
func makeDetailResponse(wf *workflow.Workflow) workflowDetailResponse {
	return workflowDetailResponse{
		Name:        wf.Name,
		Description: wf.Description,
		Enabled:     wf.Enabled,
		Triggers:    wf.Triggers,
		Vars:        wf.Vars,
		Steps:       wf.Steps,
		Config:      wf.Config,
		CreatedAt:   wf.CreatedAt,
		UpdatedAt:   wf.UpdatedAt,
	}
}

// describeTriggerType 根据触发器列表返回类型描述（cron/event/manual）。
func describeTriggerType(triggers []workflow.Trigger) string {
	for _, t := range triggers {
		if t.Cron != "" {
			return "cron"
		}
		if t.Event != "" {
			return "event"
		}
	}
	return "manual"
}
