package tracing

import (
	"context"
	"testing"
)

func TestHeadersFromContext_NoConfig(t *testing.T) {
	ctx := context.Background()
	headers := HeadersFromContext(ctx)
	if headers != nil {
		t.Fatalf("expected nil headers when no config, got %v", headers)
	}
}

func TestHeadersFromContext_EmptyMapping(t *testing.T) {
	ctx := context.Background()
	ctx = WithHeaders(ctx, map[string]string{})
	headers := HeadersFromContext(ctx)
	if headers != nil {
		t.Fatalf("expected nil headers with empty mapping, got %v", headers)
	}
}

func TestHeadersFromContext_Full(t *testing.T) {
	ctx := context.Background()
	ctx = WithHeaders(ctx, map[string]string{
		"AH-Thread-Id": "session_key",
		"AH-Trace-Id":  "turn_id",
		"X-Agent":      "agent_id",
	})
	ctx = WithSessionKey(ctx, "sess-123")
	ctx = WithTurnID(ctx, "turn-456")
	ctx = WithAgentID(ctx, "agent-789")

	headers := HeadersFromContext(ctx)
	if len(headers) != 3 {
		t.Fatalf("expected 3 headers, got %d: %v", len(headers), headers)
	}
	if headers["AH-Thread-Id"] != "sess-123" {
		t.Errorf("expected AH-Thread-Id=sess-123, got %s", headers["AH-Thread-Id"])
	}
	if headers["AH-Trace-Id"] != "turn-456" {
		t.Errorf("expected AH-Trace-Id=turn-456, got %s", headers["AH-Trace-Id"])
	}
	if headers["X-Agent"] != "agent-789" {
		t.Errorf("expected X-Agent=agent-789, got %s", headers["X-Agent"])
	}
}

func TestHeadersFromContext_CustomHeaderNames(t *testing.T) {
	ctx := context.Background()
	ctx = WithHeaders(ctx, map[string]string{
		"X-Session": "session_key",
		"X-Turn":    "turn_id",
	})
	ctx = WithSessionKey(ctx, "abc")
	ctx = WithTurnID(ctx, "def")

	headers := HeadersFromContext(ctx)
	if headers["X-Session"] != "abc" {
		t.Errorf("expected X-Session=abc, got %s", headers["X-Session"])
	}
	if headers["X-Turn"] != "def" {
		t.Errorf("expected X-Turn=def, got %s", headers["X-Turn"])
	}
}

func TestHeadersFromContext_UnknownFieldName(t *testing.T) {
	ctx := context.Background()
	ctx = WithHeaders(ctx, map[string]string{
		"X-Unknown": "unknown_field",
		"X-Valid":   "session_key",
	})
	ctx = WithSessionKey(ctx, "abc")

	headers := HeadersFromContext(ctx)
	if len(headers) != 1 {
		t.Fatalf("expected 1 header (unknown_field ignored), got %d", len(headers))
	}
	if headers["X-Valid"] != "abc" {
		t.Errorf("expected X-Valid=abc, got %s", headers["X-Valid"])
	}
}

func TestHeadersFromContext_EmptyValue(t *testing.T) {
	ctx := context.Background()
	ctx = WithHeaders(ctx, map[string]string{
		"AH-Thread-Id": "session_key",
		"AH-Trace-Id":  "turn_id",
	})
	// 只设 session_key，不设 turn_id
	ctx = WithSessionKey(ctx, "abc")

	headers := HeadersFromContext(ctx)
	if len(headers) != 1 {
		t.Fatalf("expected 1 header, got %d", len(headers))
	}
	if headers["AH-Thread-Id"] != "abc" {
		t.Errorf("expected AH-Thread-Id=abc, got %s", headers["AH-Thread-Id"])
	}
}

func TestHeadersFromContext_AllEmptyValues(t *testing.T) {
	ctx := context.Background()
	ctx = WithHeaders(ctx, map[string]string{
		"AH-Thread-Id": "session_key",
		"AH-Trace-Id":  "turn_id",
	})
	// 不注入任何值

	headers := HeadersFromContext(ctx)
	if headers != nil {
		t.Fatalf("expected nil when all values empty, got %v", headers)
	}
}

func TestHeadersFromContext_OnlyAgentID(t *testing.T) {
	ctx := context.Background()
	ctx = WithHeaders(ctx, map[string]string{
		"X-Agent-ID": "agent_id",
	})
	ctx = WithAgentID(ctx, "my-agent")

	headers := HeadersFromContext(ctx)
	if len(headers) != 1 {
		t.Fatalf("expected 1 header, got %d", len(headers))
	}
	if headers["X-Agent-ID"] != "my-agent" {
		t.Errorf("expected X-Agent-ID=my-agent, got %s", headers["X-Agent-ID"])
	}
}

func TestHeadersFromContext_AllFields(t *testing.T) {
	ctx := context.Background()
	ctx = WithHeaders(ctx, map[string]string{
		"X-Session":     "session_key",
		"X-Turn":        "turn_id",
		"X-Agent-ID":    "agent_id",
		"X-Agent-Name":  "agent_name",
		"X-Channel":     "channel",
		"X-Chat-ID":     "chat_id",
		"X-Parent-Turn": "parent_turn_id",
		"X-Sender-ID":   "sender_id",
		"X-Message-ID":  "message_id",
		"X-Model":       "model",
	})
	ctx = WithSessionKey(ctx, "sess-1")
	ctx = WithTurnID(ctx, "turn-2")
	ctx = WithAgentID(ctx, "agent-3")
	ctx = WithAgentName(ctx, "my-agent")
	ctx = WithChannel(ctx, "discord")
	ctx = WithChatID(ctx, "chat-456")
	ctx = WithParentTurnID(ctx, "parent-7")
	ctx = WithSenderID(ctx, "user-8")
	ctx = WithMessageID(ctx, "msg-9")
	ctx = WithModel(ctx, "gpt-5.4")

	headers := HeadersFromContext(ctx)
	if len(headers) != 10 {
		t.Fatalf("expected 10 headers, got %d: %v", len(headers), headers)
	}
	if headers["X-Session"] != "sess-1" {
		t.Errorf("expected X-Session=sess-1, got %s", headers["X-Session"])
	}
	if headers["X-Channel"] != "discord" {
		t.Errorf("expected X-Channel=discord, got %s", headers["X-Channel"])
	}
	if headers["X-Model"] != "gpt-5.4" {
		t.Errorf("expected X-Model=gpt-5.4, got %s", headers["X-Model"])
	}
	if headers["X-Agent-Name"] != "my-agent" {
		t.Errorf("expected X-Agent-Name=my-agent, got %s", headers["X-Agent-Name"])
	}
	if headers["X-Parent-Turn"] != "parent-7" {
		t.Errorf("expected X-Parent-Turn=parent-7, got %s", headers["X-Parent-Turn"])
	}
}

func TestAvailableFields(t *testing.T) {
	fields := AvailableFields()
	if len(fields) != 10 {
		t.Fatalf("expected 10 available fields, got %d: %v", len(fields), fields)
	}
	// 确认包含新增字段
	fieldSet := make(map[string]bool, len(fields))
	for _, f := range fields {
		fieldSet[f] = true
	}
	for _, name := range []string{"session_key", "turn_id", "agent_id", "agent_name", "channel", "chat_id", "parent_turn_id", "sender_id", "message_id", "model"} {
		if !fieldSet[name] {
			t.Errorf("expected field %s in AvailableFields()", name)
		}
	}
}
