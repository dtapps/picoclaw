package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// InternalAPI 在网关进程的 HTTP 服务器上注册工作流内部端点。
// Web 后端通过反向代理访问这些端点来执行运行时操作（运行、停止、查询实例）。
// 路径以 /internal/workflow/ 为前缀，不与频道 webhook 冲突。
type InternalAPI struct {
	service *Service
}

// NewInternalAPI 创建工作流内部 API 处理器。
func NewInternalAPI(svc *Service) *InternalAPI {
	return &InternalAPI{service: svc}
}

// HandlerMux 是注册 HTTP 处理器的接口，与 health.Server 保持一致。
type HandlerMux interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// RegisterOnMux 将内部端点注册到网关的动态路由器上。
func (a *InternalAPI) RegisterOnMux(mux HandlerMux) {
	mux.HandleFunc("/internal/workflow/run", a.handleRun)
	mux.HandleFunc("/internal/workflow/stop", a.handleStop)
	mux.HandleFunc("/internal/workflow/instances", a.handleInstances)
	mux.HandleFunc("/internal/workflow/instance", a.handleInstance)
	mux.HandleFunc("/internal/workflow/delete_instance", a.handleDeleteInstance)
}

// handleRun 手动触发工作流执行。
// 请求体: {"name": "workflow-name", "channel": "", "chat_id": ""}
func (a *InternalAPI) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name    string `json:"name"`
		Channel string `json:"channel"`
		ChatID  string `json:"chat_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	instanceID, err := a.service.RunWorkflow(context.Background(), req.Name, req.Channel, req.ChatID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"instance_id": instanceID,
		"status":      "running",
	})
}

// handleStop 停止指定工作流的所有运行中实例。
// 请求体: {"name": "workflow-name"}
func (a *InternalAPI) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	instances, err := a.service.GetInstances(req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, inst := range instances {
		if inst.Status == StatusRunning {
			if err := a.service.StopInstance(inst.ID); err != nil {
				http.Error(
					w,
					fmt.Sprintf("Failed to stop instance %s: %v", inst.ID, err),
					http.StatusInternalServerError,
				)
				return
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

// handleInstances 获取指定工作流的执行实例列表。
// 查询参数: name=workflow-name
func (a *InternalAPI) handleInstances(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing name parameter", http.StatusBadRequest)
		return
	}

	instances, err := a.service.GetInstances(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"instances": instances})
}

// handleInstance 获取指定工作流实例的详细状态。
// 查询参数: name=workflow-name&id=instance-id
func (a *InternalAPI) handleInstance(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	id := r.URL.Query().Get("id")
	if name == "" || id == "" {
		http.Error(w, "missing name or id parameter", http.StatusBadRequest)
		return
	}

	inst, err := a.service.GetInstance(name, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(inst)
}

// handleDeleteInstance 删除指定的工作流实例记录。
// 请求体: {"name": "workflow-name", "id": "instance-id"}
func (a *InternalAPI) handleDeleteInstance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if err := a.service.DeleteInstance(req.Name, req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
