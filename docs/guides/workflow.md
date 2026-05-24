# Workflow Engine

## Overview

The Workflow Engine is a declarative multi-step task orchestration system for picoclaw. It allows you to define workflows consisting of multiple steps — each step can execute an LLM prompt, call a tool, or run sub-steps in parallel — with condition expressions controlling flow and guaranteed execution.

## Design Goals

1. **Declarative**: Workflows describe "what to do" via YAML, not "how to do it", reducing orchestration complexity
2. **Reliable Execution**: Steps support retry, timeout, and error handling with automatic on_error fallback steps
3. **Condition Control**: Branch logic via `when` condition expressions — no hardcoded flow needed
4. **Data Passing**: Steps pass results via `output_key` + template syntax `{{.step_id.key}}`, and reference variables via `{{.vars.key}}`, decoupling step dependencies
5. **Multiple Triggers**: Cron scheduling, one-time execution (At), interval execution (Interval), event-driven, and manual trigger modes
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
│  Engine  │ Persist  │  Triggers (Cron/At/Interval/Event/Manual) │  Core engine + persistence
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
  - Applies `step.skills` config to control skill loading (`mode: off` skips skill loading)
  - Applies `step.tools` config to control tool definitions sending (`mode: off` disables tools)
  - Applies `config.history` config to control history context loading
  - Applies `config.system_prompt` config to control system prompt injection
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
        │     │     ├── agent_prompt → Apply step.skills and step.tools config to context → AgentPromptFunc(ctx, prompt)
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

- **Version**: Configuration format version (`version`), currently version 2
- **Name**: Unique identifier for reference and management, restricted to `a-zA-Z0-9_-` only
- **Description**: Purpose description
- **Triggers**: Define when the workflow auto-executes (manual/cron/event)
- **Vars**: Workflow-level variables to avoid repeating the same values (e.g., paths, URLs) across steps
- **Steps**: Ordered sequence of actions
- **Config**: Global options like failure strategy (stop/continue), notification channels, etc.

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
| `agent_prompt` | Execute an LLM prompt | `prompt` (prompt template), `skills` (skills config), `tools` (tools config) |
| `tool_call` | Call a registered tool | `tool` (tool name), `args` (parameters; required params must not be empty) |
| `parallel` | Execute sub-steps concurrently | `parallel` (sub-step list) |
| `if` | Conditional branch — execute true or false branch | `when` (condition), `if_true`/`if_false` (branch steps) |
| `notify` | Send notification message to bound channel | `message` (message content, supports templates) |

Each step supports the following configuration:

- **id**: Unique step identifier, restricted to `a-zA-Z0-9_` only, used for template references `{{.step_id.key}}` and condition evaluation
- **name**: Display name for the step (optional), supports any characters including CJK, used for UI display and notifications; falls back to id when not set. Can be referenced via `{{.self.name}}` in templates to avoid hardcoding the name in `args`
- **enabled**: Whether the step is enabled (optional; defaults to `true`). When set to `false`, the step is skipped (status marked as `skipped`)
- **when**: Pre-execution condition expression; step executes only when satisfied (empty condition is equivalent to `on_success`)
- **delay**: Wait duration before executing the step, e.g., `"5s"`, `"1m"`; supports cancellation during the wait period
- **retry**: Retry configuration in structured format (default: no retry)
  - `max_attempts`: Maximum retry count
  - `delay`: Retry interval, e.g., `"10s"`
- **timeout**: Timeout duration (optional), e.g., `"30s"`, `"5m"`. **Defaults to 30 minutes** when omitted or empty; minimum value is 1 second
- **output_key**: Key name for output data, referenced by subsequent steps (not applicable to `parallel` steps, as sub-steps have their own output keys)
- **notify_on_start**: Whether to send a "step started" notification to the bound channel (optional; defaults to `true`). When set to `false`, the start notification is skipped
- **notify_on_complete**: Whether to send a "step completed" notification to the bound channel (optional; defaults to `true`). The `notify` step is not affected by this field and always sends the message content directly
- **skills**: Skills config (only for `agent_prompt`), controls whether to load skill directories and activated skill prompts
  - `mode`: `default` (load skills) | `off` (do not load skills)
- **tools**: Tools config (only for `agent_prompt`), controls whether to send tool definitions to AI
  - `mode`: `default` (send tool definitions) | `off` (do not send tool definitions)

> **ID Rules**: Step IDs are restricted to `a-zA-Z0-9_` because the template syntax `{{.step_id.key}}` uses `.` as a delimiter — IDs containing `.` or other special characters would cause parsing errors, and non-ASCII characters may also cause issues. Use the `name` field for display names with Chinese or other characters.

**Skills and Tools Config Example**:

```yaml
steps:
  - id: analyze
    action: agent_prompt
    prompt: "Analyze this code"
    skills:
      mode: off  # Do not load skill prompts, saves tokens
    tools:
      mode: default  # Allow tool calls
```

### Working Directory (Workdir)

The working directory specifies the default directory for command execution in `tool_call` steps, avoiding the need to set `cwd` in each step's `args`. When set in `config`, the engine automatically injects workdir into the arguments of all `tool_call` steps that don't explicitly set `cwd`.

```yaml
config:
  workdir: "/root/.picoclaw/workspace/my-project"
```

**Priority**: Explicit `cwd` in step `args` > Workflow `config.workdir`. If `cwd` is already set in a step's `args`, workdir will not be injected, respecting the step's explicit configuration.

**Template References**: workdir supports template variables — you can use template syntax like `{{.vars.key}}`:

```yaml
vars:
  project_dir: "/root/.picoclaw/workspace/my-project"

config:
  workdir: "{{.vars.project_dir}}"
```

**Per-step override**: If a specific `tool_call` step needs a different working directory, simply set `cwd` in its `args`:

```yaml
steps:
  - id: git_status
    action: tool_call
    tool: exec
    args:
      action: run
      command: "git status"
      cwd: "/path/to/other-project"
```

### History Context

History context config controls whether to load previous conversation history when executing workflows. Once set in `config`, all `agent_prompt` steps will follow this config.

```yaml
config:
  history:
    mode: default  # default (load history) | off (do not load history)
```

**Use cases**:
- `default`: Normal workflow, can access previous conversation context
- `off`: Each step executes like a new conversation, not relying on previous history, suitable for independent tasks

### System Prompt

System prompt config controls whether to inject PicoClaw's system prompt (identity, workspace, memory, etc.) when executing workflows. Once set in `config`, all `agent_prompt` steps will follow this config.

```yaml
config:
  system_prompt:
    mode: default  # default (inject system prompt) | off (do not inject system prompt)
```

**Use cases**:
- `default`: Normal workflow, includes complete system context
- `off`: Only keeps externally provided system prompt, suitable for scenarios requiring streamlined prompts

### Trigger

Triggers determine when a workflow auto-executes:

- **manual**: Manual trigger only (default)
- **cron**: Scheduled via cron expression, e.g., `0 9 * * *` (daily at 9am)
- **at**: One-time trigger at specified datetime, e.g., `2025-05-15 09:00:00`
- **interval**: Repeated trigger at fixed intervals, e.g., `30m` (every 30 minutes), `1h` (every hour)
- **event**: Triggered by listening for specific event types on the event bus

#### Cron Trigger
- After Service starts, checks all enabled workflows' cron expressions every 60 seconds
- Uses `gronx.NextTickAfter` to calculate the next trigger time and verifies if current time is within the matching minute window
- **Precision**: Triggers only when the current time falls within the exact minute specified by the cron expression (e.g., `0 12 * * *` triggers between 12:00:00-12:00:59)
- **No early triggering**: Will NOT trigger before the scheduled time (e.g., won't trigger at 11:59:31 for a 12:00 schedule)
- Only one instance of a workflow can run at a time (deduplication with 90-second window)
- Supports timezone setting (`tz` field)
- Deduplication: Same workflow + same cron expression won't re-trigger within 90 seconds

#### At Trigger (One-time)
- Executes once at the specified date and time
- Time format: `2025-05-15 09:00:00` or `2025-05-15 09:00`
- **Execution window**: Triggers if current time is within 60 seconds after the scheduled time (e.g., for `12:00:00`, triggers between 12:00:00-12:01:00)
- Supports timezone setting (`tz` field)
- Automatically marked as completed after execution, won't trigger again
- Deduplication: Each `at` trigger executes only once

#### Interval Trigger
- Repeatedly executes at fixed time intervals
- Format: Go duration format, e.g., `30m` (30 minutes), `1h` (1 hour), `2h30m` (2 hours 30 minutes)
- **First execution**: Triggers immediately when service starts (if no previous execution recorded)
- **Subsequent executions**: Triggers only after the specified interval has elapsed since last execution
- Supports timezone setting (`tz` field)
- Deduplication: Tracks last execution time per workflow + interval combination

#### Event Trigger

The event trigger listens to the system event bus and automatically triggers workflow execution when specific events occur.

**Basic Features:**
- Subscribes to event stream via `EventBus.Channel().SubscribeChan()`
- Matches `Event.Kind` against `Trigger.Event`
- Also enforces single-instance running constraint

**Standard Event Types:**

| Event Type | Description | Common Use Cases |
|------------|-------------|------------------|
| `agent.tool.exec_start` | Tool starts execution | Log tool invocations |
| `agent.tool.exec_end` | Tool execution completed | Analyze tool execution results |
| `agent.tool.exec_error` | Tool execution error | Error handling and alerting |
| `agent.prompt.start` | Agent starts processing prompt | Log conversation start |
| `agent.prompt.end` | Agent finishes processing prompt | Log conversation end |
| `agent.response` | Agent generates response | Post-response processing |
| `workflow.instance.start` | Workflow instance starts | Workflow chaining |
| `workflow.instance.complete` | Workflow instance completed | Workflow chaining |
| `workflow.instance.error` | Workflow instance error | Error handling |
| `system.startup` | System startup | Initialization tasks |
| `system.shutdown` | System shutdown | Cleanup tasks |

**Event Filters:**
- Supports filtering event content via the `event_filters` field
- Example: Only respond to completion events for a specific tool `{ "tool": "git_commit" }`

**Event Variable Mapping:**
- Event data is automatically mapped to workflow variables, accessible via `{{.vars.event_xxx}}` in steps
- Default variables provided:
  - `event_kind` - Event type
  - `event_time` - Event occurrence time
  - `event_tool` - Tool name (tool events)
  - `event_result` - Execution result (completion events)
  - `event_error` - Error message (error events)

**Example Configuration:**
```yaml
triggers:
  - event: "agent.tool.exec_end"
    event_filters:
      tool: "git_commit"  # Only respond to git_commit tool completion events
```

### Trigger Precision and Timing

**How triggers are evaluated:**
- The workflow service runs a polling loop that checks all enabled workflows every 60 seconds
- For each trigger type, the service uses time-based logic to determine if execution should occur:

| Trigger Type | Check Logic | Precision |
|-------------|-------------|----------|
| **cron** | Uses `gronx.NextTickAfter` to calculate next trigger time, then verifies current time is within the matching minute window | ±0-59 seconds (within the cron minute) |
| **at** | Checks if current time is within 60 seconds after the scheduled time | ±0-60 seconds after scheduled time |
| **interval** | Checks if time since last execution >= configured interval | Exact interval enforcement |

**Example: Cron Trigger Behavior**
```yaml
triggers:
  - cron: "0 12 * * *"  # Daily at 12:00
    tz: "Asia/Shanghai"
```

With this configuration:
- ✅ Will trigger at 12:00:00, 12:00:30, 12:00:59 (any time within the 12:00 minute)
- ❌ Will NOT trigger at 11:59:31 (before the scheduled time)
- ❌ Will NOT trigger at 12:01:00 (after the 12:00 minute has passed)

**Deduplication:**
- Each trigger type has built-in deduplication to prevent multiple executions:
  - `cron`: Same workflow + same cron expression won't re-trigger within 90 seconds
  - `at`: Each at-time trigger executes only once
  - `interval`: Tracks last execution time per workflow + interval combination

### Condition Expressions

The `when` field is a **pre-execution condition** — evaluated before the step runs; if not satisfied, the step is skipped.

| Expression | Meaning | Example |
|------------|---------|---------|
| `on_error` | Execute when the previous step failed | Error alerts, retry notifications |
| `on_success` | Execute when the previous step succeeded (default, can be omitted) | Normal follow-up processing |
| `{{.step_id.key}} == value` | Equals | Check if output equals the specified value |
| `{{.step_id.key}} != value` | Not equals | Check if output does not equal the specified value |
| `{{.step_id.key}} contains value` | Contains | Check if output contains the specified substring |
| `{{.step_id.key}} > value` | Greater than | Numeric or string comparison |
| `{{.step_id.key}} < value` | Less than | Numeric or string comparison |
| `{{.step_id.key}} >= value` | Greater than or equal | Numeric or string comparison |
| `{{.step_id.key}} <= value` | Less than or equal | Numeric or string comparison |

Evaluation logic (`conditions.go`):
1. Empty condition → treated as `on_success`, passes if previous step succeeded
2. `on_error` → passes if previous step state is failed
3. `on_success` → passes if previous step state is completed
4. Comparison expression → split into left/right operands, resolve template references, then compare using the operator

**Comparison Operators:**
- `==` / `!=`: String exact match
- `contains`: Substring match, useful for checking if output contains specific text
- `>` / `<` / `>=` / `<=`: Try numeric comparison first; if both sides can be parsed as numbers, compare numerically, otherwise compare as strings

**Usage Examples:**
```yaml
steps:
  - id: check_status
    action: tool_call
    tool: exec
    args:
      action: run
      command: "check_service.sh"
    output_key: result
    
  - id: handle_success
    action: notify
    when: '{{.check_status.result}} == "ok"'
    message: "Service is normal"
    
  - id: handle_contains
    action: notify
    when: '{{.check_status.result}} contains "running"'
    message: "Service is running"
    
  - id: check_count
    action: tool_call
    tool: exec
    args:
      action: run
      command: "get_count.sh"
    output_key: count
    
  - id: handle_high_count
    action: notify
    when: '{{.check_count.count}} > 100'
    message: "Count exceeds threshold"
```

### Data Passing

Steps pass data via `output_key` and template syntax:

```
Step A (fetch_weather)                  Step B (summarize)
  action: agent_prompt                    action: agent_prompt
  prompt: "Check weather"                 prompt: "Weather: {{.fetch_weather.weather}}"
  output_key: weather  ─────────────►   Template resolved to actual value
```

Template resolution logic (`conditions.go`):
1. Regex match `{{.step_id.key}}`, `{{.vars.key}}`, `{{.self.key}}`, or `{{.fn.xxx}}` pattern
2. Look up corresponding value in completed steps' output data (`vars` is a special step ID storing workflow variables; `self` references the current step's own properties; `fn` invokes built-in template functions)
3. Replace template with actual output content
4. Applied to `prompt`, `args`, and `when` fields

**Step Status References (`_status` and `_error`)**:

After each step executes, the system automatically stores its execution status, accessible via the special `_status` and `_error` keys:

| Field | Description | Possible Values |
|-------|-------------|-----------------|
| `_status` | Step execution status | `completed` (success), `failed` (failure) |
| `_error` | Error message | Error description on failure, empty on success |

Usage example:

```yaml
steps:
  - id: task1
    action: agent_prompt
    prompt: "Execute a task"
    
  - id: check_result
    action: agent_prompt
    prompt: "task1 status: {{.task1._status}}"
    
  - id: handle_success
    action: agent_prompt
    when: "{{.task1._status}} == completed"
    prompt: "task1 succeeded, continue processing"
    
  - id: handle_failure
    action: agent_prompt
    when: "{{.task1._status}} == failed"
    prompt: "task1 failed, error: {{.task1._error}}"
```

> **Note**: `_status` and `_error` are system-reserved fields that can be used without defining them in `output_key`.

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

**Template Functions (`fn`)**:

Template references support built-in functions via `{{.fn.function_name}}` or `{{.fn.function_name "argument"}}` syntax. These are evaluated at step execution time to dynamically insert values such as the current time, date, or environment variables — without requiring an extra step.

**Time and Date Functions:**

| Function | Syntax | Description | Example Output |
|----------|--------|-------------|----------------|
| `now` | Current UTC time | `2026-05-13 08:30:00` |
| `now_tz` | `{{.fn.now_tz "Asia/Shanghai"}}` | Current time in specified timezone | `2026-05-13 16:30:00` |
| `date` | Current UTC date | `2026-05-13` |
| `date_tz` | `{{.fn.date_tz "Asia/Shanghai"}}` | Current date in specified timezone | `2026-05-13` |
| `unix` | Current Unix timestamp | `1747170600` |
| `days_ago` | `{{.fn.days_ago 7}}` | Date N days ago | `2026-05-06` |
| `days_from_now` | `{{.fn.days_from_now 3}}` | Date N days from now | `2026-05-16` |
| `hours_ago` | `{{.fn.hours_ago 24}}` | Time N hours ago | `2026-05-12 08:30:00` |
| `hours_from_now` | `{{.fn.hours_from_now 2}}` | Time N hours from now | `2026-05-13 10:30:00` |
| `minutes_ago` | `{{.fn.minutes_ago 30}}` | Time N minutes ago | `2026-05-13 08:00:00` |
| `minutes_from_now` | `{{.fn.minutes_from_now 15}}` | Time N minutes from now | `2026-05-13 08:45:00` |
| `weeks_ago` | `{{.fn.weeks_ago 2}}` | Date N weeks ago | `2026-04-29` |
| `day_of_week` | Day of week (1=Monday, 7=Sunday) | `6` |
| `format_time` | `{{.fn.format_time "2006/01/02"}}` | Custom time format | `2026/05/13` |

**Other Functions:**

| Function | Syntax | Description | Example Output |
|----------|--------|-------------|----------------|
| `env` | `{{.fn.env "HOME"}}` | Get environment variable value | `/root` |

Template functions are evaluated in real time at step execution and can be used in `prompt`, `args`, and `when` fields. For example:

```yaml
vars:
  tz: "Asia/Shanghai"

steps:
  - id: today
    action: agent_prompt
    prompt: "Today is {{.fn.date_tz "Asia/Shanghai"}}, please generate a daily briefing"
    output_key: result
```

> **Template Functions vs Tool Calls**: Template functions are evaluated directly during template resolution — no extra step needed, and the output is concise (e.g., `2026-05-13`). In contrast, calling tools like `get_current_time` via `tool_call` creates an additional step and produces verbose output with prefixes (e.g., `Current time (Asia/Shanghai): 2026-05-13`), requiring extra processing to extract the date. Use template functions when you only need the date/time value.

> **Template reference validation**: All template references are validated when saving a workflow:
> - `{{.vars.key}}` — the key must be defined in `vars`
> - `{{.step_id.key}}` — the step_id must be an existing step, and the key must match that step's `output_key` value (or the default `result`)
> - `{{.step_id._status}}` and `{{.step_id._error}}` — system-reserved fields for accessing step execution status, usable without definition
> - `{{.self.key}}` — the key only supports `id` and `name`
> - `{{.fn.xxx}}` — the function name must be a supported template function (now, now_tz, date, date_tz, unix, days_ago, days_from_now, hours_ago, hours_from_now, minutes_ago, minutes_from_now, weeks_ago, day_of_week, format_time, env)
> - Referencing non-existent variables, steps, output keys, or functions will raise an error, preventing silent template passthrough at runtime

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
- **ResolvedInput**: Resolved input parameters (containing Prompt and Args), used to display the actual values after template variable substitution in instance details

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
  workdir: "/root/.picoclaw/workspace/news"  # optional, default working directory for tool_call steps
  notify_channels:         # optional, notification targets list
    - channel: telegram
      chat_id: "-100xxx"
  history:                 # optional, history context config
    mode: default          # default (load history) | off (do not load history)
  system_prompt:           # optional, system prompt config
    mode: default          # default (inject system prompt) | off (do not inject system prompt)

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
    action: agent_prompt   # Required, action type: agent_prompt / tool_call / parallel / if / notify
    enabled: true          # Optional, whether enabled (default true); false skips this step (status = skipped)
    prompt: "..."          # Required for agent_prompt, prompt template with {{.step_id.key}} support
    skills:                # Optional for agent_prompt, skills config
      mode: default        # default (load skills) | off (do not load skills)
    tools:                 # Optional for agent_prompt, tools config
      mode: default        # default (send tool definitions) | off (do not send tool definitions)
    tool: tool_name        # Required for tool_call, registered tool name
    args:                  # Optional for tool_call, tool parameters (values support template references; required param values must not be empty)
      key: value
    parallel:              # Required for parallel, sub-step list
      - id: sub1
        action: agent_prompt
        prompt: "..."
    when: "on_error"       # Required for if / optional for others, pre-execution condition expression
    delay: 5s              # Optional, wait duration before step execution (e.g. 5s, 1m30s)
    timeout: 30m           # Optional, timeout duration (default 30m), e.g. 60s, 5m
    if_true:               # Optional for if, steps to execute when condition is true
      - id: handle_success
        action: agent_prompt
        prompt: "..."
    if_false:              # Optional for if, steps to execute when condition is false
      - id: handle_failure
        action: agent_prompt
        prompt: "..."
    output_key: result     # Optional, output key name
    notify_on_start: true   # Optional, whether to send "step started" notification (default true)
    notify_on_complete: true # Optional, whether to send "step completed" notification (default true; not applicable to notify/agent_prompt steps)
    message: "..."          # Required for notify, message content, supports template syntax
    retry:                 # Optional, retry configuration
      max_attempts: 3
      delay: 10s
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
| Notify | Send notification message to bound channel |

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
| `bind` | Bind current channel for notifications (append mode) | `name` |
| `unbind` | Unbind current channel from notifications | `name` |
| `enable` | Enable a workflow | `name` |
| `disable` | Disable a workflow | `name` |
| `instances` | View execution history | `name` |

The `bind` and `run` actions automatically capture the channel/chatID from the conversation context (same pattern as the Cron tool). After binding, completion notifications (success/failure/cancel) are sent to the bound channel.

**Multi-channel Support**:

- The `bind` action uses append mode, adding the current channel to the notification list without overwriting existing bindings
- The `unbind` action automatically unbinds the current conversation channel
- When a workflow executes, it sends notifications to all bound channels

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
| `/workflow bind <name>` | Bind current channel for notifications (append mode) |
| `/workflow unbind <name>` | Unbind current channel from notifications |
| `/workflow channels <name>` | List all notification channels bound to a workflow |
| `/workflow enable <name>` | Enable a workflow |
| `/workflow disable <name>` | Disable a workflow |
| `/workflow instances <name>` | View execution history |
| `/workflow stop <instance-id>` | Stop a running instance |

Usage example:

```
# Bind in Telegram channel
/workflow bind morning-briefing      → Add current Telegram channel to notification list

# View all bound channels
/workflow channels morning-briefing   → Show all notification channels

# Switch to DingTalk group and bind
/workflow bind morning-briefing      → Add DingTalk channel to notification list too

# Unbind in DingTalk group
/workflow unbind morning-briefing    → Remove only DingTalk channel binding

# Trigger execution
/workflow run morning-briefing       → Trigger with current channel bound

# View execution history
/workflow instances morning-briefing → View execution history
```

**Multi-channel Notifications**:

- The `bind` command uses **append mode**, it doesn't overwrite existing bindings but adds the current channel to the notification list
- The `unbind` command automatically unbinds the **current conversation channel**, no need to specify manually
- The `channels` command shows all bound notification channels
- When a workflow executes, it sends notifications to all bound channels

Example:
```bash
# Execute in Telegram
/workflow bind my-workflow
# → Add Telegram channel to notification list

# Execute in DingTalk
/workflow bind my-workflow
# → Add DingTalk channel to notification list (Telegram still retained)

# View bindings
/workflow channels my-workflow
# → Shows:
#   1. telegram (ID: -100xxx)
#   2. dingtalk (ID: cidSkk33JUIC1Od8i6iLuExy/x8z5ceMX5oFLqfIL1hmqs=)

# Unbind in DingTalk
/workflow unbind my-workflow
# → Remove only DingTalk channel, Telegram still retained
```

## File Storage

| Path | Description |
|------|-------------|
| `workspace/workflows/{name}.yml` | Workflow definition file (YAML, case-sensitive on all platforms) |
| `workspace/workflows/{name}.disabled` | Disable marker (presence = disabled) |
| `workspace/state/workflows/{name}_{instanceID}.json` | Instance state file (JSON, atomic writes) |

Persistence mechanism (`persist.go`):
- Definition files use YAML format for human readability and editing
- Instance state files are stored in `state/workflows/` directory, written atomically via `fileutil.WriteFileAtomic` to prevent corruption from interrupted writes
- The `enabled` field is a runtime state (`yaml:"-"` tag) not serialized to the YAML file; disable feature uses create/delete of `.disabled` suffix file to persist the state
- File names preserve case sensitivity; `MyWorkflow` and `myworkflow` are treated as different workflows
- File names processed through `sanitizeName()` for safety

### Workflow State Fields

The workflow definition file (YAML) supports the following runtime state fields for tracking execution history:

| Field | Type | Description |
|-------|------|-------------|
| `created_at` | string | Creation time (ISO 8601 format) |
| `updated_at` | string | Last update time (ISO 8601 format) |
| `last_run_at` | string | Last run time (ISO 8601 format), automatically updated after workflow execution completes |
| `last_run_status` | string | Last run status: `running` / `success` / `failed`, automatically updated after workflow execution completes |

These fields are automatically maintained by the system:
- `created_at` / `updated_at`: Automatically set when creating or updating a workflow
- `last_run_at` / `last_run_status`: Automatically updated based on instance status after workflow execution completes

Example:
```yaml
name: morning-briefing
description: Daily morning briefing
created_at: 2025-05-13T08:00:00Z
updated_at: 2025-05-13T10:30:00Z
last_run_at: 2025-05-13T10:00:00Z
last_run_status: success
```

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

**Notification ordering guarantee**:

To ensure notifications arrive in the correct order, the engine uses the following mechanism:
1. All notifications are sent via `msgBus.PublishOutbound`, entering the message bus worker queue to guarantee FIFO order
2. Notification messages are tagged with `message_kind=workflow_notification`, bypassing `streamActive` and `placeholder` checks in `preSend`, ensuring they are always sent as new messages
3. The `onStart` callback synchronously sends the start notification before launching async execution, ensuring the "workflow started" notification enters the message pipeline before step notifications

**Notification target priority**:

The engine determines the notification channel for each execution using this priority:

1. **Run-time channel** — When triggered from a chat context (`/workflow run`, `workflow action=run`), the current channel/chatID is passed to the instance and used for notifications
2. **Persistent binding** — When triggered without a chat context (Web UI, cron, event), the engine falls back to the channel set via `/workflow bind` or `workflow action=bind`
3. **YAML config** — If neither is available, the engine uses `notify_channels` from the workflow config

> **Note**: `/workflow bind` and `workflow action=bind` persist the channel into the workflow definition, so future cron/event-triggered runs also send notifications there. `/workflow run` and `workflow action=run` only pass the channel for that specific execution — they do NOT update the persistent binding.

Channel binding follows the same pattern as the Cron tool: `ToolChannel(ctx)` / `ToolChatID(ctx)` extract channel info from the execution context. When a workflow starts, a step begins, a step completes, or the workflow finishes, the `onStart`, `onStepStart`, `onStepComplete`, and `onComplete` callbacks send notifications to the bound channel via `msgBus.PublishOutbound`.

> Each workflow execution uses an independent session (`agent:workflow-{uuid}`), no shared context.

**YAML config**:

```yaml
config:
  notify_channels:
    - channel: telegram
      chat_id: "-100xxx"
      bound_at: "2026-05-18T10:30:00+08:00"  # Binding time (read-only, auto-recorded)
    - channel: dingtalk
      chat_id: "cidSkk33JUIC1Od8i6iLuExy/x8z5ceMX5oFLqfIL1hmqs="
      bound_at: "2026-05-18T10:35:00+08:00"
```

Each notification target contains the following fields:
- **channel**: Channel name, e.g., `telegram`, `dingtalk`
- **chat_id**: Chat ID, e.g., `-100xxx`, `cid...`
- **bound_at**: Binding time (read-only), records when the channel was bound to the workflow, helping track when notification targets were added

When a workflow is triggered without a channel context (e.g., cron trigger, event trigger, or web UI), the engine uses notification targets from the workflow config.

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

### Notify Step

The `notify` step is used to send notification messages to bound channels (e.g., Telegram) during workflow execution. It does not perform any actual computation or operations; it simply sends messages to the channel, suitable for progress reports, critical node notifications, and similar scenarios.

**Basic Usage:**

```yaml
steps:
  - id: start_notify
    name: Start Notification
    action: notify
    message: "🚀 Workflow started"

  - id: do_something
    action: tool_call
    tool: exec
    args:
      command: "echo hello"
    output_key: result

  - id: done_notify
    name: Completion Notification
    action: notify
    message: "✅ Workflow completed, result: {{.do_something.result}}"
```

**Features:**

- **Message Templates**: The `message` field supports template syntax and can reference outputs from previous steps using `{{.step_id.output_key}}`
- **Condition Control**: Supports `when` conditions for conditional notifications
- **Delayed Sending**: Supports `delay` field for delayed notifications
- **No Output**: The notify step produces no output; no need to set `output_key`

**Conditional Notification Example:**

```yaml
steps:
  - id: check_status
    action: tool_call
    tool: http_get
    args:
      url: "https://api.example.com/status"
    output_key: status

  - id: alert_if_error
    name: Error Alert
    action: notify
    when: '{{ne .check_status.status "ok"}}'
    message: "🚨 Service status abnormal: {{.check_status.status}}"
```

## Example Workflows

Five example workflows are included in `workspace/workflows/`:

| File | Description | Features Demonstrated |
|------|-------------|----------------------|
| `morning-briefing.yml` | Daily morning briefing, weekdays at 8am | Cron trigger, timezone, output_key data passing, if conditional branch |
| `server-health-check.yml` | Server health check every 10 minutes | tool_call step, retry, timeout, if conditional branch |
| `event-driven-task.yml` | Auto-analyze tool execution results | event trigger, if conditional branch, failure_strategy=continue |
| `parallel-data-fetch.yml` | Fetch multiple data sources concurrently, compile report | parallel step, if conditional branch, multi-source aggregation |
| `manual-quick-task.yml` | Manual multi-step task | No trigger (manual only), if conditional branch, simple data passing |
