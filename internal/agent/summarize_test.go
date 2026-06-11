package agent

import (
	"encoding/json"
	"testing"
)

func TestSummarize(t *testing.T) {
	tests := []struct {
		name  string
		tool  string
		input map[string]any
		want  string
	}{
		{"bash uses command", "bash", map[string]any{"command": "aws sts get-caller-identity"}, "aws sts get-caller-identity"},
		{"write uses path", "write", map[string]any{"path": "detections/x.yml", "content": "..."}, "detections/x.yml"},
		{"ntn joins args", "ntn", map[string]any{"args": []any{"pages", "list"}}, "ntn pages list"},
		{"grep uses pattern", "grep", map[string]any{"pattern": "AssumeRole"}, "AssumeRole"},
		// Unknown/MCP tools render a compact, sorted "key: value" glance rather
		// than raw JSON, with arrays comma-joined and integers un-floated.
		{"mcp tool flattens args", "wiz_list_cloud_events",
			map[string]any{"severity": []any{"CRITICAL", "HIGH"}, "first": 100},
			"first: 100 · severity: CRITICAL,HIGH"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, _ := json.Marshal(tt.input)
			if got := summarize(tt.tool, raw); got != tt.want {
				t.Fatalf("summarize(%s) = %q, want %q", tt.tool, got, tt.want)
			}
		})
	}
}
