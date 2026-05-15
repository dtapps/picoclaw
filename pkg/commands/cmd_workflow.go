package commands

import (
	"context"
	"strings"

	"github.com/sipeed/picoclaw/pkg/i18n"
)

func workflowCommand() Definition {
	return Definition{
		Name:        "workflow",
		Description: i18n.T("commands_workflow_description"),
		SubCommands: []SubCommand{
			{
				Name:        "list",
				Description: i18n.T("commands_workflow_list_description"),
				Handler:     workflowListHandler(),
			},
			{
				Name:        "run",
				Description: i18n.T("commands_workflow_run_description"),
				ArgsUsage:   "<name>",
				Handler:     workflowRunHandler(),
			},
			{
				Name:        "show",
				Description: i18n.T("commands_workflow_show_description"),
				ArgsUsage:   "<name>",
				Handler:     workflowShowHandler(),
			},
			{
				Name:        "bind",
				Description: i18n.T("commands_workflow_bind_description"),
				ArgsUsage:   "<name>",
				Handler:     workflowBindHandler(),
			},
			{
				Name:        "unbind",
				Description: i18n.T("commands_workflow_unbind_description"),
				ArgsUsage:   "<name>",
				Handler:     workflowUnbindHandler(),
			},
			{
				Name:        "enable",
				Description: i18n.T("commands_workflow_enable_description"),
				ArgsUsage:   "<name>",
				Handler:     workflowEnableHandler(true),
			},
			{
				Name:        "disable",
				Description: i18n.T("commands_workflow_disable_description"),
				ArgsUsage:   "<name>",
				Handler:     workflowEnableHandler(false),
			},
			{
				Name:        "instances",
				Description: i18n.T("commands_workflow_instances_description"),
				ArgsUsage:   "<name>",
				Handler:     workflowInstancesHandler(),
			},
			{
				Name:        "stop",
				Description: i18n.T("commands_workflow_stop_description"),
				ArgsUsage:   "<instance-id>",
				Handler:     workflowStopHandler(),
			},
			{
				Name:        "cron",
				Description: i18n.T("commands_workflow_cron_description"),
				Handler:     workflowCronListHandler(),
			},
		},
	}
}

func workflowListHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowList == nil {
			return req.Reply(i18n.T("commands_workflow_service_unavailable"))
		}

		workflows := rt.WorkflowList()
		if len(workflows) == 0 {
			return req.Reply(i18n.T("commands_workflow_list_none"))
		}

		var sb strings.Builder
		sb.WriteString("Workflows:\n")
		for _, wf := range workflows {
			status := "enabled"
			if !wf.Enabled {
				status = "disabled"
			}
			sb.WriteString(i18n.Tf("commands_workflow_list_item", map[string]any{
				"Name":        wf.Name,
				"Status":      status,
				"TriggerType": wf.TriggerType,
				"StepCount":   wf.StepCount,
			}) + "\n")
		}
		return req.Reply(sb.String())
	}
}

func workflowRunHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowRun == nil {
			return req.Reply(i18n.T("commands_workflow_service_unavailable"))
		}

		name := nthToken(req.Text, 2) // skip "/workflow" and "run"
		if name == "" {
			return req.Reply(i18n.T("commands_workflow_run_usage"))
		}

		instanceID, err := rt.WorkflowRun(ctx, name, req.Channel, req.ChatID)
		if err != nil {
			return req.Reply(i18n.Tf("commands_workflow_run_error", map[string]any{"Error": err.Error()}))
		}

		if req.Channel != "" {
			return req.Reply(i18n.Tf("commands_workflow_run_success_with_channel", map[string]any{
				"Name":       name,
				"InstanceID": instanceID,
				"Channel":    req.Channel,
			}))
		}
		return req.Reply(i18n.Tf("commands_workflow_run_success", map[string]any{
			"Name":       name,
			"InstanceID": instanceID,
		}))
	}
}

func workflowShowHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowShow == nil {
			return req.Reply(i18n.T("commands_workflow_service_unavailable"))
		}

		name := nthToken(req.Text, 2)
		if name == "" {
			return req.Reply(i18n.T("commands_workflow_show_usage"))
		}

		info, steps, err := rt.WorkflowShow(name)
		if err != nil {
			return req.Reply(i18n.Tf("commands_workflow_show_error", map[string]any{"Error": err.Error()}))
		}

		var sb strings.Builder
		sb.WriteString(i18n.Tf("commands_workflow_show_header", map[string]any{
			"Name":         info.Name,
			"Description":  info.Description,
			"Enabled":      info.Enabled,
			"TriggerCount": len(info.Triggers),
		}) + "\n")
		for _, t := range info.Triggers {
			sb.WriteString(i18n.Tf("commands_workflow_show_trigger_item", map[string]any{"Trigger": t}) + "\n")
		}
		sb.WriteString(i18n.Tf("commands_workflow_show_steps_header", map[string]any{"StepCount": len(steps)}) + "\n")
		for i, stepID := range steps {
			sb.WriteString(i18n.Tf("commands_workflow_show_step_item", map[string]any{
				"Index":  i + 1,
				"StepID": stepID,
			}) + "\n")
		}
		return req.Reply(sb.String())
	}
}

func workflowBindHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowBind == nil {
			return req.Reply(i18n.T("commands_workflow_service_unavailable"))
		}

		name := nthToken(req.Text, 2)
		if name == "" {
			return req.Reply(i18n.T("commands_workflow_bind_usage"))
		}

		if req.Channel == "" || req.ChatID == "" {
			return req.Reply(i18n.T("commands_workflow_bind_no_context"))
		}

		if err := rt.WorkflowBind(name, req.Channel, req.ChatID); err != nil {
			return req.Reply(i18n.Tf("commands_workflow_bind_error", map[string]any{"Error": err.Error()}))
		}

		return req.Reply(i18n.Tf("commands_workflow_bind_success", map[string]any{
			"Name":    name,
			"Channel": req.Channel,
			"ChatID":  req.ChatID,
		}))
	}
}

func workflowUnbindHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowUnbind == nil {
			return req.Reply(i18n.T("commands_workflow_service_unavailable"))
		}

		name := nthToken(req.Text, 2)
		if name == "" {
			return req.Reply(i18n.T("commands_workflow_unbind_usage"))
		}

		if err := rt.WorkflowUnbind(name); err != nil {
			return req.Reply(i18n.Tf("commands_workflow_unbind_error", map[string]any{"Error": err.Error()}))
		}

		return req.Reply(i18n.Tf("commands_workflow_unbind_success", map[string]any{"Name": name}))
	}
}

func workflowEnableHandler(enabled bool) Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowEnable == nil {
			return req.Reply(i18n.T("commands_workflow_service_unavailable"))
		}

		name := nthToken(req.Text, 2)
		if name == "" {
			if enabled {
				return req.Reply(i18n.T("commands_workflow_enable_usage"))
			}
			return req.Reply(i18n.T("commands_workflow_disable_usage"))
		}

		if err := rt.WorkflowEnable(name, enabled); err != nil {
			return req.Reply(i18n.Tf("commands_workflow_enable_error", map[string]any{"Error": err.Error()}))
		}

		if enabled {
			return req.Reply(i18n.Tf("commands_workflow_enable_success", map[string]any{"Name": name}))
		}
		return req.Reply(i18n.Tf("commands_workflow_disable_success", map[string]any{"Name": name}))
	}
}

func workflowInstancesHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowInstances == nil {
			return req.Reply(i18n.T("commands_workflow_service_unavailable"))
		}

		name := nthToken(req.Text, 2)
		if name == "" {
			return req.Reply(i18n.T("commands_workflow_instances_usage"))
		}

		instances, err := rt.WorkflowInstances(name)
		if err != nil {
			return req.Reply(i18n.Tf("commands_workflow_instances_error", map[string]any{"Error": err.Error()}))
		}

		if len(instances) == 0 {
			return req.Reply(i18n.Tf("commands_workflow_instances_none", map[string]any{"Name": name}))
		}

		var sb strings.Builder
		sb.WriteString(i18n.Tf("commands_workflow_instances_header", map[string]any{"Name": name}) + "\n")
		for _, inst := range instances {
			sb.WriteString(i18n.Tf("commands_workflow_instances_item", map[string]any{
				"ID":          inst.ID,
				"Status":      inst.Status,
				"TriggerType": inst.TriggerType,
				"StartedAt":   inst.StartedAt,
			}) + "\n")
			if inst.Error != "" {
				sb.WriteString(
					i18n.Tf("commands_workflow_instances_error_item", map[string]any{"Error": inst.Error}) + "\n",
				)
			}
		}
		return req.Reply(sb.String())
	}
}

func workflowStopHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowStop == nil {
			return req.Reply(i18n.T("commands_workflow_service_unavailable"))
		}

		instanceID := nthToken(req.Text, 2)
		if instanceID == "" {
			return req.Reply(i18n.T("commands_workflow_stop_usage"))
		}

		if err := rt.WorkflowStop(instanceID); err != nil {
			return req.Reply(i18n.Tf("commands_workflow_stop_error", map[string]any{"Error": err.Error()}))
		}

		return req.Reply(i18n.Tf("commands_workflow_stop_success", map[string]any{"InstanceID": instanceID}))
	}
}

func workflowCronListHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		if rt == nil || rt.WorkflowCronList == nil {
			return req.Reply(i18n.T("commands_workflow_service_unavailable"))
		}

		tasks := rt.WorkflowCronList()
		if len(tasks) == 0 {
			return req.Reply(i18n.T("commands_workflow_cron_none"))
		}

		var sb strings.Builder
		sb.WriteString(i18n.Tf("commands_workflow_cron_header", map[string]any{"Count": len(tasks)}) + "\n")
		for _, t := range tasks {
			sb.WriteString(i18n.Tf("commands_workflow_cron_item", map[string]any{
				"WorkflowName": t.WorkflowName,
				"TriggerType":  t.TriggerType,
				"Schedule":     t.Schedule,
				"Timezone":     t.Timezone,
				"NextRun":      t.NextRun,
			}) + "\n")
		}
		return req.Reply(sb.String())
	}
}
