package tools

import (
	"testing"

	"github.com/brittonhayes/vala/internal/brain"
)

func TestRunContextSnapshotCopiesState(t *testing.T) {
	rc := NewRunContext(nil)
	rc.SetAuthor("alice")
	rc.SetHunt("hunt-123", "Investigate root logins", brain.HuntHypothesis)
	rc.addEvidence(brain.Evidence{ID: "ev-1", Claim: "root login observed", Source: brain.EvidenceQuery})
	rc.addGap(brain.Evidence{ID: "gap-1", Claim: "cloudtrail retention incomplete", Source: brain.EvidenceGap})
	rc.markDataPlanValidated()
	rc.markCoverageUpdated()
	rc.setHuntOutcome(brain.HuntConfirmed, "https://example.test/hunt")

	snap := rc.Snapshot()
	if snap.HuntID != "hunt-123" || snap.HuntQuestion != "Investigate root logins" || snap.HuntType != brain.HuntHypothesis {
		t.Fatalf("snapshot hunt fields = %#v", snap)
	}
	if snap.Author != "alice" {
		t.Fatalf("snapshot author = %q, want alice", snap.Author)
	}
	if !snap.DataPlanValidated || !snap.CoverageUpdated {
		t.Fatalf("snapshot stage flags = data:%v coverage:%v", snap.DataPlanValidated, snap.CoverageUpdated)
	}
	if snap.HuntOutcome != brain.HuntConfirmed || snap.HuntPageURL == "" {
		t.Fatalf("snapshot outcome = (%q, %q)", snap.HuntOutcome, snap.HuntPageURL)
	}
	if len(snap.Evidence) != 1 || len(snap.Gaps) != 1 {
		t.Fatalf("snapshot evidence/gaps = %d/%d", len(snap.Evidence), len(snap.Gaps))
	}

	snap.Evidence[0].Claim = "mutated"
	snap.Gaps[0].Claim = "mutated"
	again := rc.Snapshot()
	if again.Evidence[0].Claim == "mutated" || again.Gaps[0].Claim == "mutated" {
		t.Fatal("snapshot returned aliases to RunContext slices")
	}
}
