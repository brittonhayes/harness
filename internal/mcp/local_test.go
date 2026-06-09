package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeDir creates a temp directory seeded with the given files (name -> body)
// and returns its path.
func writeDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// call runs a tool on the session and decodes its JSON text into v.
func call(t *testing.T, s Session, name string, args map[string]any, v any) CallResult {
	t.Helper()
	raw, _ := json.Marshal(args)
	res, err := s.CallTool(context.Background(), name, raw)
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if res.IsError {
		t.Fatalf("CallTool(%s) error: %s", name, res.Text)
	}
	if v != nil {
		if err := json.Unmarshal([]byte(res.Text), v); err != nil {
			t.Fatalf("decode %s result %q: %v", name, res.Text, err)
		}
	}
	return res
}

func TestLocalSessionToolsAreReadOnly(t *testing.T) {
	s, err := NewLocal("scanner", writeDir(t, nil))
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	if s.Name() != "scanner" {
		t.Fatalf("name = %q, want scanner", s.Name())
	}
	tools, err := s.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	for _, td := range tools {
		if !td.ReadOnly {
			t.Fatalf("tool %q should be read-only", td.Name)
		}
	}
}

func TestLocalLoadContextReportsIndexesAndFields(t *testing.T) {
	dir := writeDir(t, map[string]string{
		"cloudtrail.jsonl": `{"eventName":"StopLogging","userIdentity":{"type":"IAMUser"}}` + "\n" +
			`{"eventName":"ConsoleLogin","userIdentity":{"type":"Root"}}` + "\n",
	})
	s, err := NewLocal("scanner", dir)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	var ctx struct {
		Indexes []struct {
			Name   string   `json:"name"`
			Events int      `json:"events"`
			Fields []string `json:"fields"`
		} `json:"indexes"`
	}
	call(t, s, "load_context", nil, &ctx)
	if len(ctx.Indexes) != 1 {
		t.Fatalf("expected 1 index, got %d", len(ctx.Indexes))
	}
	idx := ctx.Indexes[0]
	if idx.Name != "cloudtrail" || idx.Events != 2 {
		t.Fatalf("index = %+v, want cloudtrail with 2 events", idx)
	}
	// Nested fields are flattened to dotted paths for discovery.
	want := map[string]bool{"eventName": false, "userIdentity.type": false}
	for _, f := range idx.Fields {
		if _, ok := want[f]; ok {
			want[f] = true
		}
	}
	for f, seen := range want {
		if !seen {
			t.Fatalf("field %q missing from %v", f, idx.Fields)
		}
	}
}

func TestLocalExecuteQueryFilters(t *testing.T) {
	dir := writeDir(t, map[string]string{
		"cloudtrail.jsonl": `{"eventName":"StopLogging","userIdentity":{"type":"IAMUser","userName":"bob"}}` + "\n" +
			`{"eventName":"ConsoleLogin","userIdentity":{"type":"Root"}}` + "\n" +
			`{"eventName":"ConsoleLogin","userIdentity":{"type":"IAMUser","userName":"alice"}}` + "\n",
	})
	s, err := NewLocal("scanner", dir)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}

	type result struct {
		Count    int              `json:"count"`
		Returned int              `json:"returned"`
		Results  []map[string]any `json:"results"`
	}

	// Bare term matches anywhere in the event.
	var r result
	call(t, s, "execute_query", map[string]any{"query": "StopLogging"}, &r)
	if r.Count != 1 || len(r.Results) != 1 {
		t.Fatalf("StopLogging: count=%d results=%d, want 1/1", r.Count, len(r.Results))
	}
	if r.Results[0]["_index"] != "cloudtrail" {
		t.Fatalf("result missing _index tag: %v", r.Results[0])
	}

	// field:value matches a dotted nested field.
	r = result{}
	call(t, s, "execute_query", map[string]any{"query": "userIdentity.type:Root"}, &r)
	if r.Count != 1 {
		t.Fatalf("Root filter: count=%d, want 1", r.Count)
	}

	// Multiple terms AND together — no event is both ConsoleLogin and Root user bob.
	r = result{}
	call(t, s, "execute_query", map[string]any{"query": "ConsoleLogin userIdentity.type:IAMUser"}, &r)
	if r.Count != 1 {
		t.Fatalf("ANDed terms: count=%d, want 1", r.Count)
	}

	// Empty query returns everything.
	r = result{}
	call(t, s, "execute_query", map[string]any{"query": ""}, &r)
	if r.Count != 3 {
		t.Fatalf("empty query: count=%d, want 3", r.Count)
	}
}

func TestLocalExecuteQueryLimitTruncates(t *testing.T) {
	dir := writeDir(t, map[string]string{
		"big.jsonl": `{"n":1}` + "\n" + `{"n":2}` + "\n" + `{"n":3}` + "\n",
	})
	s, err := NewLocal("scanner", dir)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	var r struct {
		Count     int  `json:"count"`
		Returned  int  `json:"returned"`
		Truncated bool `json:"truncated"`
	}
	call(t, s, "execute_query", map[string]any{"query": "", "limit": 2}, &r)
	if r.Count != 3 || r.Returned != 2 || !r.Truncated {
		t.Fatalf("limit: count=%d returned=%d truncated=%v, want 3/2/true", r.Count, r.Returned, r.Truncated)
	}
}

func TestLocalQueryRestrictsToIndex(t *testing.T) {
	dir := writeDir(t, map[string]string{
		"a.jsonl": `{"eventName":"X"}` + "\n",
		"b.jsonl": `{"eventName":"X"}` + "\n",
	})
	s, err := NewLocal("scanner", dir)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	var r struct {
		Count int `json:"count"`
	}
	call(t, s, "execute_query", map[string]any{"query": "X", "index": "a"}, &r)
	if r.Count != 1 {
		t.Fatalf("index restriction: count=%d, want 1", r.Count)
	}
}

func TestLocalMissingDirIsEmptyNotError(t *testing.T) {
	s, err := NewLocal("scanner", filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("NewLocal on missing dir should not error: %v", err)
	}
	var ctx struct {
		Indexes []any `json:"indexes"`
	}
	call(t, s, "load_context", nil, &ctx)
	if len(ctx.Indexes) != 0 {
		t.Fatalf("expected no indexes, got %d", len(ctx.Indexes))
	}
}

func TestLocalJSONArrayFile(t *testing.T) {
	dir := writeDir(t, map[string]string{
		"arr.json": `[{"eventName":"A"},{"eventName":"B"}]`,
	})
	s, err := NewLocal("scanner", dir)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	var r struct {
		Count int `json:"count"`
	}
	call(t, s, "execute_query", map[string]any{"query": ""}, &r)
	if r.Count != 2 {
		t.Fatalf("array file: count=%d, want 2", r.Count)
	}
}

func TestLocalMalformedFileErrors(t *testing.T) {
	dir := writeDir(t, map[string]string{
		"bad.jsonl": `{"eventName":"A"}` + "\n" + `{not json}` + "\n",
	})
	if _, err := NewLocal("scanner", dir); err == nil {
		t.Fatal("expected error loading malformed file, got nil")
	}
}
