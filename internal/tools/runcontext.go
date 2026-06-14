package tools

import (
	"sync"

	"github.com/brittonhayes/vala/internal/brain"
)

// RunContext is the per-run state shared by the hunt brain tools (record_finding,
// record_intel, link_artifacts, store_hunt, …). The harness session holds one;
// open_hunt sets its active hunt at runtime.
type RunContext struct {
	// HuntID is the hunt the brain tools write to. open_hunt sets it at runtime;
	// record_finding and store_hunt refuse to run until it is. HuntQuestion
	// carries the question open_hunt opened the hunt with, so store_hunt can title
	// the page.
	HuntID       string
	HuntQuestion string
	Brain        *brain.Client
	// Author identifies the operator this session runs as; the remember tool
	// stamps it onto shared memories so a team can see who learned what.
	Author string

	mu          sync.Mutex
	evidence    []brain.Evidence
	huntType    string           // set by open_hunt (PEAK hunt style)
	gaps        []brain.Evidence // visibility gaps recorded by validate_data
	dataPlanOK  bool             // set by validate_data when telemetry is validated
	coverageOK  bool             // set by update_coverage in the feedback stage
	huntOutcome string           // set by store_hunt
	huntPageURL string           // set by store_hunt
}

// RunContextSnapshot is a consistent, immutable view of the session hunt state
// for UI rendering and tool decisions that need multiple fields.
type RunContextSnapshot struct {
	HuntID       string
	HuntQuestion string
	HuntType     string
	Author       string

	Evidence []brain.Evidence
	Gaps     []brain.Evidence

	DataPlanValidated bool
	CoverageUpdated   bool
	HuntOutcome       string
	HuntPageURL       string
}

// NewRunContext builds a RunContext over the given brain client. A hunt is set
// later by the open_hunt tool via SetHunt.
func NewRunContext(b *brain.Client) *RunContext {
	return &RunContext{Brain: b}
}

// SetAuthor records the operator this session runs as.
func (rc *RunContext) SetAuthor(author string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.Author = author
}

// SetHunt records the active hunt opened by the open_hunt tool so the hunt
// brain tools (record_finding, store_hunt) have a hunt to write to. huntType is
// the PEAK hunt style the hunt was opened with.
func (rc *RunContext) SetHunt(huntID, question, huntType string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.HuntID = huntID
	rc.HuntQuestion = question
	rc.huntType = huntType
	// A fresh hunt resets the per-hunt stage accumulators so one hunt's data plan
	// or coverage update never leaks into the next.
	rc.evidence = nil
	rc.gaps = nil
	rc.dataPlanOK = false
	rc.coverageOK = false
	rc.huntOutcome = ""
	rc.huntPageURL = ""
}

// Snapshot returns a consistent copy of all UI-relevant run state. It takes the
// mutex once so a frame cannot mix fields from two different hunt states.
func (rc *RunContext) Snapshot() RunContextSnapshot {
	if rc == nil {
		return RunContextSnapshot{}
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.snapshotLocked()
}

func (rc *RunContext) snapshotLocked() RunContextSnapshot {
	return RunContextSnapshot{
		HuntID:            rc.HuntID,
		HuntQuestion:      rc.HuntQuestion,
		HuntType:          rc.huntType,
		Author:            rc.Author,
		Evidence:          cloneEvidence(rc.evidence),
		Gaps:              cloneEvidence(rc.gaps),
		DataPlanValidated: rc.dataPlanOK,
		CoverageUpdated:   rc.coverageOK,
		HuntOutcome:       rc.huntOutcome,
		HuntPageURL:       rc.huntPageURL,
	}
}

func cloneEvidence(in []brain.Evidence) []brain.Evidence {
	out := make([]brain.Evidence, len(in))
	copy(out, in)
	return out
}

func (rc *RunContext) addEvidence(e brain.Evidence) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.evidence = append(rc.evidence, e)
}

// Evidence returns the findings recorded so far.
func (rc *RunContext) Evidence() []brain.Evidence {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return cloneEvidence(rc.evidence)
}

// HuntType returns the PEAK hunt style the active hunt was opened with.
func (rc *RunContext) HuntType() string {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.huntType
}

// markDataPlanValidated records that the data-validation stage confirmed the
// telemetry needed to test the hypothesis is present.
func (rc *RunContext) markDataPlanValidated() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.dataPlanOK = true
}

// addGap records a visibility gap surfaced by the data-validation stage.
func (rc *RunContext) addGap(e brain.Evidence) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.gaps = append(rc.gaps, e)
}

// Gaps returns the visibility gaps recorded so far.
func (rc *RunContext) Gaps() []brain.Evidence {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return cloneEvidence(rc.gaps)
}

// DataPlanValidated reports whether the data-validation stage has run.
func (rc *RunContext) DataPlanValidated() bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.dataPlanOK
}

// markCoverageUpdated records that the feedback stage updated the coverage map.
func (rc *RunContext) markCoverageUpdated() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.coverageOK = true
}

// CoverageUpdated reports whether the feedback stage has updated the coverage map.
func (rc *RunContext) CoverageUpdated() bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.coverageOK
}

func (rc *RunContext) setHuntOutcome(outcome, pageURL string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.huntOutcome = outcome
	rc.huntPageURL = pageURL
}

// HuntOutcome returns the outcome status and page URL set by store_hunt, or
// empty strings if the hunt has not been stored.
func (rc *RunContext) HuntOutcome() (outcome, pageURL string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.huntOutcome, rc.huntPageURL
}
