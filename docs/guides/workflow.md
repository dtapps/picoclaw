# Workflow Engine

## Overview

The Workflow Engine is a declarative multi-step task orchestration system for picoclaw. It allows you to define workflows consisting of multiple steps — each step can execute an LLM prompt, call a tool, or run sub-steps in parallel — with condition expressions controlling flow and guaranteed execution.

## Design Goals

1. **Declarative**: Workflows describe "what to do" via YAML, not "how to do it", reducing orchestration complexity
2. **Reliable Execution**: Steps support retry, timeout, and error handling with automatic on_error fallback steps
3. **Condition Control**: Branch logic via `when` condition expressions — no hardcoded flow needed
4. **Data Passing**: Steps pass results via `output_key` + template syntax `{{.step_id.key}}`, and reference variables via `{{.vars.key}}`, decoupling step dependencies
5. **Multiple Triggers**: Cron scheduling, event-driven, and manual trigger modes
6. **System Integration**: Reuses AgentLoop's SubTurn for prompts, ToolRegistry for tool calls, and EventBus for event monitoring

## System Architecture

### Layered Design

```
┌─────────────────────────────────────────────────┐
│                   Web UI                         │  Frontend pages (list/form/detail)
├─────────────────────────────────────────────────┤
│                 REST API                         │  HTTP interface (CRUD/run/stop)
├─────────────────────────────────────────────────┤
│               Service Layer                      │  Lifecycle management, trigger scheduling
├──────────┬──────────┬───────────────────────────┤
│  Engine  │ Persist  │  Triggers (Cron/Event/Manual) │  Core engine + persistence
├──────────┴──────────┴───────────────────────────┤
│              StepExecutor                        │  Step executor (retry/timeout)
├────────────────────┬────────────────────────────┤
│   AgentPromptFunc  │      ToolCallFunc           │  Callbacks: LLM prompt / tool call
├────────────────────┴────────────────────────────┤
│            AgentLoop / ToolRegistry              │  picoclaw core capabilities
└─────────────────────────────────────────────────┘
```

### Module Responsibilities

| Module | File | Responsibility |
|--------|------|---------------|
| Data Model | `model.go` | Core structs (Workflow, Step, Trigger, WorkflowInstance, StepState) with Validate() |
| Persistence | `persist.go` | PersistStore manages YAML definitions and JSON instance state; atomic writes for safety |
| Condition Eval | `conditions.go` | EvaluateCondition parses when clauses; ResolveStepTemplates replaces `{{.step_id.key}}` references |
| Step Executor | `executor.go` | StepExecutor wraps AgentPromptFunc and ToolCallFunc callbacks; ExecuteWithRetry supports configurable retry and delay |
| Core Engine | `engine.go` | Engine manages running instances, cancel functions, step orchestration (sequential/conditional/failure strategy) |
| Lifecycle Service | `service.go` | Service integrates Engine + PersistStore; provides CRUD API, trigger management (cron loop + event subscription) |
| YAML Utils | `yaml.go` | parseYAMLWorkflow / renderYAMLWorkflow serialization helpers |

### Integration

The workflow engine integrates into the Gateway via `setupWorkflowService()`, following the same pattern as `setupCronTool()`:

```
Gateway startup
  └── setupAndStartServices()
        └── setupWorkflowService()
              ├── Create PersistStore (workspace directory)
              ├── Create StepExecutor
              │     ├── AgentPromptFunc → AgentLoop.ProcessDirectWithChannel()
              │     └── ToolCallFunc → AgentLoop.GetRegistry() → Tool.Execute()
              ├── Create Engine
              ├── Create Service (inject EventBus, MessageBus)
              ├── Register WorkflowTool to Agent (if enabled)
              └── service.Start() → load workflows → subscribe to events → start cron loop
```

Key points:
- **AgentPromptFunc**: Calls LLM via `AgentLoop.ProcessDirectWithChannel()`, reusing the full Agent processing chain
- **ToolCallFunc**: Gets tools from `ToolRegistry` and executes them, reusing all registered tools (including MCP tools)
- **Tool registration**: `cfg.Tools.IsToolEnabled("workflow")` controls registration (enabled by default)
- **Command injection**: `agentLoop.SetWorkflowService(service)` injects the service into AgentLoop for `/workflow` slash commands
- **Internal API**: Engine registers internal HTTP endpoints (`/internal/workflow/*`) on the Gateway process; the Web backend proxies runtime operations via these endpoints, while CRUD operations read/write the file system directly

## Execution Flow

### Trigger Flow

```
                    ┌──────────────┐
                    │ Trigger arrives│
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        Cron due      Event match    Manual call
     (30s polling)  (EventBus sub)  (API/Tool)
              │            │            │
              └────────────┼────────────┘
                           ▼
                  Check workflow is enabled
                  Check no running instance (dedup)
                           │
                           ▼
                  engine.RunWorkflow(ctx, wf, triggerType)
```

### Step Execution Flow

```
RunWorkflow()
  │
  ├── Create WorkflowInstance (state: pending)
  ├── Save instance state to disk
  ├── Register cancel function
  │
  └── Async executeWorkflow()
        │
        ├── Update instance state: running
        │
        ├── Iterate steps (sequential)
        │     │
        │     ├── 1. Condition check: EvaluateCondition(step.When, prev result)
        │     │     ├── Condition not met → skip step (skipped)
        │     │     └── Condition met → proceed
        │     │
  │     ├── 2. Template resolution: ResolveStepTemplates(prompt/args, completed outputs)
  │     │     └── Replace {{.step_id.output_key}} with actual values
  │     │
  │     ├── 3. Delay: if step.delay is set, wait for the specified duration
  │     │     └── Cancelled during wait → step state: cancelled
  │     │
        │     ├── 4. Execute step: ExecuteWithRetry()
        │     │     ├── agent_prompt → AgentPromptFunc(ctx, prompt)
        │     │     ├── tool_call   → ToolCallFunc(ctx, tool, args)
        │     │     ├── parallel    → goroutine concurrent sub-step execution (sub-step failures respect failure_strategy)
        │     │     └── if          → evaluate when condition, execute if_true or if_false branch (branch step failures respect failure_strategy)
        │     │
        │     ├── 5. Process result
        │     │     ├── Success → record output to output_key, continue
        │     │     └── Failure →
        │     │           ├── failure_strategy=stop → abort, find on_error handler
        │     │           └── failure_strategy=continue → record failure, continue
        │     │
        │     └── 6. Save step state and instance state to disk
        │
        ├── All steps done
        │     ├── No failures → instance state: completed
        │     └── Has failures → instance state: failed
        │
        └── Clean up cancel function
```

### Retry Mechanism

```
ExecuteWithRetry()
  │
  ├── Attempt 1
  │     ├── Success → return result
  │     └── Failure →
  │           ├── retry == 0 → return error
  │           └── retry > 0 → wait retry_delay seconds
  │
  ├── Attempt 2
  │     ├── Success → return result
  │     └── Failure → continue retrying...
  │
  └── Max retries reached → return last error
```

### Error Handling Flow

```
Step execution fails
  │
  ├── failure_strategy = "stop"
  │     ├── Abort subsequent steps
  │     ├── Find steps with when="on_error"
  │     │     ├── Found → execute error handler step
  │     │     │     ├── Handler succeeds → instance state: completed
  │     │     │     └── Handler fails → instance state: failed
  │     │     └── Not found → instance marked as failed
  │     └── (does not continue the main step loop)
  │
  └── failure_strategy = "continue"
        ├── Record step as failed
        ├── Continue executing subsequent steps
        └── After all steps, instance state: failed (but subsequent steps still ran)
```

### Cancellation Flow

```
StopInstance(instanceID)
  │
  ├── Find cancel function
  ├── Call context.Cancel()
  ├── Running step receives ctx.Done() signal
  │     ├── AgentPromptFunc returns context.Canceled
  │     └── ToolCallFunc returns context.Canceled
  ├── Step state updated to cancelled
  └── Instance state updated to cancelled
```

## Core Concepts

### Workflow

A workflow is an executable unit composed of an ordered set of steps. Each workflow contains:

- **Name**: Unique identifier for reference and management, restricted to `a-zA-Z0-9_-` only
- **Description**: Purpose description
- **Triggers**: Define when the workflow auto-executes (manual/cron/event)
- **Vars**: Workflow-level variables to avoid repeating the same values (e.g., paths, URLs) across steps
- **Steps**: Ordered sequence of actions
- **Config**: Global options like failure strategy (stop/continue)

> **Name Rules**: Workflow names are restricted to `a-zA-Z0-9_-` because they are used as YAML file names. Non-ASCII or special characters would cause file system issues. The `enabled` state is a runtime property not stored in the YAML definition — it is managed via a `.disabled` marker file (see File Storage).

### Variables (Vars)

Variables allow you to define reusable values in a workflow, avoiding repeated strings (such as directory paths or site URLs) across multiple steps. Variables are defined in the `vars` field and referenced in steps via the `{{.vars.key}}` template syntax.

```yaml
vars:
  project_dir: "/root/.picoclaw/workspace/news"
  site_url: "https://example.com"
```

Referencing variables in steps:

```yaml
steps:
  - id: search
    action: agent_prompt
    prompt: "Search {{.vars.site_url}} for latest info, save to {{.vars.project_dir}}/data"
```

**How it works**:
- When the workflow starts, the engine injects `vars` into the step outputs under the `vars` key
- During template resolution, `{{.vars.key}}` is replaced with the defined variable value
- Variable values support template references (e.g., referencing previous step outputs), but are typically used for static values
- Variables can be used in `prompt`, `args`, and `when` fields
- Template references are validated on save: if a `{{.vars.key}}` references a key not defined in `vars`, an error is raised

### Step

A step is the basic execution unit, supporting four action types:

| Action Type | Description | Key Parameters |
|-------------|-------------|----------------|
| `agent_prompt` | Execute an LLM prompt | `prompt` (prompt template) |
| `tool_call` | Call a registered tool | `tool` (tool name), `args` (parameters; required params must not be empty) |
| `parallel` | Execute sub-steps concurrently | `parallel` (sub-step list) |
| `if` | Conditional branch — execute true or false branch | `when` (condition), `if_true`/`if_false` (branch steps) |

Each step supports the following configuration:

- **id**: Unique step identifier, restricted to `a-zA-Z0-9_` only, used for template references `{{.step_id.key}}` and condition evaluation
- **name**: Display name for the step (optional), supports any characters including CJK, used for UI display and notifications; falls back to id when not set. Can be referenced via `{{.self.name}}` in templates to avoid hardcoding the name in `args`
- **when**: Pre-execution condition expression; step executes only when satisfied (empty condition is equivalent to `on_success`)
- **delay**: Wait duration before executing the step, e.g., `"5s"`, `"1m"`; supports cancellation during the wait period
- **retry**: Retry configuration in structured format (default: no retry)
  - `max_attempts`: Maximum retry count
  - `delay`: Retry interval, e.g., `"10s"`
- **timeout**: Timeout in seconds
- **output_key**: Key name for output data, referenced by subsequent steps (not applicable to `parallel` steps, as sub-steps have their own output keys)

> **ID Rules**: Step IDs are restricted to `a-zA-Z0-9_` because the template syntax `{{.step_id.key}}` uses `.` as a delimiter — IDs containing `.` or other special characters would cause parsing errors, and non-ASCII characters may also cause issues. Use the `name` field for display names with Chinese or other characters.

### Trigger

Triggers determine when a workflow auto-executes:

- **manual**: Manual trigger only (default)
- **cron**: Scheduled via cron expression, e.g., `0 9 * * *` (daily at 9am)
- **event**: Triggered by listening for specific event types on the event bus

Cron trigger behavior:
- After Service starts, checks all enabled workflows' cron expressions every 30 seconds
- Uses `gronx` library to determine if a cron expression is due
- Only one instance of a workflow can run at a time (dedup)

Event trigger behavior:
- Subscribes to event stream via `EventBus.Channel().SubscribeChan()`
- Matches `Event.Kind` against `Trigger.Event`
- Also enforces single-instance running constraint

### Condition Expressions

The `when` field is a **pre-execution condition** — evaluated before the step runs; if not satisfied, the step is skipped.

| Expression | Meaning | Example |
|------------|---------|---------|
| `on_error` | Execute when the previous step failed | Error alerts, retry notifications |
| `on_success` | Execute when the previous step succeeded (default, can be omitted) | Normal follow-up processing |
| `{{.step_id.key}} == value` | Template comparison against a specific step's output | Branching based on results |

Evaluation logic (`conditions.go`):
1. Empty condition → treated as `on_success`, passes if previous step succeeded
2. `on_error` → passes if previous step state is failed
3. `on_success` → passes if previous step state is completed
4. Contains `==` → split into left/right operands, resolve template references, then string comparison

### Data Passing

Steps pass data via `output_key` and template syntax:

```
Step A (fetch_weather)                  Step B (summarize)
  action: agent_prompt                    action: agent_prompt
  prompt: "Check weather"                 prompt: "Weather: {{.fetch_weather.weather}}"
  output_key: weather  ─────────────►   Template resolved to actual value
```

Template resolution logic (`conditions.go`):
1. Regex match `{{.step_id.key}}`, `{{.vars.key}}`, or `{{.self.key}}` pattern
2. Look up corresponding value in completed steps' output data (`vars` is a special step ID storing workflow variables; `self` references the current step's own properties)
3. Replace template with actual output content
4. Applied to `prompt`, `args`, and `when` fields

**Self-property references (`self`)**:

A step can reference its own properties via `{{.self.name}}` and `{{.self.id}}` in templates (only these two fields are supported), avoiding duplicate hardcoding:

```yaml
steps:
  - id: search_maoming
    name: Maoming
    action: tool_call
    tool: web_search
    args:
      query: "{{.self.name}} news {{.vars.date}}"
```

> **Template reference validation**: All template references are validated when saving a workflow:
> - `{{.vars.key}}` — the key must be defined in `vars`
> - `{{.step_id.key}}` — the step_id must be a step with an `output_key` defined, and the key must match that step's `output_key` value
> - `{{.self.key}}` — the key only supports `id` and `name`
> - Referencing non-existent variables, steps, or output keys will raise an error, preventing silent template passthrough at runtime

> **Tool parameter validation**: Required parameters for `tool_call` steps are validated when saving a workflow:
> - The tool's parameter schema is queried from the tool registry (supports both built-in and MCP tools)
> - Missing required parameters (declared in the `required` field) will raise an error
> - Empty parameter values (empty string, null) are treated as missing and will also raise an error
> - Referencing a non-existent tool name will raise an error (MCP tools only participate in validation when their server is connected; otherwise skipped)
> - Both frontend and backend perform this validation for double protection

### Instance State Machine

```
                    ┌─────────┐
                    │ pending │
                    └────┬────┘
                         │ execution starts
                         ▼
                    ┌─────────┐
              ┌─────│ running │─────┐
              │     └────┬────┘     │
              │          │          │
         user cancel  completed   step fails
              │          │          │
              ▼          ▼          ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │cancelled │ │completed │ │  failed   │
        └──────────┘ └──────────┘ └──────────┘
                                       │
                                  condition not met
                                       │
                                       ▼
                                 ┌──────────┐
                                 │ skipped  │
                                 └──────────┘
```

Each step (including parallel sub-steps and if branch sub-steps) also has its own `StepState` tracking:
`pending` → `running` → `completed` / `failed` / `skipped` / `cancelled`.

`StepState` contains the following fields:
- **Name**: Step display name
- **Status**: Current state
- **StartedAt** / **FinishedAt**: Start and finish timestamps
- **Attempts**: Execution attempt count (1 on first execution, increments on retry)
- **Error**: Error message on failure

Unexecuted if-branch sub-steps are automatically marked as `skipped`.

> **failureStrategy in nested steps**: The `failure_strategy` setting applies not only to top-level steps but also to if branch steps and parallel sub-steps. When `failure_strategy=continue`, failures in if branches or parallel sub-steps do not halt workflow execution — instead, a warning log is recorded and execution continues.

### Real-time Event Streaming (SSE)

The workflow engine publishes state change events to the event bus (`pkg/events`), which can be consumed
via SSE for real-time UI updates:

- `workflow.instance.start` — instance begins execution
- `workflow.instance.complete` — instance finishes (success/failure/cancel)
- `workflow.step.start` — step begins execution
- `workflow.step.complete` — step finishes (success/failure)

The SSE endpoint (`GET /api/workflows/{name}/instances/{id}/stream`) delivers these events as
Server-Sent Events. The frontend automatically connects to this stream for running instances,
providing live step progress and log updates without polling.

Workflow definitions are stored in `workspace/workflows/`, one YAML file per workflow:

```yaml
name: morning-briefing
description: Daily morning briefing workflow

triggers:
  - cron: "0 8 * * *"

vars:
  city: "Beijing"
  output_dir: "/tmp/briefing"

config:
  failure_strategy: stop  # stop or continue
  notify_channel: telegram  # optional, default notification channel
  notify_chat_id: "-100xxx" # optional, default notification chat ID

steps:
  - id: fetch_weather
    name: Fetch Weather
    action: agent_prompt
    prompt: "Check today's weather forecast for {{.vars.city}}"
    output_key: weather

  - id: fetch_news
    name: Fetch News
    action: agent_prompt
    prompt: "Get today's tech news summary"
    output_key: news

  - id: summarize
    name: Generate Briefing
    action: agent_prompt
    prompt: "Generate a daily briefing based on weather {{.fetch_weather.weather}} and news {{.fetch_news.news}}, save to {{.vars.output_dir}}"
    when: "on_success"

  - id: notify_error
    action: tool_call
    tool: send_message
    args:
      message: "Morning briefing execution failed"
    when: "on_error"
```

### Complete Step Field Reference

```yaml
steps:
  - id: step_id           # Required, unique step identifier (a-zA-Z0-9_ only), for condition references and data passing
    name: Display Name    # Optional, display name supporting any characters; shown in UI and notifications, falls back to id
    action: agent_prompt   # Required, action type: agent_prompt / tool_call / parallel / if
    prompt: "..."          # Required for agent_prompt, prompt template with {{.step_id.key}} support
    tool: tool_name        # Required for tool_call, registered tool name
    args:                  # Optional for tool_call, tool parameters (values support template references; required param values must not be empty)
      key: value
    parallel:              # Required for parallel, sub-step list
      - id: sub1
        action: agent_prompt
        prompt: "..."
    when: "on_error"       # Required for if / optional for others, pre-execution condition expression
    delay: 5s              # Optional, wait duration before step execution (e.g. 5s, 1m30s)
    if_true:               # Optional for if, steps to execute when condition is true
      - id: handle_success
        action: agent_prompt
        prompt: "..."
    if_false:              # Optional for if, steps to execute when condition is false
      - id: handle_failure
        action: agent_prompt
        prompt: "..."
    output_key: result     # Optional, output key name
    retry:                 # Optional, retry configuration
      max_attempts: 3
      delay: 10s
    timeout: 60s           # Optional, timeout duration
```

### if Conditional Step

The `if` step evaluates the `when` condition and executes either the `if_true` or `if_false` branch:

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
        prompt: "Health check failed, notify admin"
    if_false:
      - id: log_success
        action: agent_prompt
        prompt: "Server is healthy"
```

The `when` condition for `if` steps supports:
- `on_error`: Takes `if_true` branch when previous step failed, `if_false` when succeeded
- `on_success`: Takes `if_true` branch when previous step succeeded, `if_false` when failed
- `{{.step_id.key}} == value`: Takes `if_true` branch when a specific step output equals a value

> **if step events and notifications**: `if` steps also emit `workflow.step.start` and `workflow.step.complete` SSE events, and trigger `onStepStart`/`onStepComplete` notification callbacks (consistent with agent_prompt/tool_call steps).

### Visual Editor

The workflow editor page provides a visual editor based on Sequential Workflow Designer, supporting:
- Drag steps from the toolbox onto the canvas (each step type shows a description)
- Click a step to edit its properties in the right panel (ID field accepts only alphanumeric characters and underscores; Name field accepts any characters; retry configuration is shown only for agent_prompt and tool_call steps)
- Step types are fixed after dragging (Agent Prompt / Tool Call / Parallel / If)
- Auto-assigned step IDs by type when dragging from toolbox (e.g., `prompt_1`, `tool_1`, `prompt_2`, `if_1`)
- Frontend validation before saving with i18n error messages (recursive sub-step validation, nested ID uniqueness check, template reference validation, tool_call required parameter validation)
- Required parameters marked with red border and `*` prefix in value placeholder
- Condition options are localized via i18n ("Run when previous step succeeded"/"Run when previous step failed"/"Custom Condition")
- Variable key duplicate warning when editing vars
- Parallel steps display branches side by side; branches can be added or removed dynamically
- if steps render as diamonds with true/false branch lines
- Automatic save to YAML definition
- Dynamic light/dark theme switching

#### Toolbox Steps

| Step | Description |
|------|-------------|
| Agent Prompt | Send a prompt to the LLM agent and get a response |
| Tool Call | Call a registered tool by name with parameters |
| Parallel | Execute multiple sub-steps concurrently (container step) |
| If | Conditional branch: execute true or false path based on condition |

## REST API

| Method | Path | Description | Dependency |
|--------|------|-------------|------------|
| GET | `/api/workflows` | List all workflows | File system only |
| POST | `/api/workflows` | Create a workflow (default disabled, 409 on duplicate name) | File system only |
| GET | `/api/workflows/{name}` | Get workflow details | File system only |
| PUT | `/api/workflows/{name}` | Update a workflow | File system only |
| DELETE | `/api/workflows/{name}` | Delete a workflow | File system only |
| POST | `/api/workflows/{name}/run` | Trigger execution (falls back to bound channel for notifications) | Requires Gateway running |
| POST | `/api/workflows/{name}/stop` | Stop all running instances | Requires Gateway running |
| POST | `/api/workflows/{name}/toggle` | Enable/disable | File system only |
| GET | `/api/workflows/{name}/instances` | List execution history (sorted by time descending) | Requires Gateway running |
| GET | `/api/workflows/{name}/instances/{id}` | Get instance details | Requires Gateway running |
| GET | `/api/workflows/{name}/instances/{id}/stream` | SSE stream for real-time instance updates | Requires Gateway running |
| DELETE | `/api/workflows/{name}/instances/{id}` | Delete execution record | Requires Gateway running |

> CRUD operations use a temporary PersistStore to read/write files, independent of the Gateway process.
> Run/stop/instance/delete operations are proxied to the Gateway's internal API (`/internal/workflow/*`); some operations can fall back to direct file system access when the Gateway is unavailable.

### Create Workflow Example

> Workflows created via REST API default to **disabled**. Use the toggle endpoint to enable after confirming the definition.
> In the Web UI, Run/Stop/Disable actions require confirmation before execution.

```bash
curl -X POST http://localhost:3000/api/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "name": "server-health-check",
    "description": "Server health check",
    "triggers": [{"cron": "*/30 * * * *"}],
    "steps": [
      {
        "id": "check",
        "action": "agent_prompt",
        "prompt": "Check server CPU, memory and disk usage",
        "output_key": "health"
      },
      {
        "id": "alert",
        "action": "tool_call",
        "tool": "send_message",
        "args": {"message": "Server anomaly: {{.check.health}}"},
        "when": "on_error"
      }
    ]
  }'
```

### Trigger Execution Example

```bash
curl -X POST http://localhost:3000/api/workflows/server-health-check/run
```

## LLM Tool Interface

The agent can manage workflows via the `workflow` tool:

| Action | Description | Required Parameters |
|--------|-------------|---------------------|
| `list` | List all workflows | - |
| `show` | View workflow details | `name` |
| `run` | Trigger execution (auto-passes current channel for notifications) | `name` |
| `stop` | Stop an instance | `instance_id` |
| `create` | Create a workflow | `name`, `steps_yaml` |
| `delete` | Delete a workflow | `name` |
| `bind` | Bind current channel for completion notifications | `name` |
| `unbind` | Remove channel binding | `name` |
| `enable` | Enable a workflow | `name` |
| `disable` | Disable a workflow | `name` |
| `instances` | View execution history | `name` |

The `bind` and `run` actions automatically capture the channel/chatID from the conversation context (same pattern as the Cron tool). After binding, completion notifications (success/failure/cancel) are sent to the bound channel.

### Channel Binding Example (in conversation)

```
User: Bind the morning-briefing workflow to this channel

Agent: → workflow action=bind name=morning-briefing
       "Workflow 'morning-briefing' bound to channel telegram (chat_id: -100xxx). 
        Completion notifications will be sent here."
```

## Slash Command Interface

Workflows can also be managed via the `/workflow` slash command in any chat channel:

| Command | Description |
|---------|-------------|
| `/workflow list` | List all workflows |
| `/workflow run <name>` | Trigger a workflow (auto-passes current channel for notifications) |
| `/workflow show <name>` | View workflow details |
| `/workflow bind <name>` | Bind current channel for notifications |
| `/workflow unbind <name>` | Remove channel binding |
| `/workflow enable <name>` | Enable a workflow |
| `/workflow disable <name>` | Disable a workflow |
| `/workflow instances <name>` | View execution history |
| `/workflow stop <instance-id>` | Stop a running instance |

Usage example:

```
/workflow bind morning-briefing     → Bind notification channel
/workflow run morning-briefing      → Trigger with current channel bound
/workflow instances morning-briefing → View execution history
```

## File Storage

| Path | Description |
|------|-------------|
| `workspace/workflows/{name}.yml` | Workflow definition file (YAML, case-sensitive on all platforms) |
| `workspace/workflows/{name}.disabled` | Disable marker (presence = disabled) |
| `workspace/workflows/.state/{name}_{instanceID}.json` | Instance state file (JSON, atomic writes) |

Persistence mechanism (`persist.go`):
- Definition files use YAML format for human readability and editing
- Instance state files are stored in `.state/` subdirectory, written atomically via `fileutil.WriteFileAtomic` to prevent corruption from interrupted writes
- The `enabled` field is a runtime state (`yaml:"-"` tag) not serialized to the YAML file; disable feature uses create/delete of `.disabled` suffix file to persist the state
- File names preserve case sensitivity; `MyWorkflow` and `myworkflow` are treated as different workflows
- File names processed through `sanitizeName()` for safety

## Execution Results & Notifications

### Current Query Methods

After workflow execution completes, results can be viewed via:

1. **REST API**: `GET /api/workflows/{name}/instances` — query execution history
2. **REST API**: `GET /api/workflows/{name}/instances/{id}` — view instance details and step status
3. **Slash Command**: `/workflow instances <name>` — view execution history in chat
4. **LLM Tool**: `workflow action=instances name=xxx` — view execution history via agent
5. **Web UI**: History button on workflow card — view execution history and logs

### Channel Notification

Workflows support four-phase automatic channel notification:

1. **Start notification**: When a workflow begins execution, sends a `🚀 Workflow 'xxx' started` notification
2. **Step start notification**: When each step begins execution, sends a `▶️ Step 'xxx' started (Agent Prompt)` notification
3. **Step output**: After each `agent_prompt` step completes, the AI response is pushed to the channel in real time
4. **Completion notification**: When a workflow finishes, sends a summary notification (status, duration, error info)

Notifications are implemented via four Engine callbacks (`onStart`, `onStepStart`, `onStepComplete`, `onComplete`) registered in Gateway's `setupWorkflowService()`, sent via `msgBus.PublishOutbound`.

**Notification target priority**:

The engine determines the notification channel for each execution using this priority:

1. **Run-time channel** — When triggered from a chat context (`/workflow run`, `workflow action=run`), the current channel/chatID is passed to the instance and used for notifications
2. **Persistent binding** — When triggered without a chat context (Web UI, cron, event), the engine falls back to the channel set via `/workflow bind` or `workflow action=bind`
3. **YAML config fallback** — If neither is available, the engine falls back to `notify_channel`/`notify_chat_id` from the workflow config

> **Note**: `/workflow bind` and `workflow action=bind` persist the channel into the workflow definition, so future cron/event-triggered runs also send notifications there. `/workflow run` and `workflow action=run` only pass the channel for that specific execution — they do NOT update the persistent binding.

Channel binding follows the same pattern as the Cron tool: `ToolChannel(ctx)` / `ToolChatID(ctx)` extract channel info from the execution context. When a workflow starts, a step begins, a step completes, or the workflow finishes, the `onStart`, `onStepStart`, `onStepComplete`, and `onComplete` callbacks send notifications to the bound channel via `msgBus.PublishOutbound`.

> Each workflow execution uses an independent session (`agent:workflow-{uuid}`), no shared context. Channel info is inherited from the workflow instance's `Channel`/`ChatID` to the step executor, not hardcoded.

**YAML config fallback**:

```yaml
config:
  notify_channel: telegram   # Default notification channel
  notify_chat_id: "-100xxx"  # Default notification chat ID
```

When a workflow is triggered without a channel context (e.g., cron trigger, event trigger, or web UI), the engine falls back to `notify_channel`/`notify_chat_id` from the workflow config.

### Notification via Steps

In addition to automatic channel notification, you can also compose notification steps within the workflow:

```yaml
# Push result to channel after success
steps:
  - id: do_work
    action: agent_prompt
    prompt: "Execute task"
    output_key: result

  - id: notify_success
    action: agent_prompt
    prompt: "Please send the following result to the user: {{.do_work.result}}"
    when: "on_success"

  - id: notify_error
    action: agent_prompt
    prompt: "Task execution failed, please notify the admin"
    when: "on_error"
```

> **Note**: `agent_prompt` steps execute through AgentLoop. If the Agent has a default channel configured, results will be automatically sent to that channel.

### Execution Logs

Each workflow instance records structured execution logs with timestamps, step IDs, and log levels (info/warn/error). Logs are stored in the instance and can be viewed via:

1. **REST API**: Instance detail endpoint returns logs in the response
2. **UI**: Execution logs card on the workflow detail page; History dialog on the workflow list page
3. **Channel notification**: When a bound channel is notified on completion, execution logs are included in the notification message

### Current Limitations

| Feature | Status | Notes |
|---------|--------|-------|
| Execution history query | ✅ Supported | Via REST API, slash command, or LLM tool |
| Channel push notification | ✅ Supported | Four-phase notification: start / step start / step output / completion |
| Execution logs | ✅ Supported | Stored in instance, viewable via API and UI |
| Execution logs (realtime) | ✅ Supported | SSE streaming at `/api/workflows/{name}/instances/{id}/stream`, auto-updates UI |
| Sub-step state tracking | ✅ Supported | parallel/if sub-steps have independent StepStates |
| UI execution history | ✅ Supported | History button on workflow card, execution logs in detail page, delete records |
| Instance deletion | ✅ Supported | DELETE API + UI delete button |

## Comparison with CronService

| Feature | CronService | Workflow Engine |
|---------|------------|-----------------|
| Execution | Single prompt | Multi-step orchestration |
| Condition control | None | when conditions, template comparison |
| Data passing | None | output_key + template references |
| Error handling | None | on_error handling, retries |
| Trigger types | Cron only | Cron + Event + Manual |
| Parallel execution | Not supported | parallel step type |
| Failure strategy | None | stop / continue |
| UI management | Yes | Yes |

## Example Workflows

Five example workflows are included in `workspace/workflows/`:

| File | Description | Features Demonstrated |
|------|-------------|----------------------|
| `morning-briefing.yml` | Daily morning briefing, weekdays at 8am | Cron trigger, timezone, output_key data passing, if conditional branch |
| `server-health-check.yml` | Server health check every 10 minutes | tool_call step, retry, timeout, if conditional branch |
| `event-driven-task.yml` | Auto-analyze tool execution results | event trigger, if conditional branch, failure_strategy=continue |
| `parallel-data-fetch.yml` | Fetch multiple data sources concurrently, compile report | parallel step, if conditional branch, multi-source aggregation |
| `manual-quick-task.yml` | Manual multi-step task | No trigger (manual only), if conditional branch, simple data passing |
