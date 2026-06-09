package brain

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSchemaStatusOptions guards the alignment that the live workspace caught:
// Notion does not auto-create status options on write, so every "status"
// property in the schema must declare the option set its writer emits. A status
// column born without options 400s the first write.
func TestSchemaStatusOptions(t *testing.T) {
	for _, s := range Schema() {
		for _, p := range s.Props {
			if p.Type != "status" {
				continue
			}
			if len(s.StatusOptions[p.Name]) == 0 {
				t.Errorf("%s.%s is a status property with no StatusOptions", s.Name, p.Name)
			}
		}
	}
}

// TestSchemaHasExactlyOneTitle asserts each store has exactly one title column —
// the row's display field the writers populate.
func TestSchemaHasExactlyOneTitle(t *testing.T) {
	for _, s := range Schema() {
		titles := 0
		for _, p := range s.Props {
			if p.Type == "title" {
				titles++
			}
		}
		if titles != 1 {
			t.Errorf("%s has %d title properties, want exactly 1", s.Name, titles)
		}
	}
}

// TestPropConfigStatusOptions checks the status property configuration carries
// the seeded options in the Notion shape the create API expects.
func TestPropConfigStatusOptions(t *testing.T) {
	cfg := propConfig("status", []string{"Open", "Confirmed"})
	b, _ := json.Marshal(cfg)
	got := string(b)
	for _, want := range []string{`"type":"status"`, `"name":"Open"`, `"name":"Confirmed"`} {
		if !strings.Contains(got, want) {
			t.Errorf("propConfig status = %s, missing %q", got, want)
		}
	}
}
