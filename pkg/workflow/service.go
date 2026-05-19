package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/adhocore/gronx"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/commands"
	runtimeevents "github.com/sipeed/picoclaw/pkg/events"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// timeNow 用于获取当前时间，测试时可以注入模拟时间
var timeNow = time.Now

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

	// 触发器检查缓存，避免每分钟都从磁盘读取
	triggerCacheModTime int64                // 上次加载时的工作流目录修改时间
	triggerCacheData    map[string]*Workflow // 缓存的工作流数据
	triggerCacheMu      sync.RWMutex         // 保护缓存

	cron     *gronx.Gronx               // Cron 表达式解析器
	eventCh  <-chan runtimeevents.Event // 事件总线订阅通道（用于触发器）
	eventSub runtimeevents.Subscription // 事件订阅（用于清理）

	// 工作流完成事件订阅
	completeEventCh  <-chan runtimeevents.Event // 工作流完成事件通道
	completeEventSub runtimeevents.Subscription // 完成事件订阅（用于清理）

	// cron 触发记录：避免重复 + 补偿漏触
	lastCronFireMu sync.Mutex
	lastCronFire   map[string]time.Time // "workflowName|cronExpr" → 上次触发时间
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
		store:        store,
		engine:       engine,
		cfg:          cfg,
		workflows:    make(map[string]*Workflow),
		cron:         gronx.New(),
		lastCronFire: make(map[string]time.Time),
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

	// 订阅事件总线（用于触发器）
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

		// 订阅工作流完成事件（用于更新 LastRunAt 和 LastRunStatus）
		completeSub, completeCh, err := s.cfg.EventBus.Channel().SubscribeChan(
			context.Background(),
			runtimeevents.SubscribeOptions{
				Name:   "workflow-complete",
				Buffer: 16,
			},
		)
		if err == nil {
			s.completeEventSub = completeSub
			s.completeEventCh = completeCh
		} else {
			logger.ErrorCF("workflow", "订阅工作流完成事件失败", map[string]any{"error": err.Error()})
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
	if s.completeEventSub != nil {
		s.completeEventSub.Close()
		s.completeEventSub = nil
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

// GetWorkflow 获取工作流定义。
// 优先从内存缓存读取，若未命中则尝试从磁盘加载（覆盖 Web UI 等外部创建的场景）。
func (s *Service) GetWorkflow(name string) (*Workflow, bool) {
	s.mu.RLock()
	wf, ok := s.workflows[name]
	s.mu.RUnlock()
	if ok {
		return wf, true
	}

	if !s.store.WorkflowExists(name) {
		return nil, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if wf, ok := s.workflows[name]; ok {
		return wf, true
	}
	freshWf, err := s.store.LoadSingleWorkflow(name)
	if err != nil {
		return nil, false
	}
	s.workflows[name] = freshWf
	return freshWf, true
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
	s.clearTriggerCache()
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
	s.clearTriggerCache()
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
	s.clearTriggerCache()
	return nil
}

// chatIDEqual 比较两个 chatID 是否相等，处理带前缀和不带前缀的情况
// 例如 "group:123" 和 "123" 被视为相同（如果 strippedChatID 是 "123"）
func chatIDEqual(storedChatID, newChatID string) bool {
	if storedChatID == newChatID {
		return true
	}
	// 处理前缀情况：如果去掉前缀后相等，也视为相同
	// 支持 group: 和 direct: 前缀
	for _, prefix := range []string{"group:", "direct:"} {
		if strings.HasPrefix(storedChatID, prefix) {
			stripped := strings.TrimPrefix(storedChatID, prefix)
			if stripped == newChatID {
				return true
			}
		}
		if strings.HasPrefix(newChatID, prefix) {
			stripped := strings.TrimPrefix(newChatID, prefix)
			if stripped == storedChatID {
				return true
			}
		}
	}
	return false
}

// BindChannel 将频道信息绑定到工作流配置，执行完成后自动通知该频道。
// 通常由 Agent 通过命令触发，从上下文中提取 channel/chatID。
// BindChannel 将当前频道添加到工作流的通知目标列表。
// 如果该频道已存在，则更新其 chatID；否则追加到列表末尾。
func (s *Service) BindChannel(name, channel, chatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.workflows[name]; !ok {
		if !s.store.WorkflowExists(name) {
			return errNotFound(name)
		}
		freshWf, err := s.store.LoadSingleWorkflow(name)
		if err != nil {
			return err
		}
		s.workflows[name] = freshWf
	}

	freshWf, err := s.store.LoadSingleWorkflow(name)
	if err != nil {
		return err
	}

	// 查找是否已存在相同的频道和 chatID 组合（处理前缀兼容）
	found := false
	for i, target := range freshWf.Config.NotifyChannels {
		if target.Channel == channel && chatIDEqual(target.ChatID, chatID) {
			// 已存在相同的绑定，更新 chatID 为新的格式（带前缀）并更新绑定时间
			freshWf.Config.NotifyChannels[i].ChatID = chatID
			freshWf.Config.NotifyChannels[i].BoundAt = time.Now()
			found = true
			break
		}
	}

	// 如果不存在，则追加新的绑定
	if !found {
		freshWf.Config.NotifyChannels = append(freshWf.Config.NotifyChannels, NotifyTarget{
			Channel: channel,
			ChatID:  chatID,
			BoundAt: time.Now(),
		})
	}

	freshWf.UpdatedAt = time.Now()

	if err := s.store.SaveWorkflow(freshWf); err != nil {
		return err
	}

	s.workflows[name] = freshWf
	return nil
}

// UnbindChannel 移除工作流的频道绑定。
// 如果 channel 和 chatID 为空，则清空所有通知目标；否则移除匹配的特定目标。
func (s *Service) UnbindChannel(name, channel, chatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.workflows[name]; !ok {
		if !s.store.WorkflowExists(name) {
			return errNotFound(name)
		}
		freshWf, err := s.store.LoadSingleWorkflow(name)
		if err != nil {
			return err
		}
		s.workflows[name] = freshWf
	}

	freshWf, err := s.store.LoadSingleWorkflow(name)
	if err != nil {
		return err
	}

	if channel == "" && chatID == "" {
		// 清空所有通知目标
		freshWf.Config.NotifyChannels = nil
	} else {
		// 移除匹配的通知目标（处理前缀兼容）
		targets := freshWf.Config.NotifyChannels[:0]
		for _, target := range freshWf.Config.NotifyChannels {
			if target.Channel != channel || !chatIDEqual(target.ChatID, chatID) {
				targets = append(targets, target)
			}
		}
		freshWf.Config.NotifyChannels = targets
	}

	freshWf.UpdatedAt = time.Now()

	if err := s.store.SaveWorkflow(freshWf); err != nil {
		return err
	}

	s.workflows[name] = freshWf
	return nil
}

// GetNotifyChannels 获取工作流的所有通知目标。
func (s *Service) GetNotifyChannels(name string) ([]NotifyTarget, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wf, ok := s.workflows[name]
	if !ok {
		if !s.store.WorkflowExists(name) {
			return nil, errNotFound(name)
		}
		var err error
		wf, err = s.store.LoadSingleWorkflow(name)
		if err != nil {
			return nil, err
		}
	}

	return wf.Config.GetNotifyTargets(), nil
}

// SetEnabled 设置工作流的启用/禁用状态。
func (s *Service) SetEnabled(name string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.workflows[name]; !ok {
		if !s.store.WorkflowExists(name) {
			return errNotFound(name)
		}
		freshWf, err := s.store.LoadSingleWorkflow(name)
		if err != nil {
			return err
		}
		s.workflows[name] = freshWf
	}

	if err := s.store.SetEnabled(name, enabled); err != nil {
		return err
	}

	freshWf, err := s.store.LoadSingleWorkflow(name)
	if err != nil {
		return err
	}
	s.workflows[name] = freshWf
	return nil
}

// ListWorkflowsForCommand 返回用于 /workflow 命令的工作流摘要列表。
// 从磁盘读取最新定义，确保显示 Web 修改后的数据。
func (s *Service) ListWorkflowsForCommand() []commands.WorkflowInfo {
	// 从磁盘加载所有工作流，获取最新定义
	workflows, err := s.store.LoadAllWorkflows()
	if err != nil {
		return nil
	}

	result := make([]commands.WorkflowInfo, 0, len(workflows))
	for _, wf := range workflows {
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
// 从磁盘读取最新定义，确保显示 Web 修改后的数据。
func (s *Service) ShowWorkflow(name string) (*commands.WorkflowInfo, []string, error) {
	// 先从磁盘读取最新定义
	wf, err := s.store.LoadSingleWorkflow(name)
	if err != nil {
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
		Triggers:    formatTriggers(wf.Triggers),
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

// CronListForCommand 返回所有待执行的调度任务列表（包括 cron、at、interval）。
// 从磁盘读取最新定义，确保显示 Web 修改后的数据。
func (s *Service) CronListForCommand() []commands.CronTaskInfo {
	// 从磁盘加载所有工作流，获取最新定义
	workflows, err := s.store.LoadAllWorkflows()
	if err != nil {
		return nil
	}

	var result []commands.CronTaskInfo

	for _, wf := range workflows {
		if !wf.Enabled {
			continue
		}
		for _, trigger := range wf.Triggers {
			tz := trigger.TZ
			if tz == "" {
				tz = "UTC"
			}
			loc, err := time.LoadLocation(tz)
			if err != nil {
				loc = time.UTC
			}

			// 处理 Cron 触发器
			if trigger.Cron != "" {
				nextRun, err := gronx.NextTick(trigger.Cron, false)
				if err != nil {
					continue
				}
				result = append(result, commands.CronTaskInfo{
					WorkflowName: wf.Name,
					TriggerType:  "cron",
					Schedule:     trigger.Cron,
					Timezone:     tz,
					NextRun:      nextRun.In(loc).Format("2006-01-02 15:04:05 MST"),
				})
				continue
			}

			// 处理 At 触发器（一次性执行）
			if trigger.At != "" {
				nextRun, err := time.ParseInLocation("2006-01-02 15:04:05", trigger.At, loc)
				if err != nil {
					// 尝试其他格式
					nextRun, err = time.ParseInLocation("2006-01-02 15:04", trigger.At, loc)
				}
				if err != nil {
					continue
				}
				// 只显示未来的任务
				if nextRun.After(time.Now()) {
					result = append(result, commands.CronTaskInfo{
						WorkflowName: wf.Name,
						TriggerType:  "at",
						Schedule:     trigger.At,
						Timezone:     tz,
						NextRun:      nextRun.In(loc).Format("2006-01-02 15:04:05 MST"),
					})
				}
				continue
			}

			// 处理 Interval 触发器（间隔执行）
			if trigger.Interval != "" {
				duration, err := time.ParseDuration(trigger.Interval)
				if err != nil {
					continue
				}
				// 对于 interval，显示为 "每 X 分钟/小时"
				nextRun := time.Now().Add(duration)
				result = append(result, commands.CronTaskInfo{
					WorkflowName: wf.Name,
					TriggerType:  "interval",
					Schedule:     trigger.Interval,
					Timezone:     tz,
					NextRun:      nextRun.In(loc).Format("2006-01-02 15:04:05 MST"),
				})
			}
		}
	}
	return result
}

// describeTriggerType 返回人类可读的触发器类型标签（用于列表显示）。
func describeTriggerType(triggers []Trigger) string {
	if len(triggers) == 0 {
		return "manual"
	}
	var parts []string
	for _, t := range triggers {
		if t.Cron != "" {
			parts = append(parts, "cron")
		} else if t.Event != "" {
			parts = append(parts, "event")
		} else {
			parts = append(parts, "manual")
		}
	}
	if len(parts) == 0 {
		return "manual"
	}
	return strings.Join(parts, ", ")
}

// formatTriggers 返回格式化的触发器详情列表。
func formatTriggers(triggers []Trigger) []string {
	if len(triggers) == 0 {
		return []string{"manual"}
	}
	var result []string
	for i, t := range triggers {
		if t.Cron != "" {
			tz := t.TZ
			if tz == "" {
				tz = "UTC"
			}
			result = append(result, fmt.Sprintf("%d. cron: %s (tz: %s)", i+1, t.Cron, tz))
		} else if t.Event != "" {
			result = append(result, fmt.Sprintf("%d. event: %s", i+1, t.Event))
		} else {
			result = append(result, fmt.Sprintf("%d. manual", i+1))
		}
	}
	return result
}

// GetInstances 获取工作流的执行历史（返回摘要格式）。
func (s *Service) GetInstances(name string) ([]*WorkflowInstanceSummary, error) {
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

	ticker := time.NewTicker(10 * time.Second)
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
		case evt, ok := <-s.completeEventCh:
			if !ok {
				return
			}
			// 处理工作流完成事件，更新 LastRunAt 和 LastRunStatus
			if evt.Kind == runtimeevents.KindWorkflowInstanceComplete {
				s.handleWorkflowCompleteEvent(evt)
			}
		}
	}
}

// getWorkflowsForTriggerCheck 获取用于触发器检查的工作流列表。
// 使用缓存机制：只有当工作流目录修改时间变化时才重新加载。
func (s *Service) getWorkflowsForTriggerCheck() map[string]*Workflow {
	s.triggerCacheMu.Lock()
	defer s.triggerCacheMu.Unlock()

	// 获取当前目录修改时间
	currentModTime, err := s.store.GetWorkflowsDirModTime()
	if err != nil {
		// 如果无法获取修改时间，使用缓存数据（如果有）或从磁盘加载
		if s.triggerCacheData != nil {
			return s.triggerCacheData
		}
		workflows, _ := s.store.LoadAllWorkflows()
		return workflows
	}

	// 如果修改时间没有变化，返回缓存数据
	if currentModTime == s.triggerCacheModTime && s.triggerCacheData != nil {
		return s.triggerCacheData
	}

	// 修改时间变化，重新加载
	workflows, err := s.store.LoadAllWorkflows()
	if err != nil {
		// 加载失败，使用缓存数据（如果有）
		if s.triggerCacheData != nil {
			return s.triggerCacheData
		}
		return make(map[string]*Workflow)
	}

	// 更新缓存
	s.triggerCacheModTime = currentModTime
	s.triggerCacheData = workflows
	return workflows
}

// checkCronTriggers 评估所有 cron 触发器，触发到期的工作流。
// 使用缓存机制避免每分钟都从磁盘读取。
// isCronDue 检查 cron 表达式在指定时区下是否到期。
// 使用 NextTickAfter 计算下一次应该触发的时间，然后检查当前时间是否在该时间的一分钟窗口内。
// 如果 tz 为空则使用 UTC 时区。
func (s *Service) isCronDue(cronExpr, tz string) (bool, error) {
	// 获取当前时间（使用 timeNow 以便测试时可以注入）
	now := timeNow()

	// 如果指定了时区，将时间转换为该时区
	loc := time.UTC
	if tz != "" {
		loadedLoc, err := time.LoadLocation(tz)
		if err == nil {
			loc = loadedLoc
		}
	}
	now = now.In(loc)

	// 使用 gronx.IsDue 检查当前时间是否应该触发
	// 先检查精确时间点
	g := gronx.New()
	isDue, err := g.IsDue(cronExpr, now)
	if err != nil {
		return false, err
	}
	if isDue {
		return true, nil
	}

	// 如果精确时间点不匹配，检查是否在当前分钟内
	// 将时间截断到分钟，检查这一分钟是否应该触发
	truncated := now.Truncate(time.Minute)
	isDue, err = g.IsDue(cronExpr, truncated)
	if err != nil {
		return false, err
	}
	return isDue, nil
}

func (s *Service) checkCronTriggers() {
	// 获取工作流（使用缓存机制）
	workflows := s.getWorkflowsForTriggerCheck()

	logger.DebugCF("workflow", "检查调度触发器", map[string]any{"workflows": len(workflows)})

	for _, wf := range workflows {
		if !wf.Enabled {
			logger.DebugCF("workflow", "工作流未启用，跳过", map[string]any{"workflow": wf.Name})
			continue
		}
		for i, trigger := range wf.Triggers {
			// 避免重复触发：同一工作流同时只运行一个实例
			if s.engine.IsRunning(wf.Name) {
				logger.WarnCF("workflow", "跳过触发，工作流正在运行中", map[string]any{"workflow": wf.Name})
				continue
			}

			// 处理 Cron 触发器
			if trigger.Cron != "" {
				s.checkAndFireCronTrigger(wf, i, trigger)
				continue
			}

			// 处理 At 触发器（一次性执行）
			if trigger.At != "" {
				s.checkAndFireAtTrigger(wf, i, trigger)
				continue
			}

			// 处理 Interval 触发器（间隔执行）
			if trigger.Interval != "" {
				s.checkAndFireIntervalTrigger(wf, i, trigger)
			}
		}
	}
}

// checkAndFireCronTrigger 检查并触发 Cron 触发器
func (s *Service) checkAndFireCronTrigger(wf *Workflow, index int, trigger Trigger) {
	fireKey := wf.Name + "|cron|" + trigger.Cron

	logger.DebugCF("workflow", "检查 cron 表达式",
		map[string]any{"workflow": wf.Name, "cron": trigger.Cron, "tz": trigger.TZ})

	// 检查 cron 表达式是否到期（使用触发器配置的时区）
	isDue, err := s.isCronDue(trigger.Cron, trigger.TZ)
	if err != nil {
		logger.WarnCF(
			"workflow",
			"无效的 cron 表达式",
			map[string]any{"cron": trigger.Cron, "workflow": wf.Name, "error": err.Error()},
		)
		return
	}
	if !isDue {
		logger.DebugCF("workflow", "cron 未到期",
			map[string]any{"workflow": wf.Name, "cron": trigger.Cron})
		return
	}

	// 去重：同一工作流同一 cron 表达式在 90 秒内不重复触发
	s.lastCronFireMu.Lock()
	lastFire, fired := s.lastCronFire[fireKey]
	s.lastCronFireMu.Unlock()
	if fired && time.Since(lastFire) < 90*time.Second {
		logger.DebugCF("workflow", "cron 触发去重",
			map[string]any{"workflow": wf.Name, "cron": trigger.Cron, "last_fire": lastFire})
		return
	}

	// 记录本次触发
	s.lastCronFireMu.Lock()
	s.lastCronFire[fireKey] = timeNow()
	s.lastCronFireMu.Unlock()

	logger.InfoCF("workflow", "cron 触发器触发工作流",
		map[string]any{"workflow": wf.Name, "trigger_index": index})
	ctx := context.Background()
	if _, err := s.engine.RunWorkflow(ctx, wf, "cron", "", ""); err != nil {
		logger.ErrorCF("workflow", "运行工作流失败", map[string]any{"workflow": wf.Name, "error": err.Error()})
	}
}

// checkAndFireAtTrigger 检查并触发 At 触发器（一次性执行）
func (s *Service) checkAndFireAtTrigger(wf *Workflow, index int, trigger Trigger) {
	fireKey := wf.Name + "|at|" + trigger.At

	logger.DebugCF("workflow", "检查 at 触发器",
		map[string]any{"workflow": wf.Name, "at": trigger.At, "tz": trigger.TZ})

	// 解析时间
	tz := trigger.TZ
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}

	atTime, err := time.ParseInLocation("2006-01-02 15:04:05", trigger.At, loc)
	if err != nil {
		// 尝试其他格式
		atTime, err = time.ParseInLocation("2006-01-02 15:04", trigger.At, loc)
	}
	if err != nil {
		logger.WarnCF("workflow", "无效的 at 时间格式",
			map[string]any{"at": trigger.At, "workflow": wf.Name, "error": err.Error()})
		return
	}

	// 检查是否到达执行时间（允许 60 秒的误差窗口）
	now := timeNow().In(loc)
	timeDiff := now.Sub(atTime)
	if timeDiff < 0 || timeDiff > 60*time.Second {
		logger.DebugCF("workflow", "at 触发器时间未到或已过期",
			map[string]any{"workflow": wf.Name, "at": trigger.At, "diff": timeDiff.Seconds()})
		return
	}

	// 去重：At 触发器只执行一次
	s.lastCronFireMu.Lock()
	_, fired := s.lastCronFire[fireKey]
	if fired {
		s.lastCronFireMu.Unlock()
		logger.DebugCF("workflow", "at 触发器已执行过",
			map[string]any{"workflow": wf.Name, "at": trigger.At})
		return
	}
	s.lastCronFire[fireKey] = timeNow()
	s.lastCronFireMu.Unlock()

	logger.InfoCF("workflow", "at 触发器触发工作流",
		map[string]any{"workflow": wf.Name, "trigger_index": index, "at": trigger.At})
	ctx := context.Background()
	if _, err := s.engine.RunWorkflow(ctx, wf, "at", "", ""); err != nil {
		logger.ErrorCF("workflow", "运行工作流失败", map[string]any{"workflow": wf.Name, "error": err.Error()})
	}
}

// checkAndFireIntervalTrigger 检查并触发 Interval 触发器（间隔执行）
func (s *Service) checkAndFireIntervalTrigger(wf *Workflow, index int, trigger Trigger) {
	fireKey := wf.Name + "|interval|" + trigger.Interval

	logger.DebugCF("workflow", "检查 interval 触发器",
		map[string]any{"workflow": wf.Name, "interval": trigger.Interval})

	// 解析间隔
	duration, err := time.ParseDuration(trigger.Interval)
	if err != nil {
		logger.WarnCF("workflow", "无效的 interval 格式",
			map[string]any{"interval": trigger.Interval, "workflow": wf.Name, "error": err.Error()})
		return
	}

	// 检查是否到达执行时间
	s.lastCronFireMu.Lock()
	lastFire, fired := s.lastCronFire[fireKey]
	s.lastCronFireMu.Unlock()

	if fired {
		// 检查是否已经过去足够的间隔时间
		if timeNow().Sub(lastFire) < duration {
			logger.DebugCF("workflow", "interval 触发器间隔未到",
				map[string]any{"workflow": wf.Name, "interval": trigger.Interval, "last_fire": lastFire})
			return
		}
	}

	// 记录本次触发
	s.lastCronFireMu.Lock()
	s.lastCronFire[fireKey] = timeNow()
	s.lastCronFireMu.Unlock()

	logger.InfoCF("workflow", "interval 触发器触发工作流",
		map[string]any{"workflow": wf.Name, "trigger_index": index, "interval": trigger.Interval})
	ctx := context.Background()
	if _, err := s.engine.RunWorkflow(ctx, wf, "interval", "", ""); err != nil {
		logger.ErrorCF("workflow", "运行工作流失败", map[string]any{"workflow": wf.Name, "error": err.Error()})
	}
}

// checkEventTriggers 评估事件触发器。
// 使用缓存机制避免每次都从磁盘读取。
// 支持事件过滤条件和事件数据映射到工作流变量。
func (s *Service) checkEventTriggers(evt runtimeevents.Event) {
	// 获取工作流（使用缓存机制）
	workflows := s.getWorkflowsForTriggerCheck()

	// 将 payload 转换为字符串用于日志
	var payloadStr string
	if evt.Payload != nil {
		switch p := evt.Payload.(type) {
		case string:
			payloadStr = p
		case []byte:
			payloadStr = string(p)
		default:
			if b, err := json.Marshal(p); err == nil {
				payloadStr = string(b)
			}
		}
	}
	logger.DebugCF("workflow", "收到事件",
		map[string]any{"kind": evt.Kind, "payload": truncateString(payloadStr, 200)})

	for _, wf := range workflows {
		if !wf.Enabled {
			continue
		}
		for _, trigger := range wf.Triggers {
			if trigger.Event == "" {
				continue
			}
			// 匹配事件类型
			if string(evt.Kind) != trigger.Event {
				continue
			}

			// 检查事件过滤条件
			if !s.matchEventFilters(evt, trigger.EventFilters) {
				logger.DebugCF("workflow", "事件过滤条件不匹配",
					map[string]any{"workflow": wf.Name, "event": trigger.Event, "filters": trigger.EventFilters})
				continue
			}

			if s.engine.IsRunning(wf.Name) {
				logger.InfoCF("workflow", "跳过事件触发，工作流正在运行中", map[string]any{"workflow": wf.Name})
				continue
			}

			// 构建事件变量映射
			eventVars := s.buildEventVars(evt, trigger.EventMapping)
			logger.DebugCF("workflow", "事件触发器匹配成功",
				map[string]any{"workflow": wf.Name, "event": trigger.Event, "vars": eventVars})

			logger.InfoCF("workflow", "事件触发器触发工作流",
				map[string]any{"event": trigger.Event, "workflow": wf.Name, "vars_count": len(eventVars)})

			// 合并事件变量到工作流变量
			mergedVars := make(map[string]string)
			maps.Copy(mergedVars, wf.Vars)
			maps.Copy(mergedVars, eventVars)

			// 创建工作流副本，包含事件变量
			wfCopy := *wf
			wfCopy.Vars = mergedVars

			ctx := context.Background()
			if _, err := s.engine.RunWorkflow(ctx, &wfCopy, "event", "", ""); err != nil {
				logger.ErrorCF("workflow", "运行工作流失败", map[string]any{"workflow": wf.Name, "error": err.Error()})
			}
		}
	}
}

// payloadToMap 将事件的 payload 转换为 map[string]interface{}
func payloadToMap(payload any) (map[string]any, bool) {
	if payload == nil {
		return nil, false
	}

	switch p := payload.(type) {
	case map[string]any:
		return p, true
	case string:
		var result map[string]any
		if err := json.Unmarshal([]byte(p), &result); err == nil {
			return result, true
		}
	case []byte:
		var result map[string]any
		if err := json.Unmarshal(p, &result); err == nil {
			return result, true
		}
	default:
		// 尝试序列化后再反序列化
		if b, err := json.Marshal(p); err == nil {
			var result map[string]any
			if err := json.Unmarshal(b, &result); err == nil {
				return result, true
			}
		}
	}
	return nil, false
}

// matchEventFilters 检查事件是否匹配过滤条件
func (s *Service) matchEventFilters(evt runtimeevents.Event, filters map[string]string) bool {
	if len(filters) == 0 {
		return true
	}

	// 解析事件 payload 为 map
	payload, ok := payloadToMap(evt.Payload)
	if !ok {
		logger.DebugCF("workflow", "无法解析事件 payload", map[string]any{})
		// 如果无法解析，只有在没有过滤条件时才匹配
		return len(filters) == 0
	}

	// 检查每个过滤条件
	for key, expectedValue := range filters {
		actualValue, exists := getNestedValue(payload, key)
		if !exists {
			logger.DebugCF("workflow", "过滤条件字段不存在",
				map[string]any{"key": key})
			return false
		}
		if fmt.Sprintf("%v", actualValue) != expectedValue {
			logger.DebugCF("workflow", "过滤条件值不匹配",
				map[string]any{"key": key, "expected": expectedValue, "actual": actualValue})
			return false
		}
	}

	return true
}

// buildEventVars 根据事件数据映射构建变量
func (s *Service) buildEventVars(evt runtimeevents.Event, mapping map[string]string) map[string]string {
	vars := make(map[string]string)

	// 解析事件 payload
	payload, ok := payloadToMap(evt.Payload)
	if !ok {
		logger.WarnCF("workflow", "无法解析事件 payload 进行变量映射", map[string]any{})
		// 即使没有 payload，也添加事件元数据
		vars["event_kind"] = string(evt.Kind)
		vars["event_time"] = time.Now().Format("2006-01-02 15:04:05")
		return vars
	}

	if len(mapping) == 0 {
		// 默认映射：将常用字段直接映射
		// 映射常用字段
		if tool, ok := payload["tool"]; ok {
			vars["event_tool"] = fmt.Sprintf("%v", tool)
		}
		if result, ok := payload["result"]; ok {
			vars["event_result"] = fmt.Sprintf("%v", result)
		}
		if errVal, ok := payload["error"]; ok {
			vars["event_error"] = fmt.Sprintf("%v", errVal)
		}
	}

	// 根据映射规则提取变量
	for varName, path := range mapping {
		value, exists := getNestedValue(payload, path)
		if exists {
			vars[varName] = fmt.Sprintf("%v", value)
		}
	}

	// 添加事件元数据
	vars["event_kind"] = string(evt.Kind)
	vars["event_time"] = time.Now().Format("2006-01-02 15:04:05")

	return vars
}

// getNestedValue 从嵌套的 map 中获取值，支持点号分隔的路径
func getNestedValue(data map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		value, exists := current[part]
		if !exists {
			return nil, false
		}

		// 如果是最后一部分，返回值
		if i == len(parts)-1 {
			return value, true
		}

		// 否则，继续深入
		next, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}

	return nil, false
}

// truncateString 截断字符串到指定长度
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// reloadWorkflowsUnsafe 从磁盘重新加载工作流定义（调用者需持有锁）。
func (s *Service) reloadWorkflowsUnsafe() error {
	workflows, err := s.store.LoadAllWorkflows()
	if err != nil {
		return err
	}

	for _, wf := range workflows {
		// 计算运行时状态字段
		s.computeWorkflowRuntimeState(wf)
	}

	s.workflows = workflows
	return nil
}

// computeWorkflowRuntimeState 计算工作流的运行时状态字段（NextRunAt, LastRunAt, LastRunStatus）。
func (s *Service) computeWorkflowRuntimeState(wf *Workflow) {
	// 计算下次运行时间（取所有定时触发器中最近的）
	var nextRun *time.Time
	now := time.Now()

	for _, trigger := range wf.Triggers {
		var triggerNext *time.Time

		switch {
		case trigger.Cron != "":
			triggerNext = s.computeNextCronRun(trigger.Cron, trigger.TZ, now)
		case trigger.At != "":
			triggerNext = s.computeNextAtRun(trigger.At, trigger.TZ, now)
		case trigger.Interval != "":
			triggerNext = s.computeNextIntervalRun(trigger.Interval, now)
		}

		if triggerNext != nil && (nextRun == nil || triggerNext.Before(*nextRun)) {
			nextRun = triggerNext
		}
	}
	wf.NextRunAt = nextRun

	// 从实例记录中获取上次运行信息
	instances, err := s.store.LoadInstances(wf.Name)
	if err != nil || len(instances) == 0 {
		return
	}

	// 找到最近的一次运行
	var lastInstance *WorkflowInstanceSummary
	for i := range instances {
		if lastInstance == nil || instances[i].StartedAt.After(lastInstance.StartedAt) {
			lastInstance = instances[i]
		}
	}

	if lastInstance != nil {
		wf.LastRunAt = &lastInstance.StartedAt
		switch lastInstance.Status {
		case "running":
			wf.LastRunStatus = "running"
		case "completed":
			wf.LastRunStatus = "success"
		case "failed", "canceled":
			wf.LastRunStatus = "failed"
		default:
			wf.LastRunStatus = lastInstance.Status
		}
	}
}

// computeNextCronRun 计算 cron 触发器的下次运行时间。
func (s *Service) computeNextCronRun(cronExpr, tz string, now time.Time) *time.Time {
	loc := time.UTC
	if tz != "" {
		loadedLoc, err := time.LoadLocation(tz)
		if err == nil {
			loc = loadedLoc
		}
	}
	now = now.In(loc)

	nextTick, err := gronx.NextTickAfter(cronExpr, now, false)
	if err != nil {
		return nil
	}
	nextTick = nextTick.In(loc)
	return &nextTick
}

// computeNextAtRun 计算 at 触发器的下次运行时间。
func (s *Service) computeNextAtRun(atStr, tz string, now time.Time) *time.Time {
	loc := time.UTC
	if tz != "" {
		loadedLoc, err := time.LoadLocation(tz)
		if err == nil {
			loc = loadedLoc
		}
	}

	// 尝试解析时间
	atTime, err := time.ParseInLocation("2006-01-02 15:04:05", atStr, loc)
	if err != nil {
		atTime, err = time.ParseInLocation("2006-01-02 15:04", atStr, loc)
	}
	if err != nil {
		return nil
	}

	// 只有未来时间才返回
	if atTime.After(now) {
		return &atTime
	}
	return nil
}

// computeNextIntervalRun 计算 interval 触发器的下次运行时间。
func (s *Service) computeNextIntervalRun(intervalStr string, now time.Time) *time.Time {
	duration, err := time.ParseDuration(intervalStr)
	if err != nil {
		return nil
	}

	nextRun := now.Add(duration)
	return &nextRun
}

// clearTriggerCache 清除触发器检查缓存。
// 在工作流被修改后调用，确保下次触发器检查使用最新数据。
func (s *Service) clearTriggerCache() {
	s.triggerCacheMu.Lock()
	defer s.triggerCacheMu.Unlock()
	s.triggerCacheModTime = 0
	s.triggerCacheData = nil
}

// handleWorkflowCompleteEvent 处理工作流完成事件，更新 LastRunAt 和 LastRunStatus。
// 注意：先从磁盘读取最新工作流定义，避免用内存中的旧数据覆盖用户修改。
func (s *Service) handleWorkflowCompleteEvent(evt runtimeevents.Event) {
	workflowName := evt.Source.Name
	if workflowName == "" {
		return
	}

	// 从事件中获取状态
	status := "success"
	if payload, ok := evt.Payload.(map[string]any); ok {
		if s, ok := payload["status"].(string); ok {
			switch s {
			case "completed":
				status = "success"
			case "failed", "canceled":
				status = "failed"
			default:
				status = s
			}
		}
	}

	// 先从磁盘读取最新工作流定义，避免覆盖用户修改
	wf, err := s.store.LoadSingleWorkflow(workflowName)
	if err != nil {
		logger.ErrorCF("workflow", "加载工作流失败，无法更新状态",
			map[string]any{"workflow": workflowName, "error": err.Error()})
		return
	}

	// 更新工作流状态
	now := time.Now()
	wf.LastRunAt = &now
	wf.LastRunStatus = status

	s.mu.Lock()
	defer s.mu.Unlock()

	// 保存到磁盘
	if err := s.store.SaveWorkflow(wf); err != nil {
		logger.ErrorCF("workflow", "保存工作流状态失败",
			map[string]any{"workflow": workflowName, "error": err.Error()})
		return
	}

	// 同时更新内存中的缓存
	s.workflows[workflowName] = wf

	logger.DebugCF("workflow", "更新工作流最后运行状态",
		map[string]any{"workflow": workflowName, "status": status, "time": now})
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
