package commands

import (
	"context"

	"github.com/sipeed/picoclaw/pkg/config"
)

type MCPServerInfo struct {
	Name      string
	Enabled   bool
	Deferred  bool
	Connected bool
	ToolCount int
}

type MCPToolParameterInfo struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

type MCPToolInfo struct {
	Name        string
	Description string
	Parameters  []MCPToolParameterInfo
}

// ContextStats describes current session context window usage.
type ContextStats struct {
	UsedTokens        int
	TotalTokens       int // model context window
	HistoryTokens     int // history-only tokens (what maybeSummarize checks)
	CompressAtTokens  int // hard budget compression threshold
	SummarizeAtTokens int // soft summarization trigger
	UsedPercent       int // 0-100
	MessageCount      int
}

// StopResult describes the outcome of a stop request for the current session.
type StopResult struct {
	Stopped  bool
	TaskName string
}

// WorkflowInfo 是用于命令输出的工作流摘要视图。
type WorkflowInfo struct {
	Name        string
	Description string
	Enabled     bool
	TriggerType string
	StepCount   int
	Vars        map[string]string
	Triggers    []string // 格式化的触发器详情列表
}

// WorkflowInstanceInfo 是用于命令输出的工作流实例摘要视图。
type WorkflowInstanceInfo struct {
	ID           string
	WorkflowName string
	Status       string
	TriggerType  string
	StartedAt    string
	FinishedAt   string
	Error        string
}

// Runtime provides runtime dependencies to command handlers. It is constructed
// per-request by the agent loop so that per-request state (like session scope)
// can coexist with long-lived callbacks (like GetModelInfo).
type Runtime struct {
	Config             *config.Config
	GetModelInfo       func() (name, provider string)
	AskSideQuestion    func(ctx context.Context, question string) (string, error)
	ListAgentIDs       func() []string
	ListDefinitions    func() []Definition
	ListSkillNames     func() []string
	ListMCPServers     func(ctx context.Context) []MCPServerInfo
	ListMCPTools       func(ctx context.Context, serverName string) ([]MCPToolInfo, error)
	GetEnabledChannels func() []string
	GetActiveTurn      func() any // Returning any to avoid circular dependency with agent package
	GetContextStats    func() *ContextStats
	SwitchModel        func(value string) (oldModel string, err error)
	SwitchChannel      func(value string) error
	ClearHistory       func() error
	ReloadConfig       func() error
	StopActiveTurn     func() (StopResult, error)

	// 工作流命令
	WorkflowList      func() []WorkflowInfo
	WorkflowRun       func(ctx context.Context, name, channel, chatID string) (string, error)
	WorkflowShow      func(name string) (*WorkflowInfo, []string, error) // info, stepIDs, 错误
	WorkflowBind      func(name, channel, chatID string) error
	WorkflowUnbind    func(name, channel, chatID string) error
	WorkflowChannels  func(name string) ([]NotifyTarget, error)
	WorkflowEnable    func(name string, enabled bool) error
	WorkflowInstances func(name string) ([]WorkflowInstanceInfo, error)
	WorkflowStop      func(instanceID string) error
	WorkflowCronList  func() []CronTaskInfo
}

// CronTaskInfo 表示一个待执行的调度任务。
type CronTaskInfo struct {
	WorkflowName string `json:"workflow_name"`
	TriggerType  string `json:"trigger_type"` // cron, at, interval
	Schedule     string `json:"schedule"`     // cron 表达式、at 时间或 interval 值
	Timezone     string `json:"timezone"`
	NextRun      string `json:"next_run"`
}

// NotifyTarget 表示通知目标（频道 + 聊天 ID）。
type NotifyTarget struct {
	Channel string `json:"channel"` // 频道名称，如 "dingtalk"、"telegram"
	ChatID  string `json:"chat_id"` // 聊天 ID
}
