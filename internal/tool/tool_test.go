package tool

import (
	"context"
	"encoding/json"
	"testing"
)

// fakeTool is a minimal Tool used to exercise the registry.
type fakeTool struct {
	name     string
	readOnly bool
}

func (f fakeTool) Name() string        { return f.name }
func (f fakeTool) Description() string { return "desc of " + f.name }
func (f fakeTool) ReadOnly() bool      { return f.readOnly }
func (f fakeTool) Schema() Schema      { return Schema{Properties: map[string]any{}} }
func (f fakeTool) Run(context.Context, json.RawMessage) (Result, error) {
	return Text("ok"), nil
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeTool{name: "zebra"}, fakeTool{name: "alpha", readOnly: true})

	if _, ok := r.Get("alpha"); !ok {
		t.Fatal("expected to find alpha")
	}
	if _, ok := r.Get("missing"); ok {
		t.Fatal("did not expect to find missing")
	}

	all := r.All()
	if len(all) != 2 || all[0].Name() != "alpha" || all[1].Name() != "zebra" {
		t.Fatalf("All() not sorted: %v", names(all))
	}
}

func TestToolDefs(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeTool{name: "alpha"})
	defs := r.ToolDefs()
	if len(defs) != 1 {
		t.Fatalf("expected 1 tool def, got %d", len(defs))
	}
	td := defs[0]
	if td.Name != "alpha" {
		t.Fatalf("unexpected tool def: %+v", td)
	}
	if td.Properties == nil {
		t.Fatal("expected non-nil properties")
	}
}

func names(ts []Tool) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Name()
	}
	return out
}
