package cmd

import (
	"testing"

	"github.com/brittonhayes/vala/internal/brain"
	"github.com/brittonhayes/vala/internal/config"
)

// TestBrainStoreSelectsBackend pins the backend-selection precedence: Notion
// wins when configured, a brain file gives a durable store, and an unconfigured
// brain falls back to ephemeral memory.
func TestBrainStoreSelectsBackend(t *testing.T) {
	cwd := t.TempDir()

	if _, ok := brainStore(config.Config{}, cwd).(*brain.Mem); !ok {
		t.Fatalf("unconfigured brain should be *brain.Mem")
	}
	if _, ok := brainStore(config.Config{BrainFile: "brain.json"}, cwd).(*brain.File); !ok {
		t.Fatalf("brain_file should select *brain.File")
	}
	cfg := config.Config{BrainFile: "brain.json", Notion: brain.DBIDs{Hunts: "ds"}}
	if _, ok := brainStore(cfg, cwd).(*brain.NTN); !ok {
		t.Fatalf("configured Notion should win and select *brain.NTN")
	}
}

// TestBrainConfigured drives the predicate that gates the first-run prompt and
// brainStore's backend choice: any of hunts, intel, or evidence being set means
// the brain persists to Notion.
func TestBrainConfigured(t *testing.T) {
	cases := []struct {
		name string
		ids  brain.DBIDs
		want bool
	}{
		{"empty", brain.DBIDs{}, false},
		{"only parent set", brain.DBIDs{Parent: "page_1"}, false},
		{"evidence set", brain.DBIDs{Evidence: "ds_e"}, true},
		{"hunts set", brain.DBIDs{Hunts: "ds_h"}, true},
		{"intel set", brain.DBIDs{Intel: "ds_i"}, true},
		{"fully configured", brain.DBIDs{Evidence: "e", Hunts: "h", Intel: "i"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := brainConfigured(config.Config{Notion: c.ids}); got != c.want {
				t.Errorf("brainConfigured(%+v) = %v, want %v", c.ids, got, c.want)
			}
		})
	}
}

// fullNotion is a DBIDs with every store's data source set (a complete brain).
var fullNotion = brain.DBIDs{
	Database: "db", Evidence: "e", Hunts: "h", Intel: "i",
	Detections: "d", Backlog: "b", Memory: "m", Coverage: "c",
}

// TestBrainComplete pins the broken-brain detector: a brain missing any store is
// incomplete (the failing-coverage case), while the parent database is not
// required so a working legacy multi-database brain still reads as complete.
func TestBrainComplete(t *testing.T) {
	if !brainComplete(config.Config{Notion: fullNotion}) {
		t.Error("all stores set should be complete")
	}
	partial := fullNotion
	partial.Coverage = ""
	if brainComplete(config.Config{Notion: partial}) {
		t.Error("a missing coverage store should be incomplete")
	}
	legacy := fullNotion
	legacy.Database = "" // legacy multi-DB brains have no parent database
	if !brainComplete(config.Config{Notion: legacy}) {
		t.Error("the parent database should not be required for completeness")
	}
}
