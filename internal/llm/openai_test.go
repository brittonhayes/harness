package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestToOpenAIMessagesShape(t *testing.T) {
	// A conversation that exercises every block type: a user turn, an assistant
	// turn with text + a tool call, and the tool result fed back.
	conv := []Message{
		UserText("hunt for root logins"),
		AssistantMessage(
			TextBlock("Looking now."),
			ToolUseBlock("call_1", "bash", json.RawMessage(`{"command":"echo hi"}`)),
		),
		UserMessage(ToolResultBlock("call_1", "hi", false)),
	}
	got := toOpenAIMessages("you are vala", conv)

	// system, user, assistant(with tool_calls), tool
	if len(got) != 4 {
		t.Fatalf("message count = %d, want 4: %+v", len(got), got)
	}
	if got[0].Role != "system" || got[0].Content != "you are vala" {
		t.Errorf("first message = %+v, want system prompt", got[0])
	}
	if got[1].Role != "user" || got[1].Content != "hunt for root logins" {
		t.Errorf("second message = %+v, want user", got[1])
	}
	asst := got[2]
	if asst.Role != "assistant" || asst.Content != "Looking now." {
		t.Errorf("assistant message text = %+v", asst)
	}
	if len(asst.ToolCalls) != 1 || asst.ToolCalls[0].ID != "call_1" || asst.ToolCalls[0].Function.Name != "bash" {
		t.Errorf("assistant tool_calls = %+v", asst.ToolCalls)
	}
	// Tool results become a dedicated role:"tool" message, not a user message.
	if got[3].Role != "tool" || got[3].ToolCallID != "call_1" || got[3].Content != "hi" {
		t.Errorf("tool result message = %+v", got[3])
	}
}

func TestOpenAICompleteRoundTrip(t *testing.T) {
	var gotBody oaiRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer sk-test" {
			t.Errorf("Authorization = %q", auth)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"done","tool_calls":[
				{"id":"call_9","type":"function","function":{"name":"read","arguments":"{\"path\":\"/etc/hosts\"}"}}
			]}}],
			"usage":{"prompt_tokens":12,"completion_tokens":3}
		}`))
	}))
	defer srv.Close()

	info := ProviderInfo{ID: "openai-compatible", Protocol: ProtocolOpenAI, BaseURL: srv.URL}
	p := newOpenAI(info, "sk-test", srv.URL, "gpt-test", 1000, 128000)

	tools := []ToolDef{{Name: "read", Description: "read a file", Properties: map[string]any{"path": map[string]any{"type": "string"}}, Required: []string{"path"}}}
	resp, err := p.Complete(context.Background(), "sys", []Message{UserText("hi")}, tools)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	// Request: tool definition forwarded with an object schema.
	if len(gotBody.Tools) != 1 || gotBody.Tools[0].Function.Name != "read" {
		t.Errorf("tools sent = %+v", gotBody.Tools)
	}
	if gotBody.Tools[0].Function.Parameters["type"] != "object" {
		t.Errorf("tool params type = %v, want object", gotBody.Tools[0].Function.Parameters["type"])
	}

	// Response: text + tool_use decoded into neutral blocks, usage carried.
	if len(resp.Content) != 2 {
		t.Fatalf("response blocks = %d, want 2: %+v", len(resp.Content), resp.Content)
	}
	if resp.Content[0].Type != BlockText || resp.Content[0].Text != "done" {
		t.Errorf("first block = %+v", resp.Content[0])
	}
	tu := resp.Content[1]
	if tu.Type != BlockToolUse || tu.Name != "read" || tu.ID != "call_9" {
		t.Errorf("tool_use block = %+v", tu)
	}
	if string(tu.Input) != `{"path":"/etc/hosts"}` {
		t.Errorf("tool_use input = %s", tu.Input)
	}
	if resp.Usage.InputTokens != 12 || resp.Usage.OutputTokens != 3 {
		t.Errorf("usage = %+v", resp.Usage)
	}
}

func TestOpenAICompleteErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()

	p := newOpenAI(ProviderInfo{ID: "openai", Protocol: ProtocolOpenAI}, "bad", srv.URL, "m", 100, 1000)
	_, err := p.Complete(context.Background(), "", []Message{UserText("hi")}, nil)
	if err == nil {
		t.Fatal("expected an error for a 401 response")
	}
}
