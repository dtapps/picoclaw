package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	runtimeevents "github.com/sipeed/picoclaw/pkg/events"
)

// InternalAPI 在网关进程的 HTTP 服务器上注册工作流内部端点。
// Web 后端通过反向代理访问这些端点来执行运行时操作（运行、停止、查询实例）。
// 路径以 /internal/workflow/ 为前缀，不与频道 webhook 冲突。
type InternalAPI struct {
	service  *Service
	eventBus runtimeevents.Bus
}

// NewInternalAPI 创建工作流内部 API 处理器。
func NewInternalAPI(svc *Service) *InternalAPI {
	return &InternalAPI{
		service:  svc,
		eventBus: svc.cfg.EventBus,
	}
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
	mux.HandleFunc("/internal/workflow/stream", a.handleStream)
	mux.HandleFunc("/internal/workflow/clear_cache", a.handleClearCache)
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

// handleStream 通过 SSE 实时推送工作流实例的状态变更事件。
// 查询参数: name=workflow-name&id=instance-id
// 客户端使用 EventSource 或 fetch + ReadableStream 接收事件。
// 事件类型: step_start, step_complete, instance_complete
func (a *InternalAPI) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	instanceID := r.URL.Query().Get("id")
	if name == "" || instanceID == "" {
		http.Error(w, "missing name or id parameter", http.StatusBadRequest)
		return
	}

	if a.eventBus == nil {
		http.Error(w, "event bus not available", http.StatusServiceUnavailable)
		return
	}

	// 先创建订阅，避免 headers 写入后订阅失败无法回退
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	sub, ch, subErr := a.eventBus.Channel().KindPrefix("workflow.").SubscribeChan(ctx, runtimeevents.SubscribeOptions{
		Name:   "workflow-sse-" + instanceID,
		Buffer: 32,
	})
	if subErr != nil {
		http.Error(w, fmt.Sprintf("Failed to subscribe to events: %v", subErr), http.StatusInternalServerError)
		return
	}
	defer sub.Close()

	// 设置 SSE 响应头
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 先发送当前实例状态
	inst, err := a.service.GetInstance(name, instanceID)
	if err == nil && inst != nil {
		data, _ := json.Marshal(inst)
		fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", data)
		flusher.Flush()

		// 如果实例已经处于终态，发送 instance_complete 后立即关闭连接
		if inst.Status == StatusCompleted || inst.Status == StatusFailed || inst.Status == StatusCancelled {
			completeData, _ := json.Marshal(map[string]any{
				"event": string(runtimeevents.KindWorkflowInstanceComplete),
				"payload": map[string]any{
					"workflow": inst.WorkflowName,
					"status":   inst.Status,
					"error":    inst.Error,
				},
				"time": time.Now(),
			})
			fmt.Fprintf(w, "event: instance_complete\ndata: %s\n\n", completeData)
			flusher.Flush()
			return
		}
	}

	// 监听工作流事件，过滤当前实例
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			// 只转发当前实例的事件
			if evt.Scope.RuntimeID != instanceID {
				continue
			}

			data, err := json.Marshal(map[string]any{
				"event":   string(evt.Kind),
				"payload": evt.Payload,
				"time":    evt.Time,
			})
			if err != nil {
				continue
			}

			// 根据 kind 决定 SSE event name
			eventName := "step_update"
			if evt.Kind == runtimeevents.KindWorkflowInstanceComplete {
				eventName = "instance_complete"
			} else if evt.Kind == runtimeevents.KindWorkflowInstanceStart {
				eventName = "instance_start"
			}

			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, data)
			flusher.Flush()

			// 实例完成后关闭连接
			if evt.Kind == runtimeevents.KindWorkflowInstanceComplete {
				return
			}
		}
	}
}

// handleClearCache 清除工作流触发器缓存。
// Web 后端在工作流被修改后调用此端点，确保触发器使用最新定义。
func (a *InternalAPI) handleClearCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a.service.clearTriggerCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
