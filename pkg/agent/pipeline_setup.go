// PicoClaw - Ultra-lightweight personal AI agent

package agent

import (
	"context"
	"strings"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// SetupTurn extracts the one-time initialization phase, returning a
// turnExecution populated with history, messages, and candidate selection.
// It replaces lines 56-145 of the original runTurn.
func (p *Pipeline) SetupTurn(ctx context.Context, ts *turnState) (*turnExecution, error) {
	cfg := p.Cfg
	maxMediaSize := cfg.Agents.Defaults.GetMaxMediaSize()

	var history []providers.Message
	var summary string
	if !ts.opts.NoHistory {
		if resp, err := p.ContextManager.Assemble(ctx, &AssembleRequest{
			SessionKey:     ts.sessionKey,
			Budget:         ts.agent.ContextWindow,
			MaxTokens:      ts.agent.MaxTokens,
			MaxInputTokens: ts.agent.MaxInputTokens,
		}); err == nil && resp != nil {
			history = resp.History
			summary = resp.Summary
		}
	}
	ts.captureRestorePoint(history, summary)

	contextualSkills := ts.activeSkills
	if ts.agent.ContextBuilder != nil {
		contextualSkills = ts.agent.ContextBuilder.ResolveActiveSkillsForContext(ts.activeSkills)
	}
	ts.recordSkillContextSnapshot(skillContextTriggerInitialBuild, contextualSkills)
	initialPromptReq := promptBuildRequestForTurn(ts, history, summary, ts.userMessage, ts.media)
	initialPromptReq.ActiveSkills = append([]string(nil), contextualSkills...)
	messages := ts.agent.ContextBuilder.BuildMessagesFromPrompt(initialPromptReq)

	messages = resolveMediaRefs(messages, p.MediaStore, maxMediaSize)

	if !ts.opts.NoHistory {
		var toolDefs []providers.ToolDefinition
		if !NoToolsFromCtx(ctx) {
			toolDefs = ts.agent.Tools.ToProviderDefs()
		}
		if isOverContextBudget(ts.agent.ContextWindow, messages, toolDefs, ts.agent.MaxTokens) {
			logger.WarnCF("agent", "Proactive compression: context budget exceeded before LLM call",
				map[string]any{"session_key": ts.sessionKey})
			if err := p.ContextManager.Compact(ctx, &CompactRequest{
				SessionKey: ts.sessionKey,
				Reason:     ContextCompressReasonProactive,
				Budget:     ts.agent.ContextWindow,
			}); err != nil {
				logger.WarnCF("agent", "Proactive compact failed", map[string]any{
					"session_key": ts.sessionKey,
					"error":       err.Error(),
				})
			}
			ts.refreshRestorePointFromSession(ts.agent)
			if resp, err := p.ContextManager.Assemble(ctx, &AssembleRequest{
				SessionKey:     ts.sessionKey,
				Budget:         ts.agent.ContextWindow,
				MaxTokens:      ts.agent.MaxTokens,
				MaxInputTokens: ts.agent.MaxInputTokens,
			}); err == nil && resp != nil {
				history = resp.History
				summary = resp.Summary
			}
			rebuildPromptReq := promptBuildRequestForTurn(ts, history, summary, ts.userMessage, ts.media)
			rebuildPromptReq.ActiveSkills = append([]string(nil), contextualSkills...)
			messages = ts.agent.ContextBuilder.BuildMessagesFromPrompt(rebuildPromptReq)
			messages = resolveMediaRefs(messages, p.MediaStore, maxMediaSize)

			// 压缩后二次检查：若仍超预算则强制截断历史消息
			if isOverContextBudget(ts.agent.ContextWindow, messages, toolDefs, ts.agent.MaxTokens) {
				logger.WarnCF("agent", "压缩后仍超预算，强制截断历史消息",
					map[string]any{"session_key": ts.sessionKey})
				// 仅保留 system + 最新 N 条消息以适配预算
				maxHistoryLen := len(messages) - 2 // 保留 system + user 消息
				for maxHistoryLen > 0 {
					testMsgs := append([]providers.Message{messages[0]}, messages[len(messages)-maxHistoryLen:]...)
					if !isOverContextBudget(ts.agent.ContextWindow, testMsgs, toolDefs, ts.agent.MaxTokens) {
						messages = testMsgs
						break
					}
					maxHistoryLen--
				}
			}
		}

		// 如果设置了 MaxInputTokens，额外检查并截断历史消息
		if ts.agent.MaxInputTokens > 0 {
			inputTokens := 0
			for _, msg := range messages {
				inputTokens += EstimateMessageTokens(msg)
			}
			if toolDefs != nil {
				inputTokens += EstimateToolDefsTokens(toolDefs)
			}
			if inputTokens > ts.agent.MaxInputTokens {
				logger.WarnCF("agent", "超过 MaxInputTokens 限制，正在截断历史消息",
					map[string]any{
						"session_key":      ts.sessionKey,
						"input_tokens":     inputTokens,
						"max_input_tokens": ts.agent.MaxInputTokens,
					})
				// 仅保留 system + 最新 N 条消息以适配预算
				maxHistoryLen := len(messages) - 2 // 保留 system + user 消息
				for maxHistoryLen > 0 {
					testMsgs := append([]providers.Message{messages[0]}, messages[len(messages)-maxHistoryLen:]...)
					testTokens := 0
					for _, msg := range testMsgs {
						testTokens += EstimateMessageTokens(msg)
					}
					if toolDefs != nil {
						testTokens += EstimateToolDefsTokens(toolDefs)
					}
					if testTokens <= ts.agent.MaxInputTokens {
						messages = testMsgs
						break
					}
					maxHistoryLen--
				}
			}
		}
	}

	if !ts.opts.NoHistory && (strings.TrimSpace(ts.userMessage) != "" || len(ts.media) > 0) {
		rootMsg := userPromptMessage(ts.userMessage, ts.media)
		if len(rootMsg.Media) > 0 {
			ts.agent.Sessions.AddFullMessage(ts.sessionKey, rootMsg)
		} else {
			ts.agent.Sessions.AddMessage(ts.sessionKey, rootMsg.Role, rootMsg.Content)
		}
		ts.recordPersistedMessage(rootMsg)
		ts.ingestMessage(ctx, p.al, rootMsg)
	}

	activeCandidates, activeModel, usedLight := p.al.selectCandidates(ts.agent, ts.userMessage, messages)
	activeProvider := ts.agent.Provider
	if usedLight && ts.agent.LightProvider != nil {
		activeProvider = ts.agent.LightProvider
	}
	activeModelName := strings.TrimSpace(ts.agent.Model)
	if usedLight {
		activeModelName = strings.TrimSpace(sideQuestionModelName(ts.agent, true))
	}
	activeModelName = resolvedCandidateModelName(activeCandidates, activeModelName)

	exec := newTurnExecution(
		ts.agent,
		ts.opts,
		history,
		summary,
		messages,
	)
	exec.activeCandidates = activeCandidates
	exec.activeModel = activeModel
	exec.activeModelConfig = resolveActiveModelConfig(
		p.Cfg,
		ts.agent.Workspace,
		activeCandidates,
		activeModel,
		p.Cfg.Agents.Defaults.Provider,
	)
	exec.llmModelName = activeModelName
	exec.activeProvider = activeProvider
	exec.usedLight = usedLight

	return exec, nil
}
