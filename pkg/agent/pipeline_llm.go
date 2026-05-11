// PicoClaw - Ultra-lightweight personal AI agent

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/constants"
	runtimeevents "github.com/sipeed/picoclaw/pkg/events"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/providers/tracing"
)

// CallLLM performs an LLM call with fallback support, hook invocation, and retry logic.
// It handles PreLLM setup, the actual LLM invocation with retry, and AfterLLM processing.
// Returns Control indicating what the coordinator should do next.
func (p *Pipeline) CallLLM(
	ctx context.Context,
	turnCtx context.Context,
	ts *turnState,
	exec *turnExecution,
	iteration int,
) (Control, error) {
	al := p.al
	maxMediaSize := p.Cfg.Agents.Defaults.GetMaxMediaSize()

	// PreLLM: resolve media refs (except on iteration 1 where user media is already resolved)
	if iteration > 1 {
		exec.messages = resolveMediaRefs(exec.messages, p.MediaStore, maxMediaSize)
	}

	// PreLLM: graceful terminal handling
	exec.gracefulTerminal, _ = ts.gracefulInterruptRequested()
	exec.providerToolDefs = ts.agent.Tools.ToProviderDefs()

	// Native web search support
	webSearchEnabled := al.cfg.Tools.IsToolEnabled("web")
	exec.useNativeSearch = webSearchEnabled && al.cfg.Tools.Web.PreferNative &&
		func() bool {
			if ns, ok := ts.agent.Provider.(providers.NativeSearchCapable); ok {
				return ns.SupportsNativeSearch()
			}
			return false
		}()

	if exec.useNativeSearch {
		filtered := make([]providers.ToolDefinition, 0, len(exec.providerToolDefs))
		for _, td := range exec.providerToolDefs {
			if td.Function.Name != "web_search" {
				filtered = append(filtered, td)
			}
		}
		exec.providerToolDefs = filtered
	}

	exec.callMessages = exec.messages
	if exec.gracefulTerminal {
		exec.callMessages = append(append([]providers.Message(nil), exec.messages...), ts.interruptHintMessage())
		exec.providerToolDefs = nil
		ts.markGracefulTerminalUsed()
	}

	exec.llmOpts = map[string]any{
		"max_tokens":       ts.agent.MaxTokens,
		"temperature":      ts.agent.Temperature,
		"prompt_cache_key": ts.agent.ID,
	}
	if exec.useNativeSearch {
		exec.llmOpts["native_search"] = true
	}
	if ts.agent.ThinkingLevel != ThinkingOff {
		if tc, ok := ts.agent.Provider.(providers.ThinkingCapable); ok && tc.SupportsThinking() {
			exec.llmOpts["thinking_level"] = string(ts.agent.ThinkingLevel)
		} else {
			logger.WarnCF("agent", "thinking_level is set but current provider does not support it, ignoring",
				map[string]any{"agent_id": ts.agent.ID, "thinking_level": string(ts.agent.ThinkingLevel)})
		}
	}

	exec.llmModel = exec.activeModel

	// BeforeLLM hook
	if p.Hooks != nil {
		llmReq, decision := p.Hooks.BeforeLLM(turnCtx, &LLMHookRequest{
			Meta:             ts.eventMeta("runTurn", "turn.llm.request"),
			Context:          cloneTurnContext(ts.turnCtx),
			Model:            exec.llmModel,
			Messages:         exec.callMessages,
			Tools:            exec.providerToolDefs,
			Options:          exec.llmOpts,
			GracefulTerminal: exec.gracefulTerminal,
		})
		switch decision.normalizedAction() {
		case HookActionContinue, HookActionModify:
			if llmReq != nil {
				exec.llmModel = llmReq.Model
				exec.callMessages = llmReq.Messages
				exec.providerToolDefs = llmReq.Tools
				exec.llmOpts = llmReq.Options
			}
		case HookActionAbortTurn:
			exec.abortedByHook = true
			return ControlBreak, nil
		case HookActionHardAbort:
			_ = ts.requestHardAbort()
			exec.abortedByHardAbort = true
			return ControlBreak, nil
		}
	}

	al.emitEvent(
		runtimeevents.KindAgentLLMRequest,
		ts.eventMeta("runTurn", "turn.llm.request"),
		LLMRequestPayload{
			Model:         exec.llmModel,
			MessagesCount: len(exec.callMessages),
			ToolsCount:    len(exec.providerToolDefs),
			MaxTokens:     ts.agent.MaxTokens,
			Temperature:   ts.agent.Temperature,
		},
	)

	logger.DebugCF("agent", "LLM request",
		map[string]any{
			"agent_id":          ts.agent.ID,
			"iteration":         iteration,
			"model":             exec.llmModel,
			"messages_count":    len(exec.callMessages),
			"tools_count":       len(exec.providerToolDefs),
			"max_tokens":        ts.agent.MaxTokens,
			"temperature":       ts.agent.Temperature,
			"system_prompt_len": len(exec.callMessages[0].Content),
		})
	logger.DebugCF("agent", "Full LLM request",
		map[string]any{
			"iteration":     iteration,
			"messages_json": formatMessagesForLog(exec.callMessages),
			"tools_json":    formatToolsForLog(exec.providerToolDefs),
		})

	// LLM call closure with fallback support
	callLLM := func(messagesForCall []providers.Message, toolDefsForCall []providers.ToolDefinition) (*providers.LLMResponse, error) {
		providerCtx, providerCancel := context.WithCancel(turnCtx)
		ts.setProviderCancel(providerCancel)
		defer func() {
			providerCancel()
			ts.clearProviderCancel(providerCancel)
		}()

		// 注入追踪信息到 context，供 provider 层读取并设置 HTTP 请求头
		if p.Cfg != nil && p.Cfg.Tracing.IsEnabled() {
			providerCtx = tracing.WithHeaders(providerCtx, p.Cfg.Tracing.Headers)
			providerCtx = tracing.WithSessionKey(providerCtx, ts.sessionKey)
			providerCtx = tracing.WithTurnID(providerCtx, ts.turnID)
			providerCtx = tracing.WithAgentID(providerCtx, ts.agentID)
			providerCtx = tracing.WithAgentName(providerCtx, ts.agent.Name)
			providerCtx = tracing.WithChannel(providerCtx, ts.channel)
			providerCtx = tracing.WithChatID(providerCtx, ts.chatID)
			providerCtx = tracing.WithParentTurnID(providerCtx, ts.parentTurnID)
			providerCtx = tracing.WithSenderID(providerCtx, ts.opts.SenderID)
			providerCtx = tracing.WithMessageID(providerCtx, ts.opts.MessageID)
			providerCtx = tracing.WithModel(providerCtx, exec.llmModel)
		}

		al.activeRequests.Add(1)
		defer al.activeRequests.Done()

		if len(exec.activeCandidates) > 1 && p.Fallback != nil {
			fbResult, fbErr := p.Fallback.Execute(
				providerCtx,
				exec.activeCandidates,
				func(ctx context.Context, provider, model string) (*providers.LLMResponse, error) {
					candidateProvider := exec.activeProvider
					if cp, ok := ts.agent.CandidateProviders[providers.ModelKey(provider, model)]; ok {
						candidateProvider = cp
					}
					return candidateProvider.Chat(ctx, messagesForCall, toolDefsForCall, model, exec.llmOpts)
				},
			)
			if fbErr != nil {
				return nil, fbErr
			}
			if fbResult.Provider != "" && len(fbResult.Attempts) > 0 {
				logger.InfoCF(
					"agent",
					fmt.Sprintf("Fallback: succeeded with %s/%s after %d attempts",
						fbResult.Provider, fbResult.Model, len(fbResult.Attempts)+1),
					map[string]any{"agent_id": ts.agent.ID, "iteration": iteration},
				)
			}
			return fbResult.Response, nil
		}
		return exec.activeProvider.Chat(providerCtx, messagesForCall, toolDefsForCall, exec.llmModel, exec.llmOpts)
	}

	// Retry loop
	var err error
	maxRetries := p.Cfg.Agents.Defaults.MaxLLMRetries
	if maxRetries <= 0 {
		maxRetries = 2
	}
	backoffSecs := p.Cfg.Agents.Defaults.LLMRetryBackoffSecs
	if backoffSecs <= 0 {
		backoffSecs = 2
	}
	for retry := 0; retry <= maxRetries; retry++ {
		exec.response, err = callLLM(exec.callMessages, exec.providerToolDefs)
		if err == nil {
			break
		}
		if ts.hardAbortRequested() && errors.Is(err, context.Canceled) {
			_ = ts.requestHardAbort()
			exec.abortedByHardAbort = true
			return ControlBreak, nil
		}

		// Retry without media if vision is unsupported
		if hasMediaRefs(exec.callMessages) && isVisionUnsupportedError(err) && retry < maxRetries {
			al.emitEvent(
				runtimeevents.KindAgentLLMRetry,
				ts.eventMeta("runTurn", "turn.llm.retry"),
				LLMRetryPayload{
					Attempt:    retry + 1,
					MaxRetries: maxRetries,
					Reason:     "vision_unsupported",
					Error:      err.Error(),
					Backoff:    0,
				},
			)
			logger.WarnCF("agent", "Vision unsupported, retrying without media", map[string]any{
				"error": err.Error(),
				"retry": retry,
			})
			exec.callMessages = stripMessageMedia(exec.callMessages)
			if !ts.opts.NoHistory {
				exec.history = stripMessageMedia(exec.history)
				ts.agent.Sessions.SetHistory(ts.sessionKey, exec.history)
				for i := range ts.persistedMessages {
					ts.persistedMessages[i].Media = nil
				}
				ts.refreshRestorePointFromSession(ts.agent)
			}
			continue
		}

		errMsg := strings.ToLower(err.Error())
		isTimeoutError := errors.Is(err, context.DeadlineExceeded) ||
			strings.Contains(errMsg, "deadline exceeded") ||
			strings.Contains(errMsg, "client.timeout") ||
			strings.Contains(errMsg, "timed out") ||
			strings.Contains(errMsg, "timeout exceeded")

		isNetworkError := !isTimeoutError && (strings.Contains(errMsg, "connection reset") ||
			strings.Contains(errMsg, "connection refused") ||
			strings.Contains(errMsg, "broken pipe") ||
			strings.Contains(errMsg, "no such host") ||
			strings.Contains(errMsg, "network is unreachable") ||
			strings.Contains(errMsg, "read tcp") ||
			strings.Contains(errMsg, "write tcp") ||
			strings.Contains(errMsg, "eof"))

		isContextError := !isTimeoutError && (strings.Contains(errMsg, "context_length_exceeded") ||
			strings.Contains(errMsg, "context window") ||
			strings.Contains(errMsg, "context_window") ||
			strings.Contains(errMsg, "maximum context length") ||
			strings.Contains(errMsg, "token limit") ||
			strings.Contains(errMsg, "too many tokens") ||
			strings.Contains(errMsg, "max_tokens") ||
			strings.Contains(errMsg, "invalidparameter") ||
			strings.Contains(errMsg, "prompt is too long") ||
			strings.Contains(errMsg, "request too large"))

		if isTimeoutError && retry < maxRetries {
			backoff := time.Duration(retry+1) * time.Duration(backoffSecs) * time.Second
			al.emitEvent(
				runtimeevents.KindAgentLLMRetry,
				ts.eventMeta("runTurn", "turn.llm.retry"),
				LLMRetryPayload{
					Attempt:    retry + 1,
					MaxRetries: maxRetries,
					Reason:     "timeout",
					Error:      err.Error(),
					Backoff:    backoff,
				},
			)
			logger.WarnCF("agent", "Timeout error, retrying after backoff", map[string]any{
				"error":   err.Error(),
				"retry":   retry,
				"backoff": backoff.String(),
			})
			if sleepErr := sleepWithContext(turnCtx, backoff); sleepErr != nil {
				if ts.hardAbortRequested() {
					_ = ts.requestHardAbort()
					return ControlBreak, nil
				}
				err = sleepErr
				break
			}
			continue
		}

		if isNetworkError && retry < maxRetries {
			backoff := time.Duration(retry+1) * time.Duration(backoffSecs) * time.Second
			al.emitEvent(
				runtimeevents.KindAgentLLMRetry,
				ts.eventMeta("runTurn", "turn.llm.retry"),
				LLMRetryPayload{
					Attempt:    retry + 1,
					MaxRetries: maxRetries,
					Reason:     "network",
					Error:      err.Error(),
					Backoff:    backoff,
				},
			)
			logger.WarnCF("agent", "Network error, retrying after backoff", map[string]any{
				"error":   err.Error(),
				"retry":   retry,
				"backoff": backoff.String(),
			})
			if sleepErr := sleepWithContext(turnCtx, backoff); sleepErr != nil {
				if ts.hardAbortRequested() {
					_ = ts.requestHardAbort()
					return ControlBreak, nil
				}
				err = sleepErr
				break
			}
			continue
		}

		if isContextError && retry < maxRetries && !ts.opts.NoHistory {
			al.emitEvent(
				runtimeevents.KindAgentLLMRetry,
				ts.eventMeta("runTurn", "turn.llm.retry"),
				LLMRetryPayload{
					Attempt:    retry + 1,
					MaxRetries: maxRetries,
					Reason:     "context_limit",
					Error:      err.Error(),
				},
			)
			logger.WarnCF(
				"agent",
				"Context window error detected, attempting compression",
				map[string]any{
					"error": err.Error(),
					"retry": retry,
				},
			)

			if retry == 0 && !constants.IsInternalChannel(ts.channel) {
				al.bus.PublishOutbound(ctx, outboundMessageForTurn(
					ts,
					"Context window exceeded. Compressing history and retrying...",
				))
			}

			if compactErr := p.ContextManager.Compact(ctx, &CompactRequest{
				SessionKey: ts.sessionKey,
				Reason:     ContextCompressReasonRetry,
				Budget:     ts.agent.ContextWindow,
			}); compactErr != nil {
				logger.WarnCF("agent", "Context overflow compact failed", map[string]any{
					"session_key": ts.sessionKey,
					"error":       compactErr.Error(),
				})
			}
			ts.refreshRestorePointFromSession(ts.agent)
			if asmResp, asmErr := p.ContextManager.Assemble(ctx, &AssembleRequest{
				SessionKey: ts.sessionKey,
				Budget:     ts.agent.ContextWindow,
				MaxTokens:  ts.agent.MaxTokens,
			}); asmErr == nil && asmResp != nil {
				exec.history = asmResp.History
				exec.summary = asmResp.Summary
			}
			exec.messages = ts.agent.ContextBuilder.BuildMessagesFromPrompt(
				promptBuildRequestForTurn(ts, exec.history, exec.summary, "", nil),
			)
			exec.callMessages = exec.messages
			if exec.gracefulTerminal {
				msgs := append([]providers.Message(nil), exec.messages...)
				exec.callMessages = append(msgs, ts.interruptHintMessage())
			}
			continue
		}
		break
	}

	if err != nil {
		al.emitEvent(
			runtimeevents.KindAgentError,
			ts.eventMeta("runTurn", "turn.error"),
			ErrorPayload{
				Stage:   "llm",
				Message: err.Error(),
			},
		)
		logger.ErrorCF("agent", "LLM call failed",
			map[string]any{
				"agent_id":  ts.agent.ID,
				"iteration": iteration,
				"model":     exec.llmModel,
				"error":     err.Error(),
			})
		return ControlBreak, fmt.Errorf("LLM call failed after retries: %w", err)
	}

	// 内联工具调用提取逻辑
	// 当 LLM 返回的响应没有标准 tool_calls 但 content 中包含内联工具调用模式时，
	// 自动提取并转换为标准格式。典型场景：kimi-k2 将工具调用写在 content 文本中
	if p.Cfg != nil && p.Cfg.Agents.Defaults.IsInlineToolCallsEnabled() {
		toolCallsBefore := len(exec.response.ToolCalls)
		contentBefore := exec.response.Content
		providers.NormalizeInlineToolCalls(exec.response)
		toolCallsAfter := len(exec.response.ToolCalls)
		if toolCallsAfter > toolCallsBefore {
			logger.InfoCF("agent", "内联工具调用提取成功",
				map[string]any{
					"agent_id":          ts.agent.ID,
					"iteration":         iteration,
					"tool_calls_before": toolCallsBefore,
					"tool_calls_after":  toolCallsAfter,
					"original_content":  contentBefore,
					"cleaned_content":   exec.response.Content,
				})
		} else if strings.Contains(contentBefore, "[tool_use: ") {
			logger.WarnCF("agent", "内联工具调用提取失败：检测到 [tool_use: 标记但未能提取",
				map[string]any{
					"agent_id":  ts.agent.ID,
					"iteration": iteration,
					"content":   contentBefore,
				})
		}
	} else if len(exec.response.ToolCalls) == 0 && strings.Contains(exec.response.Content, "[tool_use: ") {
		logger.DebugCF("agent", "检测到内联工具调用但功能未启用",
			map[string]any{
				"agent_id": ts.agent.ID,
				"hint":     "请启用 agents.defaults.inline_tool_calls.enabled 配置",
			})
	}

	// 空响应自动重试逻辑
	// 当 LLM 返回的响应没有 tool calls 且内容匹配空响应模式时，自动重新请求
	// 典型场景：kimi-k2 返回 [{'type': 'text', 'text': ''}] 这种格式异常的响应
	if p.Cfg != nil && p.Cfg.Agents.Defaults.IsEmptyResponseRetryEnabled() {
		patterns := p.Cfg.Agents.Defaults.GetEmptyResponsePatterns()
		maxRetries := p.Cfg.Agents.Defaults.GetEmptyResponseMaxRetries()
		// 条件：无 tool calls 且内容匹配空响应模式
		if len(exec.response.ToolCalls) == 0 && matchesEmptyResponsePattern(exec.response.Content, patterns) {
			logger.WarnCF("agent", "LLM 返回空响应或格式异常，正在重试",
				map[string]any{
					"agent_id":    ts.agent.ID,
					"iteration":   iteration,
					"content":     exec.response.Content,
					"max_retries": maxRetries,
				})

			for retry := 0; retry < maxRetries; retry++ {
				// 发送重试事件，用于前端展示和日志追踪
				al.emitEvent(
					runtimeevents.KindAgentLLMRetry,
					ts.eventMeta("runTurn", "turn.llm.retry"),
					LLMRetryPayload{
						Attempt:    retry + 1,
						MaxRetries: maxRetries,
						Reason:     "empty_response",
						Error:      fmt.Sprintf("响应内容匹配空响应模式: %q", exec.response.Content),
					},
				)

				retryResp, retryErr := callLLM(exec.callMessages, exec.providerToolDefs)
				if retryErr != nil {
					// 重试请求失败，继续下一次重试
					logger.WarnCF("agent", "空响应重试请求失败",
						map[string]any{
							"agent_id": ts.agent.ID,
							"retry":    retry + 1,
							"error":    retryErr.Error(),
						})
					continue
				}

				// 重试成功条件：有 tool calls，或内容不再匹配空响应模式
				if len(retryResp.ToolCalls) > 0 || !matchesEmptyResponsePattern(retryResp.Content, patterns) {
					exec.response = retryResp
					logger.InfoCF("agent", "空响应重试成功",
						map[string]any{
							"agent_id":      ts.agent.ID,
							"retry":         retry + 1,
							"content_chars": len(retryResp.Content),
						})
					break
				}

				logger.WarnCF("agent", "空响应重试后仍返回空响应",
					map[string]any{
						"agent_id": ts.agent.ID,
						"retry":    retry + 1,
						"content":  retryResp.Content,
					})
			}
		}
	} else if p.Cfg != nil && len(exec.response.ToolCalls) == 0 && matchesEmptyResponsePattern(exec.response.Content, p.Cfg.Agents.Defaults.GetEmptyResponsePatterns()) {
		logger.DebugCF("agent", "检测到空响应但功能未启用",
			map[string]any{
				"agent_id": ts.agent.ID,
				"hint":     "请启用 agents.defaults.empty_response_retry.enabled 配置",
			})
	}

	// 模型响应内容清理逻辑
	// 在空响应重试之后执行，确保最终 content 不包含 Anthropic 风格包装和特殊 token
	// 典型场景：kimi-k2 返回 [{'type': 'text', 'text': '好，我重新来一遍！'}<|tool_call_end|><|tool_calls_section_end|>
	if p.Cfg != nil && p.Cfg.Agents.Defaults.IsCleanContentEnabled() {
		cleaned := providers.CleanModelContent(exec.response.Content)
		if cleaned != exec.response.Content {
			logger.InfoCF("agent", "清理模型响应内容",
				map[string]any{
					"agent_id":         ts.agent.ID,
					"original_content": exec.response.Content,
					"cleaned_content":  cleaned,
				})
			exec.response.Content = cleaned
		}
	} else if p.Cfg != nil && needsCleanContent(exec.response.Content) {
		logger.DebugCF("agent", "检测到需要清理的响应内容但功能未启用",
			map[string]any{
				"agent_id": ts.agent.ID,
				"hint":     "请启用 agents.defaults.inline_tool_calls.clean_content 配置",
			})
	}

	// AfterLLM hook
	if p.Hooks != nil {
		llmResp, decision := p.Hooks.AfterLLM(turnCtx, &LLMHookResponse{
			Meta:     ts.eventMeta("runTurn", "turn.llm.response"),
			Context:  cloneTurnContext(ts.turnCtx),
			Model:    exec.llmModel,
			Response: exec.response,
		})
		switch decision.normalizedAction() {
		case HookActionContinue, HookActionModify:
			if llmResp != nil && llmResp.Response != nil {
				exec.response = llmResp.Response
			}
		case HookActionAbortTurn:
			exec.abortedByHook = true
			return ControlBreak, nil
		case HookActionHardAbort:
			_ = ts.requestHardAbort()
			exec.abortedByHardAbort = true
			return ControlBreak, nil
		}
	}

	// Save finishReason to turnState for SubTurn truncation detection
	if innerTS := turnStateFromContext(ctx); innerTS != nil {
		innerTS.SetLastFinishReason(exec.response.FinishReason)
		if exec.response.Usage != nil {
			innerTS.SetLastUsage(exec.response.Usage)
		}
	}

	reasoningContent := responseReasoningContent(exec.response)
	shouldPublishPicoToolCallInterim := ts.channel == "pico" && len(exec.response.ToolCalls) > 0
	if shouldPublishPicoToolCallInterim {
		// Pico tool-call turns publish their reasoning/content/tool summary as a
		// structured sequence after the tool-call payload is normalized below.
	} else if ts.channel == "pico" {
		go al.publishPicoReasoning(turnCtx, reasoningContent, ts.chatID)
	} else {
		go al.handleReasoning(
			turnCtx,
			reasoningContent,
			ts.channel,
			al.targetReasoningChannelID(ts.channel),
		)
	}
	al.emitEvent(
		runtimeevents.KindAgentLLMResponse,
		ts.eventMeta("runTurn", "turn.llm.response"),
		LLMResponsePayload{
			ContentLen:   len(exec.response.Content),
			ToolCalls:    len(exec.response.ToolCalls),
			HasReasoning: exec.response.Reasoning != "" || exec.response.ReasoningContent != "",
		},
	)

	llmResponseFields := map[string]any{
		"agent_id":       ts.agent.ID,
		"iteration":      iteration,
		"content_chars":  len(exec.response.Content),
		"tool_calls":     len(exec.response.ToolCalls),
		"reasoning":      exec.response.Reasoning,
		"target_channel": al.targetReasoningChannelID(ts.channel),
		"channel":        ts.channel,
	}
	if exec.response.Usage != nil {
		llmResponseFields["prompt_tokens"] = exec.response.Usage.PromptTokens
		llmResponseFields["completion_tokens"] = exec.response.Usage.CompletionTokens
		llmResponseFields["total_tokens"] = exec.response.Usage.TotalTokens
	}
	logger.DebugCF("agent", "LLM response", llmResponseFields)

	// No-tool-call path: steering check and direct response
	if len(exec.response.ToolCalls) == 0 || exec.gracefulTerminal {
		responseContent := exec.response.Content
		if responseContent == "" && exec.response.ReasoningContent != "" && ts.channel != "pico" {
			responseContent = exec.response.ReasoningContent
		}
		if steerMsgs := al.dequeueSteeringMessagesForScope(ts.sessionKey); len(steerMsgs) > 0 {
			logger.InfoCF("agent", "Steering arrived after direct LLM response; continuing turn",
				map[string]any{
					"agent_id":       ts.agent.ID,
					"iteration":      iteration,
					"steering_count": len(steerMsgs),
				})
			exec.pendingMessages = append(exec.pendingMessages, steerMsgs...)
			return ControlContinue, nil
		}
		exec.finalContent = responseContent
		logger.InfoCF("agent", "LLM response without tool calls (direct answer)",
			map[string]any{
				"agent_id":      ts.agent.ID,
				"iteration":     iteration,
				"content_chars": len(exec.finalContent),
			})
		return ControlBreak, nil
	}

	// Tool-call path: normalize and prepare for tool execution
	exec.normalizedToolCalls = make([]providers.ToolCall, 0, len(exec.response.ToolCalls))
	for _, tc := range exec.response.ToolCalls {
		exec.normalizedToolCalls = append(exec.normalizedToolCalls, providers.NormalizeToolCall(tc))
	}

	toolNames := make([]string, 0, len(exec.normalizedToolCalls))
	for _, tc := range exec.normalizedToolCalls {
		toolNames = append(toolNames, tc.Name)
	}
	logger.InfoCF("agent", "LLM requested tool calls",
		map[string]any{
			"agent_id":  ts.agent.ID,
			"tools":     toolNames,
			"count":     len(exec.normalizedToolCalls),
			"iteration": iteration,
		})

	exec.allResponsesHandled = len(exec.normalizedToolCalls) > 0
	assistantMsg := providers.Message{
		Role:             "assistant",
		Content:          exec.response.Content,
		ReasoningContent: reasoningContent,
	}
	for _, tc := range exec.normalizedToolCalls {
		argumentsJSON, _ := json.Marshal(tc.Arguments)
		toolFeedbackExplanation := toolFeedbackExplanationForToolCall(
			exec.response,
			tc,
			exec.messages,
		)
		extraContent := tc.ExtraContent
		if strings.TrimSpace(toolFeedbackExplanation) != "" {
			if extraContent == nil {
				extraContent = &providers.ExtraContent{}
			}
			extraContent.ToolFeedbackExplanation = toolFeedbackExplanation
		}
		thoughtSignature := ""
		if tc.Function != nil {
			thoughtSignature = tc.Function.ThoughtSignature
		}
		assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, providers.ToolCall{
			ID:   tc.ID,
			Type: "function",
			Name: tc.Name,
			Function: &providers.FunctionCall{
				Name:             tc.Name,
				Arguments:        string(argumentsJSON),
				ThoughtSignature: thoughtSignature,
			},
			ExtraContent:     extraContent,
			ThoughtSignature: thoughtSignature,
		})
	}
	exec.messages = append(exec.messages, assistantMsg)
	if !ts.opts.NoHistory {
		ts.agent.Sessions.AddFullMessage(ts.sessionKey, assistantMsg)
		ts.recordPersistedMessage(assistantMsg)
		ts.ingestMessage(turnCtx, al, assistantMsg)
	}
	if shouldPublishPicoToolCallInterim {
		al.publishPicoToolCallInterim(
			turnCtx,
			ts,
			reasoningContent,
			exec.response.Content,
			assistantMsg.ToolCalls,
		)
	}

	return ControlToolLoop, nil
}
