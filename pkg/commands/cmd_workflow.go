package commands

import (
	"context"
	"fmt"
	"strings"
)

func workflowCommand() Definition {
	return Definition{
		Name:        "workflow",
		Description: "Manage declarative workflows: run, bind channels, list, and inspect",
		SubCommands: []SubCommand{
			{
				Name:        "list",
				Description: "List all workflows",
				Handler:     workflowListHandler(),
			},
			{
				Name:        "run",
				Description: "Trigger a workflow by name",
				ArgsUsage:   "<name>",
				Handler:     workflowRunHandler(),
			},
			{
				Name:        "show",
				Description: "Show workflow details",
				ArgsUsage:   "<name>",
				Handler:     workflowShowHandler(),
			},
			{
				Name:        "bind",
				Description: "Bind the current channel for completion notifications",
				ArgsUsage:   "<name>",
				Handler:     workflowBindHandler(),
			},
			{
				Name:        "unbind",
				Description: "Remove channel binding from a workflow",
				ArgsUsage:   "<name>",
				Handler:     workflowUnbindHandler(),
			},
			{
				Name:        "enable",
				Description: "Enable a workflow",
				ArgsUsage:   "<name>",
				Handler:     workflowEnableHandler(true),
			},
			{
				Name:        "disable",
				Description: "Disable a workflow",
				ArgsUsage:   "<name>",
				Handler:     workflowEnableHandler(false),
			},
			{
				Name:        "instances",
				Description: "Show execution history for a workflow",
				ArgsUsage:   "<name>",
				Handler:     workflowInstancesHandler(),
			},
			{
				Name:        "stop",
				Description: "Stop a running workflow instance",
				ArgsUsage:   "<instance-id>",
				Handler:     workflowStopHandler(),
			},
			{
				Name:        "cron",
				Description: "List upcoming cron tasks",
				Handler:     workflowCronListHandler(),
			},
		},
	}
}

func workflowListHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowList == nil {
			return req.Reply("Workflow service not available")
		}

		workflows := rt.WorkflowList()
		if len(workflows) == 0 {
			return req.Reply("No workflows defined")
		}

		var sb strings.Builder
		sb.WriteString("Workflows:\n")
		for _, wf := range workflows {
			status := "enabled"
			if !wf.Enabled {
				status = "disabled"
			}
			sb.WriteString(fmt.Sprintf("- %s (%s, %s, %d steps)\n",
				wf.Name, status, wf.TriggerType, wf.StepCount))
		}
		return req.Reply(sb.String())
	}
}

func workflowRunHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowRun == nil {
			return req.Reply("Workflow service not available")
		}

		name := nthToken(req.Text, 2) // skip "/workflow" and "run"
		if name == "" {
			return req.Reply("Usage: /workflow run <name>")
		}

		instanceID, err := rt.WorkflowRun(ctx, name, req.Channel, req.ChatID)
		if err != nil {
			return req.Reply(fmt.Sprintf("Error: %v", err))
		}

		msg := fmt.Sprintf("Workflow '%s' triggered, instance: %s", name, instanceID)
		if req.Channel != "" {
			msg += fmt.Sprintf(" (notifications → %s)", req.Channel)
		}
		return req.Reply(msg)
	}
}

func workflowShowHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowShow == nil {
			return req.Reply("Workflow service not available")
		}

		name := nthToken(req.Text, 2)
		if name == "" {
			return req.Reply("Usage: /workflow show <name>")
		}

		info, steps, err := rt.WorkflowShow(name)
		if err != nil {
			return req.Reply(fmt.Sprintf("Error: %v", err))
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Workflow: %s\n", info.Name))
		sb.WriteString(fmt.Sprintf("Description: %s\n", info.Description))
		sb.WriteString(fmt.Sprintf("Enabled: %v\n", info.Enabled))
		sb.WriteString(fmt.Sprintf("Triggers (%d):\n", len(info.Triggers)))
		for _, t := range info.Triggers {
			sb.WriteString(fmt.Sprintf("  %s\n", t))
		}
		sb.WriteString(fmt.Sprintf("Steps (%d):\n", len(steps)))
		for i, stepID := range steps {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, stepID))
		}
		return req.Reply(sb.String())
	}
}

func workflowBindHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowBind == nil {
			return req.Reply("Workflow service not available")
		}

		name := nthToken(req.Text, 2)
		if name == "" {
			return req.Reply("Usage: /workflow bind <name>")
		}

		if req.Channel == "" || req.ChatID == "" {
			return req.Reply("Error: no session context (channel/chat_id not set)")
		}

		if err := rt.WorkflowBind(name, req.Channel, req.ChatID); err != nil {
			return req.Reply(fmt.Sprintf("Error: %v", err))
		}

		return req.Reply(
			fmt.Sprintf("Workflow '%s' bound to channel %s (chat_id: %s). Completion notifications will be sent here.",
				name, req.Channel, req.ChatID),
		)
	}
}

func workflowUnbindHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowUnbind == nil {
			return req.Reply("Workflow service not available")
		}

		name := nthToken(req.Text, 2)
		if name == "" {
			return req.Reply("Usage: /workflow unbind <name>")
		}

		if err := rt.WorkflowUnbind(name); err != nil {
			return req.Reply(fmt.Sprintf("Error: %v", err))
		}

		return req.Reply(fmt.Sprintf("Workflow '%s' channel binding removed", name))
	}
}

func workflowEnableHandler(enabled bool) Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowEnable == nil {
			return req.Reply("Workflow service not available")
		}

		name := nthToken(req.Text, 2)
		if name == "" {
			action := "enable"
			if !enabled {
				action = "disable"
			}
			return req.Reply(fmt.Sprintf("Usage: /workflow %s <name>", action))
		}

		if err := rt.WorkflowEnable(name, enabled); err != nil {
			return req.Reply(fmt.Sprintf("Error: %v", err))
		}

		status := "enabled"
		if !enabled {
			status = "disabled"
		}
		return req.Reply(fmt.Sprintf("Workflow '%s' %s", name, status))
	}
}

func workflowInstancesHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowInstances == nil {
			return req.Reply("Workflow service not available")
		}

		name := nthToken(req.Text, 2)
		if name == "" {
			return req.Reply("Usage: /workflow instances <name>")
		}

		instances, err := rt.WorkflowInstances(name)
		if err != nil {
			return req.Reply(fmt.Sprintf("Error: %v", err))
		}

		if len(instances) == 0 {
			return req.Reply(fmt.Sprintf("No instances for workflow '%s'", name))
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Instances for '%s':\n", name))
		for _, inst := range instances {
			sb.WriteString(fmt.Sprintf("- %s (%s, %s, started: %s)\n",
				inst.ID, inst.Status, inst.TriggerType, inst.StartedAt))
			if inst.Error != "" {
				sb.WriteString(fmt.Sprintf("  error: %s\n", inst.Error))
			}
		}
		return req.Reply(sb.String())
	}
}

func workflowStopHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowStop == nil {
			return req.Reply("Workflow service not available")
		}

		instanceID := nthToken(req.Text, 2)
		if instanceID == "" {
			return req.Reply("Usage: /workflow stop <instance-id>")
		}

		if err := rt.WorkflowStop(instanceID); err != nil {
			return req.Reply(fmt.Sprintf("Error: %v", err))
		}

		return req.Reply(fmt.Sprintf("Instance '%s' stopped", instanceID))
	}
}

func workflowCronListHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowCronList == nil {
			return req.Reply("Workflow service not available")
		}

		tasks := rt.WorkflowCronList()
		if len(tasks) == 0 {
			return req.Reply("No cron tasks scheduled")
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Upcoming Scheduled Tasks (%d):\n", len(tasks)))
		for _, t := range tasks {
			sb.WriteString(fmt.Sprintf("- %s [%s]: %s (tz: %s) → %s\n",
				t.WorkflowName, t.TriggerType, t.Schedule, t.Timezone, t.NextRun))
		}
		return req.Reply(sb.String())
	}
}
