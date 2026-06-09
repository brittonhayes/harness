package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LocalSession is a file-backed Session: an offline stand-in for a remote
// evidence server like Scanner. It reads log events from a directory of JSON
// files and exposes the same Scanner-shaped, read-only tools the agent already
// knows — load_context to discover indexes and fields, and execute_query to
// search them — so a hunt can be driven end to end without any network access
// or API key. Name it "scanner" and the existing prompts and fixtures work
// unchanged against local sample data.
//
// Each file in the directory is one index. Files may be a JSON array of event
// objects, or newline-delimited JSON (one object per line, .jsonl/.ndjson). The
// index name is the file's base name without extension.
type LocalSession struct {
	name    string
	indexes []localIndex
}

// localIndex is one loaded log file: a named collection of event objects.
type localIndex struct {
	name   string
	events []map[string]any
}

// defaultQueryLimit caps how many matching rows execute_query returns inline,
// keeping a broad query from flooding the model's context.
const defaultQueryLimit = 100

// NewLocal loads every JSON log file under dir and returns a file-backed
// session. An empty or missing directory is not an error — the session simply
// exposes no events, which surfaces clearly through load_context — but a file
// that exists and fails to parse is, so bad sample data is caught early.
func NewLocal(name, dir string) (Session, error) {
	if name == "" {
		name = "scanner"
	}
	s := &LocalSession{name: name}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("local evidence %q: read dir %s: %w", name, dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".json", ".jsonl", ".ndjson":
		default:
			continue
		}
		path := filepath.Join(dir, e.Name())
		events, err := loadEvents(path)
		if err != nil {
			return nil, fmt.Errorf("local evidence %q: %w", name, err)
		}
		idxName := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		s.indexes = append(s.indexes, localIndex{name: idxName, events: events})
	}
	sort.Slice(s.indexes, func(i, j int) bool { return s.indexes[i].name < s.indexes[j].name })
	return s, nil
}

// loadEvents parses one log file as either a JSON array of objects or
// newline-delimited JSON objects, choosing by content so a .json file holding
// NDJSON still loads.
func loadEvents(path string) ([]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var arr []map[string]any
		if err := json.Unmarshal([]byte(trimmed), &arr); err != nil {
			return nil, fmt.Errorf("parse JSON array %s: %w", path, err)
		}
		return arr, nil
	}
	var events []map[string]any
	for i, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			return nil, fmt.Errorf("parse %s line %d: %w", path, i+1, err)
		}
		events = append(events, ev)
	}
	return events, nil
}

func (s *LocalSession) Name() string { return s.name }

func (s *LocalSession) ListTools(context.Context) ([]ToolDesc, error) {
	return []ToolDesc{
		{
			Name:        "load_context",
			Description: "Discover the available log indexes and their fields before querying. Returns each index's event count and observed field names. Call this first to learn what you can hunt in.",
			Properties:  map[string]any{},
			ReadOnly:    true,
		},
		{
			Name:        "execute_query",
			Description: "Search the local log indexes for events. The query is space-separated terms, ANDed together: a bare term (e.g. StopLogging) matches anywhere in the event; a field:value term (e.g. userIdentity.type:Root) matches that dotted field. An empty query returns all events. Optionally restrict to one index, or raise the result limit.",
			Properties: map[string]any{
				"query": map[string]any{"type": "string", "description": "Space-separated search terms; bare or field:value, ANDed."},
				"index": map[string]any{"type": "string", "description": "Restrict the search to a single index by name (optional)."},
				"limit": map[string]any{"type": "integer", "description": fmt.Sprintf("Max rows to return (default %d).", defaultQueryLimit)},
			},
			Required: []string{"query"},
			ReadOnly: true,
		},
	}, nil
}

// queryArgs is the decoded input to execute_query.
type queryArgs struct {
	Query string `json:"query"`
	Index string `json:"index"`
	Limit int    `json:"limit"`
}

func (s *LocalSession) CallTool(_ context.Context, name string, args json.RawMessage) (CallResult, error) {
	switch name {
	case "load_context":
		return s.loadContext()
	case "execute_query":
		var a queryArgs
		if len(args) > 0 {
			if err := json.Unmarshal(args, &a); err != nil {
				return CallResult{Text: fmt.Sprintf("invalid arguments: %v", err), IsError: true}, nil
			}
		}
		return s.executeQuery(a)
	default:
		return CallResult{Text: fmt.Sprintf("unknown tool %q", name), IsError: true}, nil
	}
}

func (s *LocalSession) Close() error { return nil }

// loadContext reports the indexes, their sizes, and the union of dotted field
// names seen in each, so the agent can scope a query without guessing.
func (s *LocalSession) loadContext() (CallResult, error) {
	type indexCtx struct {
		Name   string   `json:"name"`
		Events int      `json:"events"`
		Fields []string `json:"fields"`
	}
	out := struct {
		Indexes []indexCtx `json:"indexes"`
	}{Indexes: []indexCtx{}}
	for _, idx := range s.indexes {
		fields := map[string]bool{}
		for _, ev := range idx.events {
			collectFields("", ev, fields)
		}
		names := make([]string, 0, len(fields))
		for f := range fields {
			names = append(names, f)
		}
		sort.Strings(names)
		out.Indexes = append(out.Indexes, indexCtx{Name: idx.name, Events: len(idx.events), Fields: names})
	}
	return jsonResult(out)
}

// executeQuery searches the (optionally named) indexes and returns matching
// rows, each tagged with its source index, up to the limit.
func (s *LocalSession) executeQuery(a queryArgs) (CallResult, error) {
	limit := a.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	terms := parseTerms(a.Query)

	results := []map[string]any{}
	total := 0
	for _, idx := range s.indexes {
		if a.Index != "" && idx.name != a.Index {
			continue
		}
		for _, ev := range idx.events {
			if !matchAll(ev, terms) {
				continue
			}
			total++
			if len(results) >= limit {
				continue
			}
			row := map[string]any{"_index": idx.name}
			for k, v := range ev {
				row[k] = v
			}
			results = append(results, row)
		}
	}
	out := map[string]any{"count": total, "returned": len(results), "results": results}
	if total > len(results) {
		out["truncated"] = true
	}
	return jsonResult(out)
}

// term is one parsed query token: a bare substring, or a field:value filter.
type term struct {
	field string // dotted field path; empty for a bare term
	value string // lowercased match value
}

// parseTerms splits a query into ANDed terms. A token containing ':' becomes a
// field:value filter; everything else is a bare substring match.
func parseTerms(q string) []term {
	var terms []term
	for _, tok := range strings.Fields(q) {
		if i := strings.Index(tok, ":"); i > 0 {
			terms = append(terms, term{field: tok[:i], value: strings.ToLower(tok[i+1:])})
			continue
		}
		terms = append(terms, term{value: strings.ToLower(tok)})
	}
	return terms
}

// matchAll reports whether the event satisfies every term.
func matchAll(ev map[string]any, terms []term) bool {
	for _, t := range terms {
		if t.field != "" {
			v, ok := lookup(ev, t.field)
			if !ok || !strings.Contains(strings.ToLower(stringify(v)), t.value) {
				return false
			}
			continue
		}
		if !strings.Contains(strings.ToLower(stringify(ev)), t.value) {
			return false
		}
	}
	return true
}

// lookup resolves a dotted field path against a nested event.
func lookup(ev map[string]any, path string) (any, bool) {
	var cur any = ev
	for _, seg := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[seg]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// collectFields walks an event and records every dotted leaf path into seen.
func collectFields(prefix string, ev map[string]any, seen map[string]bool) {
	for k, v := range ev {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		if child, ok := v.(map[string]any); ok {
			collectFields(path, child, seen)
			continue
		}
		seen[path] = true
	}
}

// stringify renders a value for substring matching: scalars verbatim, composite
// values as their JSON encoding.
func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	case map[string]any, []any:
		b, _ := json.Marshal(t)
		return string(b)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// jsonResult marshals v into a CallResult, matching the JSON-text shape the real
// Scanner tools (and the harness fake) return.
func jsonResult(v any) (CallResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return CallResult{Text: fmt.Sprintf("marshal result: %v", err), IsError: true}, nil
	}
	return CallResult{Text: string(b)}, nil
}
