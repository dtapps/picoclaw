package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/adhocore/gronx"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/commands"
	runtimeevents "github.com/sipeed/picoclaw/pkg/events"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// Service 管理工作流引擎的完整生命周期。
// 负责加载工作流定义、设置触发器、协调执行，以及提供 CRUD API。
// 对标 CronService 的设计模式。
type Service struct {
	store   *PersistStore // 持久化存储
	engine  *Engine       // 核心引擎
	cfg     ServiceConfig // 服务配置
	mu      sync.RWMutex  // 保护 workflows 等运行时状态
	running bool          // 是否正在运行
	stopCh  chan struct{} // 停止信号通道
	doneCh  chan struct{} // runLoop 退出信号，用于同步等待

	workflows map[string]*Workflow // 已加载的工作流定义

	cron     *gronx.Gronx               // Cron 表达式解析器
	eventCh  <-chan runtimeevents.Event // 事件总线订阅通道
	eventSub runtimeevents.Subscription // 事件订阅（用于清理）
}

// ServiceConfig 工作流服务配置。
type ServiceConfig struct {
	WorkspaceDir string             // 工作空间目录
	MsgBus       *bus.MessageBus    // 消息总线
	EventBus     runtimeevents.Bus  // 事件总线
	ToolSchema   ToolSchemaProvider // 工具参数 Schema 查询（可选，用于保存时校验 tool_call 必填参数）
}

// NewService 创建新的工作流服务。
func NewService(store *PersistStore, engine *Engine, cfg ServiceConfig) *Service {
	return &Service{
		store:     store,
		engine:    engine,
		cfg:       cfg,
		workflows: make(map[string]*Workflow),
		cron:      gronx.New(),
	}
}

// Start 加载工作流定义并启动触发器评估循环。
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	// 初始化存储目录
	if err := s.store.Init(); err != nil {
		return err
	}

	// 加载工作流定义
	if err := s.reloadWorkflowsUnsafe(); err != nil {
		return err
	}

	// 订阅事件总线
	if s.cfg.EventBus != nil {
		sub, ch, err := s.cfg.EventBus.Channel().SubscribeChan(
			context.Background(),
			runtimeevents.SubscribeOptions{Name: "workflow-triggers", Buffer: 16},
		)
		if err == nil {
			s.eventSub = sub
			s.eventCh = ch
		} else {
			logger.ErrorCF("workflow", "订阅事件总线失败", map[string]any{"error": err.Error()})
		}
	}

	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	s.running = true

	// 启动主循环
	go s.runLoop()

	logger.InfoCF("workflow", "服务已启动", map[string]any{"workflows": len(s.workflows)})
	return nil
}

// Stop 停止工作流服务。
func (s *Service) Stop() {
	s.mu.Lock()

	if !s.running {
		s.mu.Unlock()
		return
	}

	s.running = false
	if s.stopCh != nil {
		close(s.stopCh)
	}

	// 清理事件订阅
	if s.eventSub != nil {
		s.eventSub.Close()
		s.eventSub = nil
	}

	doneCh := s.doneCh
	s.mu.Unlock()

	// 等待 runLoop goroutine 退出，避免临时目录清理时仍有文件写入
	if doneCh != nil {
		<-doneCh
	}

	// 等待所有运行中的工作流实例完成
	s.engine.WaitRunning()

	logger.InfoC("workflow", "服务已停止")
}

// Reload 重新从磁盘加载工作流定义。
func (s *Service) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reloadWorkflowsUnsafe()
}

// RunWorkflow 手动触发指定工作流的执行。
// channel 和 chatID 用于绑定频道，执行完成后通过回调通知该频道。
func (s *Service) RunWorkflow(ctx context.Context, name, channel, chatID string) (string, error) {
	// 从磁盘读取最新定义，避免用过期内存状态执行旧步骤
	freshWf, err := s.store.LoadSingleWorkflow(name)
	if err != nil {
		return "", errNotFound(name)
	}

	return s.engine.RunWorkflow(ctx, freshWf, "manual", channel, chatID)
}

// StopInstance 取消正在运行的实例。
func (s *Service) StopInstance(instanceID string) error {
	return s.engine.StopInstance(instanceID)
}

// GetWorkflow 获取已加载的工作流定义。
func (s *Service) GetWorkflow(name string) (*Workflow, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	wf, ok := s.workflows[name]
	return wf, ok
}

// ListWorkflows 返回所有已加载的工作流定义。
func (s *Service) ListWorkflows() []*Workflow {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Workflow, 0, len(s.workflows))
	for _, wf := range s.workflows {
		result = append(result, wf)
	}
	return result
}

// CreateWorkflow 创建新工作流并持久化。
// 会先校验定义合法性，然后写入 YAML 文件。
func (s *Service) CreateWorkflow(wf *Workflow) error {
	if err := wf.ValidateWithToolSchema(s.cfg.ToolSchema); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.store.WorkflowExists(wf.Name) {
		return errAlreadyExists(wf.Name)
	}

	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()
	wf.Enabled = false

	if err := s.store.SaveWorkflow(wf); err != nil {
		return err
	}

	s.workflows[wf.Name] = wf
	return nil
}

// UpdateWorkflow 更新已有工作流定义。
func (s *Service) UpdateWorkflow(wf *Workflow) error {
	if err := wf.ValidateWithToolSchema(s.cfg.ToolSchema); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.store.WorkflowExists(wf.Name) {
		return errNotFound(wf.Name)
	}

	wf.UpdatedAt = time.Now()

	if err := s.store.SaveWorkflow(wf); err != nil {
		return err
	}

	s.workflows[wf.Name] = wf
	return nil
}

// DeleteWorkflow 删除工作流。
func (s *Service) DeleteWorkflow(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.store.WorkflowExists(name) {
		return errNotFound(name)
	}

	if err := s.store.DeleteWorkflow(name); err != nil {
		return err
	}

	delete(s.workflows, name)
	return nil
}

// BindChannel 将频道信息绑定到工作流配置，执行完成后自动通知该频道。
// 通常由 Agent 通过命令触发，从上下文中提取 channel/chatID。
func (s *Service) BindChannel(name, channel, chatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.workflows[name]; !ok {
		return errNotFound(name)
	}

	// 从磁盘重新读取最新数据，避免用过期内存状态覆写外部修改（如 Web UI 编辑）
	freshWf, err := s.store.LoadSingleWorkflow(name)
	if err != nil {
		return err
	}

	freshWf.Config.NotifyChannel = channel
	freshWf.Config.NotifyChatID = chatID
	freshWf.UpdatedAt = time.Now()

	if err := s.store.SaveWorkflow(freshWf); err != nil {
		return err
	}

	// 同步更新内存状态
	s.workflows[name] = freshWf
	return nil
}

// UnbindChannel 移除工作流的频道绑定。
func (s *Service) UnbindChannel(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.workflows[name]; !ok {
		return errNotFound(name)
	}

	// 从磁盘重新读取最新数据，避免用过期内存状态覆写外部修改
	freshWf, err := s.store.LoadSingleWorkflow(name)
	if err != nil {
		return err
	}

	freshWf.Config.NotifyChannel = ""
	freshWf.Config.NotifyChatID = ""
	freshWf.UpdatedAt = time.Now()

	if err := s.store.SaveWorkflow(freshWf); err != nil {
		return err
	}

	s.workflows[name] = freshWf
	return nil
}

// SetEnabled 设置工作流的启用/禁用状态。
func (s *Service) SetEnabled(name string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.workflows[name]; !ok {
		return errNotFound(name)
	}

	if err := s.store.SetEnabled(name, enabled); err != nil {
		return err
	}

	// 从磁盘重新读取以同步内存状态（包括 Enabled 和可能被外部修改的其他字段）
	freshWf, err := s.store.LoadSingleWorkflow(name)
	if err != nil {
		return err
	}
	s.workflows[name] = freshWf
	return nil
}

// ListWorkflowsForCommand 返回用于 /workflow 命令的工作流摘要列表。
func (s *Service) ListWorkflowsForCommand() []commands.WorkflowInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]commands.WorkflowInfo, 0, len(s.workflows))
	for _, wf := range s.workflows {
		result = append(result, commands.WorkflowInfo{
			Name:        wf.Name,
			Description: wf.Description,
			Enabled:     wf.Enabled,
			TriggerType: describeTriggerType(wf.Triggers),
			StepCount:   len(wf.Steps),
			Vars:        wf.Vars,
		})
	}
	return result
}

// ShowWorkflow 返回用于 /workflow show 命令的工作流详情。
func (s *Service) ShowWorkflow(name string) (*commands.WorkflowInfo, []string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wf, ok := s.workflows[name]
	if !ok {
		return nil, nil, errNotFound(name)
	}

	stepIDs := make([]string, 0, len(wf.Steps))
	for _, step := range wf.Steps {
		stepIDs = append(stepIDs, step.ID)
	}

	info := &commands.WorkflowInfo{
		Name:        wf.Name,
		Description: wf.Description,
		Enabled:     wf.Enabled,
		TriggerType: describeTriggerType(wf.Triggers),
		StepCount:   len(wf.Steps),
		Vars:        wf.Vars,
	}
	return info, stepIDs, nil
}

// InstancesForCommand 返回用于 /workflow instances 命令的实例摘要列表。
func (s *Service) InstancesForCommand(name string) ([]commands.WorkflowInstanceInfo, error) {
	instances, err := s.store.LoadInstances(name)
	if err != nil {
		return nil, err
	}

	result := make([]commands.WorkflowInstanceInfo, 0, len(instances))
	for _, inst := range instances {
		startedAt := ""
		if !inst.StartedAt.IsZero() {
			startedAt = inst.StartedAt.Format("2006-01-02 15:04:05")
		}
		finishedAt := ""
		if inst.FinishedAt != nil {
			finishedAt = inst.FinishedAt.Format("2006-01-02 15:04:05")
		}
		result = append(result, commands.WorkflowInstanceInfo{
			ID:           inst.ID,
			WorkflowName: inst.WorkflowName,
			Status:       inst.Status,
			TriggerType:  inst.TriggerType,
			StartedAt:    startedAt,
			FinishedAt:   finishedAt,
			Error:        inst.Error,
		})
	}
	return result, nil
}

// describeTriggerType 返回人类可读的触发器类型标签。
func describeTriggerType(triggers []Trigger) string {
	if len(triggers) == 0 {
		return "manual"
	}
	var parts []string
	for _, t := range triggers {
		if t.Cron != "" {
			parts = append(parts, "cron:"+t.Cron)
		}
		if t.Event != "" {
			parts = append(parts, "event:"+t.Event)
		}
	}
	if len(parts) == 0 {
		return "manual"
	}
	return fmt.Sprintf("%s", parts[0])
}

// GetInstances 获取工作流的执行历史。
func (s *Service) GetInstances(name string) ([]*WorkflowInstance, error) {
	return s.store.LoadInstances(name)
}

// GetInstance 获取指定的运行实例。
func (s *Service) GetInstance(workflowName, instanceID string) (*WorkflowInstance, error) {
	return s.store.LoadInstance(workflowName, instanceID)
}

// DeleteInstance 删除指定的运行实例记录。
func (s *Service) DeleteInstance(workflowName, instanceID string) error {
	return s.store.DeleteInstance(workflowName, instanceID)
}

// runLoop 主事件循环，定期检查 cron 触发器和事件触发器。
func (s *Service) runLoop() {
	defer close(s.doneCh)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkCronTriggers()
		case evt, ok := <-s.eventCh:
			if !ok {
				return
			}
			s.checkEventTriggers(evt)
		}
	}
}

// checkCronTriggers 评估所有 cron 触发器，触发到期的工作流。
func (s *Service) checkCronTriggers() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, wf := range s.workflows {
		if !wf.Enabled {
			continue
		}
		for _, trigger := range wf.Triggers {
			if trigger.Cron == "" {
				continue
			}

			// 检查 cron 表达式是否到期
			isDue, err := s.cron.IsDue(trigger.Cron)
			if err != nil {
				logger.WarnCF(
					"workflow",
					"无效的 cron 表达式",
					map[string]any{"cron": trigger.Cron, "workflow": wf.Name, "error": err.Error()},
				)
				continue
			}
			if !isDue {
				continue
			}

			// 避免重复触发：同一工作流同时只运行一个实例
			if s.engine.IsRunning(wf.Name) {
				logger.InfoCF("workflow", "跳过 cron 触发，工作流正在运行中", map[string]any{"workflow": wf.Name})
				continue
			}

			logger.InfoCF("workflow", "cron 触发器触发工作流", map[string]any{"workflow": wf.Name})
			// 从磁盘读取最新定义，避免用过期步骤执行
			freshWf, err := s.store.LoadSingleWorkflow(wf.Name)
			if err != nil {
				logger.ErrorCF("workflow", "读取工作流定义失败", map[string]any{"workflow": wf.Name, "error": err.Error()})
				continue
			}
			ctx := context.Background()
			if _, err := s.engine.RunWorkflow(ctx, freshWf, "cron", "", ""); err != nil {
				logger.ErrorCF("workflow", "运行工作流失败", map[string]any{"workflow": wf.Name, "error": err.Error()})
			}
		}
	}
}

// checkEventTriggers 评估事件触发器。
func (s *Service) checkEventTriggers(evt runtimeevents.Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, wf := range s.workflows {
		if !wf.Enabled {
			continue
		}
		for _, trigger := range wf.Triggers {
			if trigger.Event == "" {
				continue
			}
			// 匹配事件类型
			if string(evt.Kind) == trigger.Event {
				if s.engine.IsRunning(wf.Name) {
					logger.InfoCF("workflow", "跳过事件触发，工作流正在运行中", map[string]any{"workflow": wf.Name})
					continue
				}

				logger.InfoCF("workflow", "事件触发器触发工作流", map[string]any{"event": trigger.Event, "workflow": wf.Name})
				// 从磁盘读取最新定义，避免用过期步骤执行
				freshWf, err := s.store.LoadSingleWorkflow(wf.Name)
				if err != nil {
					logger.ErrorCF("workflow", "读取工作流定义失败", map[string]any{"workflow": wf.Name, "error": err.Error()})
					continue
				}
				ctx := context.Background()
				if _, err := s.engine.RunWorkflow(ctx, freshWf, "event", "", ""); err != nil {
					logger.ErrorCF("workflow", "运行工作流失败", map[string]any{"workflow": wf.Name, "error": err.Error()})
				}
			}
		}
	}
}

// reloadWorkflowsUnsafe 从磁盘重新加载工作流定义（调用者需持有锁）。
func (s *Service) reloadWorkflowsUnsafe() error {
	workflows, err := s.store.LoadAllWorkflows()
	if err != nil {
		return err
	}
	s.workflows = workflows
	return nil
}

// --- 错误类型 ---

type (
	notFoundError      string
	alreadyExistsError string
)

func (e notFoundError) Error() string      { return "工作流不存在: " + string(e) }
func (e alreadyExistsError) Error() string { return "工作流已存在: " + string(e) }

func errNotFound(name string) notFoundError           { return notFoundError(name) }
func errAlreadyExists(name string) alreadyExistsError { return alreadyExistsError(name) }
