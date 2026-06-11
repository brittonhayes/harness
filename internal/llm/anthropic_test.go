package llm

import (
	"encoding/json"
	"testing"
)

func TestToAnthropicToolsEmptyIsNil(t *testing.T) {
	if got := toAnthropicTools(nil); got != nil {
		t.Fatalf("toAnthropicTools(nil) = %v, want nil so a tool-free call sends no tools", got)
	}
}

func TestToAnthropicToolsShape(t *testing.T) {
	tools := []ToolDef{{
		Name:        "read",
		Description: "read a file",
		Properties:  map[string]any{"path": map[string]any{"type": "string"}},
		Required:    []string{"path"},
	}}
	got := toAnthropicTools(tools)
	if len(got) != 1 || got[0].OfTool == nil {
		t.Fatalf("expected one tool param, got %+v", got)
	}
	tp := got[0].OfTool
	if tp.Name != "read" {
		t.Errorf("tool name = %q, want read", tp.Name)
	}
	if tp.InputSchema.Properties == nil {
		t.Error("expected non-nil input schema properties")
	}
}

func TestToAnthropicMessagesRolesAndBlocks(t *testing.T) {
	conv := []Message{
		UserText("hi"),
		AssistantMessage(
			TextBlock("ok"),
			ToolUseBlock("c1", "bash", json.RawMessage(`{"command":"ls"}`)),
		),
		UserMessage(ToolResultBlock("c1", "out", false)),
	}
	got := toAnthropicMessages(conv)
	if len(got) != 3 {
		t.Fatalf("message count = %d, want 3", len(got))
	}
	if got[0].Role != "user" || got[1].Role != "assistant" || got[2].Role != "user" {
		t.Errorf("roles = %q, %q, %q", got[0].Role, got[1].Role, got[2].Role)
	}
}
