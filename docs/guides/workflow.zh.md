# 工作流引擎（Workflow Engine）

## 概述

工作流引擎是 picoclaw 的声明式多步骤任务编排系统。它允许你定义包含多个步骤的工作流，每个步骤可以执行 LLM 提示词、调用工具或并行执行子步骤，并通过条件表达式控制流程走向，保障任务的可靠执行。

## 设计目标

1. **声明式定义**：工作流通过 YAML 描述"做什么"，而非"怎么做"，降低编排复杂度
2. **可靠执行**：步骤支持重试、超时、错误处理，失败后自动执行 on_error 处理步骤
3. **条件控制**：通过 `when` 条件表达式实现分支逻辑，无需硬编码流程
4. **数据传递**：步骤间通过 `output_key` + 模板语法 `{{.step_id.key}}` 传递结果，通过 `{{.vars.key}}` 引用变量，解耦步骤依赖
5. **多触发方式**：支持 Cron 定时、事件驱动、手动触发三种方式
6. **与现有系统集成**：复用 AgentLoop 的 SubTurn 机制执行提示词，复用 ToolRegistry 调用工具，复用事件总线监听事件

## 系统架构

### 整体分层

```
┌─────────────────────────────────────────────────┐
│                   Web UI                         │  前端页面（列表/表单/详情）
├─────────────────────────────────────────────────┤
│                 REST API                         │  HTTP 接口（CRUD/执行/停止）
├─────────────────────────────────────────────────┤
│               Service 层                         │  生命周期管理、触发器调度
├──────────┬──────────┬───────────────────────────┤
│  Engine  │ Persist  │  触发器（Cron/Event/手动）  │  核心引擎 + 持久化
├──────────┴──────────┴───────────────────────────┤
│              StepExecutor                        │  步骤执行器（重试/超时）
├────────────────────┬────────────────────────────┤
│   AgentPromptFunc  │      ToolCallFunc           │  回调：LLM 提示/工具调用
├────────────────────┴────────────────────────────┤
│            AgentLoop / ToolRegistry              │  picoclaw 核心能力
└─────────────────────────────────────────────────┘
```

### 模块职责

| 模块 | 文件 | 职责 |
|------|------|------|
| 数据模型 | `model.go` | 定义 Workflow、Step、Trigger、WorkflowInstance、StepState 等核心结构体，包含 Validate() 校验逻辑 |
| 持久化 | `persist.go` | PersistStore 管理工作流定义（YAML）和实例状态（JSON）的读写，使用原子写入确保数据安全 |
| 条件求值 | `conditions.go` | EvaluateCondition 解析 when 条件，ResolveStepTemplates 替换 `{{.step_id.key}}` 模板引用 |
| 步骤执行器 | `executor.go` | StepExecutor 封装 AgentPromptFunc 和 ToolCallFunc 回调，ExecuteWithRetry 支持可配置的重试和延迟 |
| 核心引擎 | `engine.go` | Engine 管理运行中的实例、cancel 函数、步骤编排（顺序执行/条件判断/失败策略） |
| 生命周期服务 | `service.go` | Service 整合 Engine + PersistStore，提供 CRUD API、触发器管理（cron 检查循环 + 事件订阅） |
| YAML 工具 | `yaml.go` | parseYAMLWorkflow / renderYAMLWorkflow 序列化工具 |

### 集成方式

工作流引擎通过 `setupWorkflowService()` 集成到 Gateway，与 `setupCronTool()` 模式一致：

```
Gateway 启动
  └── setupAndStartServices()
        └── setupWorkflowService()
              ├── 创建 PersistStore（工作空间目录）
              ├── 创建 StepExecutor
              │     ├── AgentPromptFunc → AgentLoop.ProcessDirectWithChannel()
              │     └── ToolCallFunc → AgentLoop.GetRegistry() → Tool.Execute()
              ├── 创建 Engine
              ├── 创建 Service（注入 EventBus、MessageBus）
              ├── 注册 WorkflowTool 到 Agent（如果启用）
              └── service.Start() → 加载工作流 → 订阅事件 → 启动 cron 循环
```

关键点：
- **AgentPromptFunc**：通过 `AgentLoop.ProcessDirectWithChannel()` 调用 LLM，复用完整的 Agent 处理链
- **ToolCallFunc**：从 `ToolRegistry` 获取工具并执行，复用所有已注册工具（包括 MCP 工具）
- **工具注册**：`cfg.Tools.IsToolEnabled("workflow")` 控制是否注册（默认启用）
- **命令注入**：`agentLoop.SetWorkflowService(service)` 将服务注入 AgentLoop，支持 `/workflow` 斜杠命令
- **内部 API**：Engine 在 Gateway 进程中注册内部 HTTP 端点（`/internal/workflow/*`），Web 后端通过反向代理访问运行时操作，CRUD 操作直接读写文件系统

## 执行流程

### 触发流程

```
                    ┌──────────────┐
                    │  触发请求到达  │
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        Cron 到期      事件匹配      手动调用
     (30s 轮询)    (EventBus 订阅)  (API/Tool)
              │            │            │
              └────────────┼────────────┘
                           ▼
                  检查工作流是否启用
                  检查是否已有运行中实例（防重复）
                           │
                           ▼
                  engine.RunWorkflow(ctx, wf, triggerType)
```

### 步骤执行流程

```
RunWorkflow()
  │
  ├── 创建 WorkflowInstance（状态: pending）
  ├── 保存实例状态到磁盘
  ├── 注册 cancel 函数
  │
  └── 异步执行 executeWorkflow()
        │
        ├── 更新实例状态: running
        │
        ├── 遍历 steps（顺序执行）
        │     │
        │     ├── 1. 条件检查：EvaluateCondition(step.When, 上一步结果)
        │     │     ├── 条件不满足 → 跳过该步骤（skipped）
        │     │     └── 条件满足 → 继续执行
        │     │
  │     ├── 2. 模板替换：ResolveStepTemplates(prompt/args, 已完成步骤输出)
  │     │     └── 将 {{.step_id.output_key}} 替换为实际值
  │     │
  │     ├── 3. 延迟等待：如果设置了 step.delay，等待指定时长
  │     │     └── 等待期间被取消 → 步骤状态: cancelled
  │     │
  │     ├── 4. 执行步骤：ExecuteWithRetry()
        │     │     ├── agent_prompt → AgentPromptFunc(ctx, prompt)
        │     │     ├── tool_call   → ToolCallFunc(ctx, tool, args)
        │     │     ├── parallel    → goroutine 并行执行子步骤（子步骤失败遵守 failure_strategy）
        │     │     └── if          → 评估 when 条件，执行 if_true 或 if_false 分支（分支步骤失败遵守 failure_strategy）
        │     │
        │     ├── 5. 处理执行结果
        │     │     ├── 成功 → 记录输出到 output_key，继续下一步
        │     │     └── 失败 →
        │     │           ├── failure_strategy=stop → 中止，查找 on_error 处理步骤
        │     │           └── failure_strategy=continue → 记录失败，继续下一步
        │     │
        │     └── 6. 保存步骤状态和实例状态到磁盘
        │
        ├── 所有步骤完成
        │     ├── 无失败 → 实例状态: completed
        │     └── 有失败 → 实例状态: failed
        │
        └── 清理 cancel 函数
```

### 重试机制

```
ExecuteWithRetry()
  │
  ├── 第 1 次执行
  │     ├── 成功 → 返回结果
  │     └── 失败 →
  │           ├── retry == 0 → 返回错误
  │           └── retry > 0 → 等待 retry_delay 秒
  │
  ├── 第 2 次执行
  │     ├── 成功 → 返回结果
  │     └── 失败 → 继续重试...
  │
  └── 达到最大重试次数 → 返回最后一次的错误
```

### 错误处理流程

```
步骤执行失败
  │
  ├── failure_strategy = "stop"
  │     ├── 中止后续步骤执行
  │     ├── 查找是否有 when="on_error" 的步骤
  │     │     ├── 有 → 执行错误处理步骤
  │     │     │     ├── 处理步骤成功 → 实例状态: completed
  │     │     │     └── 处理步骤失败 → 实例状态: failed
  │     │     └── 无 → 实例标记为 failed
  │     └── （不再继续主循环中的后续步骤）
  │
  └── failure_strategy = "continue"
        ├── 记录该步骤为 failed
        ├── 继续执行后续步骤
        └── 所有步骤完成后，实例状态: failed（但仍执行了后续步骤）
```

### 取消流程

```
StopInstance(instanceID)
  │
  ├── 查找 cancel 函数
  ├── 调用 context.Cancel()
  ├── 执行中的步骤收到 ctx.Done() 信号
  │     ├── AgentPromptFunc 返回 context.Canceled
  │     └── ToolCallFunc 返回 context.Canceled
  ├── 步骤状态更新为 cancelled
  └── 实例状态更新为 cancelled
```

## 核心概念

### 工作流（Workflow）

工作流是由一组有序步骤组成的可执行单元。每个工作流包含：

- **名称**：唯一标识符，用于引用和管理，仅允许英文字母、数字、连字符和下划线（`a-zA-Z0-9_-`）
- **描述**：工作流用途说明
- **触发器**：定义工作流何时自动执行（手动/Cron/事件）
- **变量**：工作流级变量，避免在步骤中重复写相同的值（如路径、URL 等）
- **步骤**：按顺序执行的操作序列
- **配置**：失败策略（stop/continue）等全局选项

> **名称规则说明**：工作流名称限制为 `a-zA-Z0-9_-`，因为名称直接用作 YAML 文件名，非 ASCII 或特殊字符会导致文件系统问题。`enabled`（启用状态）是运行时属性，不会序列化到 YAML 定义中，而是通过 `.disabled` 标记文件管理（详见文件存储）。

### 变量（Vars）

变量用于在工作流中定义可复用的值，避免在多个步骤中重复相同的字符串（如目录路径、站点 URL 等）。变量通过 `vars` 字段定义，在步骤中通过 `{{.vars.key}}` 模板语法引用。

```yaml
vars:
  project_dir: "/root/.picoclaw/workspace/news"
  site_url: "https://example.com"
```

在步骤中引用变量：

```yaml
steps:
  - id: search
    action: agent_prompt
    prompt: "搜索 {{.vars.site_url}} 的最新信息，保存到 {{.vars.project_dir}}/data"
```

**工作原理**：
- 工作流启动时，引擎将 `vars` 注入到步骤输出的 `vars` 键中
- 模板解析时，`{{.vars.key}}` 会被替换为变量定义中的值
- 变量值支持模板引用（如引用前序步骤的输出），但通常用于静态值
- 变量可在 `prompt`、`args`、`when` 字段中使用

### 步骤（Step）

步骤是工作流的基本执行单元，支持四种动作类型：

| 动作类型 | 说明 | 关键参数 |
|---------|------|---------|
| `agent_prompt` | 调用 LLM 执行提示词 | `prompt`（提示词模板） |
| `tool_call` | 调用已注册的工具 | `tool`（工具名）、`args`（参数） |
| `parallel` | 并行执行多个子步骤 | `parallel`（子步骤列表） |
| `if` | 条件判断，执行 true 或 false 分支 | `when`（条件表达式）、`if_true`/`if_false`（分支步骤） |

每个步骤支持以下配置：

- **id**：步骤唯一标识，仅允许英文字母、数字和下划线（`a-zA-Z0-9_`），用于模板引用 `{{.step_id.key}}` 和条件判断
- **name**：步骤显示名称（可选），支持中文等任意字符，用于 UI 展示和通知，不填写时显示 id
- **when**：条件表达式，满足时才执行该步骤
- **delay**：步骤执行前的等待时间，如 `"5s"`、`"1m"`，等待期间支持取消
- **retry**：重试配置，结构化格式（默认不重试）
  - `max_attempts`：最大重试次数
  - `delay`：重试间隔，如 `"10s"`
- **timeout**：超时时间（秒）
- **output_key**：输出数据的键名，供后续步骤引用（`parallel` 步骤不适用，子步骤各自有自己的 output_key）

> **ID 规则说明**：步骤 ID 之所以限制为 `a-zA-Z0-9_`，是因为模板语法 `{{.step_id.key}}` 使用 `.` 作为分隔符，ID 中包含 `.` 或其他特殊字符会导致解析错误，非 ASCII 字符也可能引发问题。如需在 UI 中显示中文名称，请使用 `name` 字段。

### 触发器（Trigger）

触发器决定工作流何时自动执行：

- **manual**：仅手动触发（默认）
- **cron**：按 Cron 表达式定时触发，如 `0 9 * * *`（每天 9 点）
- **event**：监听事件总线上的特定事件类型触发

Cron 触发器的工作方式：
- Service 启动后，以 30 秒为周期检查所有已启用工作流的 cron 表达式
- 使用 `gronx` 库判断 cron 表达式是否到期
- 同一工作流同时只允许一个实例运行（防重复触发）

事件触发器的工作方式：
- 通过 `EventBus.Channel().SubscribeChan()` 订阅事件流
- 匹配 `Event.Kind` 与 `Trigger.Event` 字段
- 同样遵循单实例运行约束

### 条件表达式

步骤的 `when` 字段支持以下条件：

| 表达式 | 含义 | 示例 |
|--------|------|------|
| `on_error` | 上一步失败时执行 | 错误告警、重试通知 |
| `on_success` | 上一步成功时执行（可省略，默认行为） | 正常后续处理 |
| `{{.step_id.key}} == value` | 模板比较，检查指定步骤的输出值 | 根据结果分支 |

求值逻辑（`conditions.go`）：
1. 空条件 → 视为 `on_success`，上一步成功则通过
2. `on_error` → 上一步状态为 failed 时通过
3. `on_success` → 上一步状态为 completed 时通过
4. 包含 `==` → 拆分左右操作数，对模板引用求值后进行字符串比较

### 数据传递

步骤之间通过 `output_key` 和模板语法传递数据：

```
步骤 A（fetch_weather）                  步骤 B（summarize）
  action: agent_prompt                    action: agent_prompt
  prompt: "查询天气"                       prompt: "天气: {{.fetch_weather.weather}}"
  output_key: weather  ──────────────►   模板替换为实际值
```

模板解析逻辑（`conditions.go`）：
1. 正则匹配 `{{.step_id.key}}` 或 `{{.vars.key}}` 模式
2. 在已完成步骤的输出数据中查找对应值（`vars` 作为特殊步骤 ID，存储工作流变量）
3. 替换模板为实际输出内容
4. 同时应用于 `prompt`、`args`、`when` 字段

### 实例状态机

```
                    ┌─────────┐
                    │ pending │
                    └────┬────┘
                         │ 开始执行
                         ▼
                    ┌─────────┐
              ┌─────│ running │─────┐
              │     └────┬────┘     │
              │          │          │
         用户取消    执行完成    步骤失败
              │          │          │
              ▼          ▼          ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │cancelled │ │completed │ │  failed   │
        └──────────┘ └──────────┘ └──────────┘
                                       │
                                  条件不满足
                                       │
                                       ▼
                                 ┌──────────┐
                                 │ skipped  │
                                 └──────────┘
```

每个步骤（包括 parallel 子步骤和 if 分支子步骤）也有独立的 `StepState` 追踪：
`pending` → `running` → `completed` / `failed` / `skipped` / `cancelled`。

`StepState` 包含以下字段：
- **Name**：步骤显示名称
- **Status**：当前状态
- **StartedAt** / **FinishedAt**：开始和结束时间
- **Attempts**：执行尝试次数（首次执行为 1，重试后递增）
- **Error**：失败时的错误信息

未执行的 if 分支子步骤自动标记为 `skipped`。

> **failureStrategy 在嵌套步骤中的行为**：`failure_strategy` 不仅作用于顶级步骤，也作用于 if 分支步骤和 parallel 子步骤。当 `failure_strategy=continue` 时，if 分支或 parallel 子步骤的失败不会中断工作流执行，而是记录警告日志并继续。

### 实时事件流（SSE）

工作流引擎将状态变更事件发布到事件总线（`pkg/events`），可通过 SSE 实时推送到 UI：

- `workflow.instance.start` — 实例开始执行
- `workflow.instance.complete` — 实例执行结束（成功/失败/取消）
- `workflow.step.start` — 步骤开始执行
- `workflow.step.complete` — 步骤执行结束（成功/失败）

SSE 端点（`GET /api/workflows/{name}/instances/{id}/stream`）以 Server-Sent Events 格式推送这些事件。
前端对运行中的实例自动连接 SSE 流，无需轮询即可获得实时步骤进度和日志更新。

工作流定义存放在 `workspace/workflows/` 目录下，每个工作流一个 YAML 文件：

```yaml
name: morning-briefing
description: 每日早间简报工作流

triggers:
  - cron: "0 8 * * *"

vars:
  city: "北京"
  output_dir: "/tmp/briefing"

config:
  failure_strategy: stop  # stop 或 continue
  notify_channel: telegram  # 可选，默认通知频道
  notify_chat_id: "-100xxx" # 可选，默认通知聊天 ID

steps:
  - id: fetch_weather
    name: 获取天气
    action: agent_prompt
    prompt: "查询今天{{.vars.city}}的天气预报"
    output_key: weather

  - id: fetch_news
    name: 获取新闻
    action: agent_prompt
    prompt: "获取今日科技新闻摘要"
    output_key: news

  - id: summarize
    name: 生成简报
    action: agent_prompt
    prompt: "根据天气信息 {{.fetch_weather.weather}} 和新闻 {{.fetch_news.news}}，生成今日简报，保存到 {{.vars.output_dir}}"
    when: "on_success"

  - id: notify_error
    action: tool_call
    tool: send_message
    args:
      message: "早间简报执行失败"
    when: "on_error"
```

### 完整步骤字段说明

```yaml
steps:
  - id: step_id           # 必填，步骤唯一标识，仅允许 a-zA-Z0-9_，用于条件引用和数据传递
    name: 步骤名称        # 可选，步骤显示名称，支持中文等任意字符，不填时显示 id
    action: agent_prompt   # 必填，动作类型：agent_prompt / tool_call / parallel / if
    prompt: "..."          # agent_prompt 必填，提示词模板，支持 {{.step_id.key}} 引用
    tool: tool_name        # tool_call 必填，已注册的工具名称
    args:                  # tool_call 可选，工具参数，值支持模板引用
      key: value
    parallel:              # parallel 必填，子步骤列表
      - id: sub1
        action: agent_prompt
        prompt: "..."
    when: "on_error"       # if 必填/其他可选，条件表达式
    delay: 5s              # 可选，步骤执行前的等待时间（如 5s、1m30s）
    if_true:               # if 可选，条件为 true 时执行的步骤
      - id: handle_success
        action: agent_prompt
        prompt: "..."
    if_false:              # if 可选，条件为 false 时执行的步骤
      - id: handle_failure
        action: agent_prompt
        prompt: "..."
    output_key: result     # 可选，输出键名
    retry:                 # 可选，重试配置
      max_attempts: 3
      delay: 10s
    timeout: 60s           # 可选，超时时间
```

### if 条件步骤

`if` 步骤根据 `when` 条件表达式选择执行 `if_true` 或 `if_false` 分支：

```yaml
steps:
  - id: health_check
    action: tool_call
    tool: exec
    args:
      command: "curl -sf http://localhost:8080/health"
    output_key: health

  - id: check_result
    action: if
    when: on_error
    if_true:
      - id: notify_admin
        action: agent_prompt
        prompt: "服务器健康检查失败，请通知管理员"
    if_false:
      - id: log_success
        action: agent_prompt
        prompt: "服务器运行正常"
```

`if` 步骤的 `when` 条件支持：
- `on_error`：上一步失败时走 `if_true`，成功时走 `if_false`
- `on_success`：上一步成功时走 `if_true`，失败时走 `if_false`
- `{{.step_id.key}} == value`：指定步骤输出等于某值时走 `if_true`

> **if 步骤的事件与通知**：`if` 步骤也会触发 `workflow.step.start` 和 `workflow.step.complete` SSE 事件，以及 `onStepStart`/`onStepComplete` 通知回调（与 agent_prompt/tool_call 步骤一致）。

### 可视化编辑器

工作流编辑页面提供基于 Sequential Workflow Designer 的可视化编辑器，支持：
- 从工具箱拖拽步骤到画布（每种步骤类型显示说明文字）
- 点击步骤在右侧面板编辑属性（ID 字段仅允许输入字母、数字和下划线，名称字段支持任意字符；重试配置仅对 agent_prompt 和 tool_call 步骤显示）
- 步骤类型拖出后固定（Agent 提示 / 工具调用 / 并行 / If 条件）
- 从工具箱拖拽步骤时自动按类型分配 ID（如 `prompt_1`、`tool_1`、`prompt_2`、`if_1`）
- 保存前前端校验，支持 i18n 错误提示（递归校验子步骤、嵌套 ID 唯一性检查）
- 编辑变量时 key 重复警告
- 并行步骤以分支形式并排显示，支持动态增删分支
- if 步骤显示为菱形，带 true/false 分支线
- 自动保存为 YAML 定义
- 支持明暗主题动态切换

#### 工具箱步骤

| 步骤 | 说明 |
|------|------|
| Agent 提示 | 向 LLM Agent 发送提示词并获取回复 |
| 工具调用 | 按名称调用已注册的工具并传入参数 |
| 并行 | 并行执行多个子步骤（容器步骤） |
| If 条件 | 条件判断：根据条件执行 true 或 false 分支 |

## REST API

| 方法 | 路径 | 说明 | 依赖 |
|------|------|------|------|
| GET | `/api/workflows` | 列出所有工作流 | 仅文件系统 |
| POST | `/api/workflows` | 创建工作流（默认禁用，重名返回 409） | 仅文件系统 |
| GET | `/api/workflows/{name}` | 获取工作流详情 | 仅文件系统 |
| PUT | `/api/workflows/{name}` | 更新工作流 | 仅文件系统 |
| DELETE | `/api/workflows/{name}` | 删除工作流 | 仅文件系统 |
| POST | `/api/workflows/{name}/run` | 手动触发执行（回退到已绑定频道发送通知） | 需 Gateway 运行 |
| POST | `/api/workflows/{name}/stop` | 停止所有运行中实例 | 需 Gateway 运行 |
| POST | `/api/workflows/{name}/toggle` | 启用/禁用 | 仅文件系统 |
| GET | `/api/workflows/{name}/instances` | 查询执行历史（按时间倒序） | 需 Gateway 运行 |
| GET | `/api/workflows/{name}/instances/{id}` | 查询实例详情 | 需 Gateway 运行 |
| GET | `/api/workflows/{name}/instances/{id}/stream` | SSE 实时推送实例状态变更 | 需 Gateway 运行 |
| DELETE | `/api/workflows/{name}/instances/{id}` | 删除执行记录 | 需 Gateway 运行 |

> CRUD 操作通过临时创建 PersistStore 读写文件，不依赖 Gateway 进程。
> 执行/停止/实例查询/删除操作通过反向代理转发到 Gateway 进程的内部 API（`/internal/workflow/*`），Gateway 不可用时部分操作可降级为文件系统直读。

### 创建工作流示例

> 通过 REST API 创建的工作流默认为**禁用状态**，确认定义无误后使用 toggle 接口启用。
> Web UI 中运行/停止/禁用操作均需确认后才会执行。

```bash
curl -X POST http://localhost:3000/api/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "name": "server-health-check",
    "description": "服务器健康检查",
    "triggers": [{"cron": "*/30 * * * *"}],
    "steps": [
      {
        "id": "check",
        "action": "agent_prompt",
        "prompt": "检查服务器CPU、内存和磁盘使用情况",
        "output_key": "health"
      },
      {
        "id": "alert",
        "action": "tool_call",
        "tool": "send_message",
        "args": {"message": "服务器异常: {{.check.health}}"},
        "when": "on_error"
      }
    ]
  }'
```

### 触发执行示例

```bash
curl -X POST http://localhost:3000/api/workflows/server-health-check/run
```

## LLM 工具接口

Agent 可通过 `workflow` 工具管理工作流：

| 动作 | 说明 | 必选参数 |
|------|------|---------|
| `list` | 列出所有工作流 | - |
| `show` | 查看工作流详情 | `name` |
| `run` | 触发执行（自动传递当前频道用于通知） | `name` |
| `stop` | 停止实例 | `instance_id` |
| `create` | 创建工作流 | `name`, `steps_yaml` |
| `delete` | 删除工作流 | `name` |
| `bind` | 绑定当前频道用于完成通知 | `name` |
| `unbind` | 移除频道绑定 | `name` |
| `enable` | 启用工作流 | `name` |
| `disable` | 禁用工作流 | `name` |
| `instances` | 查看执行历史 | `name` |

`bind` 和 `run` 动作会自动从对话上下文中捕获 channel/chatID（与 Cron 工具一致）。绑定后，工作流执行完成（成功/失败/取消）会自动通知绑定的频道。

### 频道绑定示例（对话中）

```
用户：把 morning-briefing 工作流绑定到这个频道

Agent：→ workflow action=bind name=morning-briefing
       "Workflow 'morning-briefing' bound to channel telegram (chat_id: -100xxx)。
        完成通知将发送到这里。"
```

## 斜杠命令接口

工作流也可以通过 `/workflow` 斜杠命令在任何聊天频道中管理：

| 命令 | 说明 |
|------|------|
| `/workflow list` | 列出所有工作流 |
| `/workflow run <name>` | 触发执行（自动传递当前频道用于通知） |
| `/workflow show <name>` | 查看工作流详情 |
| `/workflow bind <name>` | 绑定当前频道用于通知 |
| `/workflow unbind <name>` | 移除频道绑定 |
| `/workflow enable <name>` | 启用工作流 |
| `/workflow disable <name>` | 禁用工作流 |
| `/workflow instances <name>` | 查看执行历史 |
| `/workflow stop <instance-id>` | 停止运行中实例 |

使用示例：

```
/workflow bind morning-briefing      → 绑定通知频道
/workflow run morning-briefing       → 触发执行并绑定当前频道
/workflow instances morning-briefing → 查看执行历史
```

## 文件存储

| 路径 | 说明 |
|------|------|
| `workspace/workflows/{name}.yml` | 工作流定义文件（YAML 格式，所有平台区分大小写） |
| `workspace/workflows/{name}.disabled` | 禁用标记（文件存在即表示禁用） |
| `workspace/workflows/.state/{name}_{instanceID}.json` | 实例状态文件（JSON 格式，原子写入） |

持久化机制（`persist.go`）：
- 定义文件使用 YAML 格式，便于人工阅读和编辑
- 实例状态文件存储在 `.state/` 子目录中，通过 `fileutil.WriteFileAtomic` 原子写入，防止写入中断导致数据损坏
- `enabled` 字段是运行时状态（`yaml:"-"` 标签），不会序列化到 YAML 文件中；禁用功能通过创建/删除 `.disabled` 后缀文件来持久化状态
- 文件名保留大小写；`MyWorkflow` 和 `myworkflow` 视为不同工作流
- 文件名使用 `sanitizeName()` 处理，确保安全

## 执行结果与通知

### 当前查询方式

工作流执行完成后，可通过以下方式查看结果：

1. **REST API**：`GET /api/workflows/{name}/instances` 查询执行历史
2. **REST API**：`GET /api/workflows/{name}/instances/{id}` 查看实例详情和步骤状态
3. **斜杠命令**：`/workflow instances <name>` 在聊天中查看执行历史
4. **LLM 工具**：`workflow action=instances name=xxx` 通过 Agent 查看执行历史
5. **Web UI**：工作流卡片历史按钮 — 查看执行历史和日志

### 频道通知

工作流支持四阶段自动通知频道：

1. **开始通知**：工作流开始执行时，发送 `🚀 工作流 'xxx' 开始执行` 通知
2. **步骤开始通知**：每个步骤开始执行时，发送 `▶️ 步骤 'xxx' 开始执行（Agent 提示）` 通知
3. **步骤输出**：每个 `agent_prompt` 步骤完成后，将 AI 响应内容实时推送到频道
4. **完成通知**：工作流执行结束后，发送摘要通知（状态、耗时、错误信息）

通知通过 Engine 的四个回调实现（`onStart`、`onStepStart`、`onStepComplete`、`onComplete`），在 Gateway 的 `setupWorkflowService()` 中注册，通过 `msgBus.PublishOutbound` 发送到绑定的频道。

**通知目标优先级**：

引擎按以下优先级确定每次执行的通知频道：

1. **运行时频道** — 从聊天上下文触发时（`/workflow run`、`workflow action=run`），当前频道/chatID 传递给实例，用于发送通知
2. **持久绑定** — 无聊天上下文触发时（Web UI、cron、事件），引擎回退到通过 `/workflow bind` 或 `workflow action=bind` 设置的频道
3. **YAML 配置回退** — 如果以上均不可用，引擎回退到工作流配置中的 `notify_channel`/`notify_chat_id`

> **注意**：`/workflow bind` 和 `workflow action=bind` 将频道持久化到工作流定义中，后续的 cron/事件触发也会发送通知到该频道。`/workflow run` 和 `workflow action=run` 只将频道传递给当次执行，不会更新持久绑定。

频道绑定与 Cron 工具采用相同模式：`ToolChannel(ctx)`/`ToolChatID(ctx)` 从执行上下文提取频道信息。工作流开始、步骤开始、步骤完成和结束时，分别通过 `onStart`、`onStepStart`、`onStepComplete`、`onComplete` 回调经 `msgBus.PublishOutbound` 发送通知到绑定的频道。

> 每次工作流执行使用独立的会话（`agent:workflow-{uuid}`），不会共享上下文。频道信息从工作流实例的 `Channel`/`ChatID` 继承到步骤执行器，而非固定值。

**YAML 配置回退**：

```yaml
config:
  notify_channel: telegram   # 默认通知频道
  notify_chat_id: "-100xxx"  # 默认通知聊天 ID
```

当工作流在没有频道上下文的情况下触发（如 cron 触发、事件触发或 Web UI），引擎会回退使用工作流配置中的 `notify_channel`/`notify_chat_id`。

### 通过步骤实现通知

除了自动频道通知外，还可以在工作流内组合通知步骤：

```yaml
# 成功后推送结果到频道
steps:
  - id: do_work
    action: agent_prompt
    prompt: "执行任务"
    output_key: result

  - id: notify_success
    action: agent_prompt
    prompt: "请将以下结果发送给用户：{{.do_work.result}}"
    when: "on_success"

  - id: notify_error
    action: agent_prompt
    prompt: "任务执行失败，请通知管理员"
    when: "on_error"
```

> **注意**：`agent_prompt` 步骤通过 AgentLoop 执行，如果 Agent 配置了默认频道，结果会自动发送到该频道。

### 执行日志

每个工作流实例记录结构化的执行日志，包含时间戳、步骤 ID 和日志级别（info/warn/error）。日志存储在实例中，可通过以下方式查看：

1. **REST API**：实例详情接口返回日志数据
2. **UI**：工作流详情页的执行日志卡片；工作流列表页的历史记录弹窗
3. **频道通知**：绑定频道收到完成通知时，通知消息包含执行日志

### 当前限制

| 功能 | 状态 | 说明 |
|------|------|------|
| 执行历史查询 | ✅ 已支持 | 通过 REST API、斜杠命令或 LLM 工具 |
| 频道推送通知 | ✅ 已支持 | 四阶段通知：开始/步骤开始/步骤输出/完成 |
| 执行日志 | ✅ 已支持 | 存储在实例中，通过 API 和 UI 查看 |
| 执行日志（实时） | ✅ 已支持 | SSE 流式推送 `/api/workflows/{name}/instances/{id}/stream`，UI 自动更新 |
| 子步骤状态追踪 | ✅ 已支持 | parallel/if 子步骤拥有独立的 StepState |
| UI 执行历史 | ✅ 已支持 | 工作流卡片历史按钮，详情页执行日志卡片，支持删除记录 |
| 实例删除 | ✅ 已支持 | DELETE API + UI 删除按钮 |

## 与 CronService 的对比

| 特性 | CronService | Workflow Engine |
|------|------------|-----------------|
| 执行方式 | 单次提示词 | 多步骤编排 |
| 条件控制 | 无 | when 条件、模板比较 |
| 数据传递 | 无 | output_key + 模板引用 |
| 错误处理 | 无 | on_error 处理、重试 |
| 触发方式 | Cron | Cron + 事件 + 手动 |
| 并行执行 | 不支持 | parallel 步骤类型 |
| 失败策略 | 无 | stop / continue |
| UI 管理 | 有 | 有 |

## 示例工作流

项目自带 5 个示例工作流，存放在 `workspace/workflows/` 下：

| 文件 | 说明 | 演示特性 |
|------|------|---------|
| `morning-briefing.yml` | 每日早间简报，工作日 8 点自动执行 | Cron 触发、时区、output_key 数据传递、if 条件分支 |
| `server-health-check.yml` | 每 10 分钟检查服务器健康状态 | tool_call 步骤、retry 重试、timeout 超时、if 条件分支 |
| `event-driven-task.yml` | 工具执行完成后自动分析结果 | event 事件触发、if 条件分支、failure_strategy=continue |
| `parallel-data-fetch.yml` | 同时获取多个数据源后汇总报告 | parallel 并行步骤、if 条件分支、多数据源汇总 |
| `manual-quick-task.yml` | 手动触发的简单多步骤任务 | 无触发器（手动触发）、if 条件分支、简单数据传递 |
