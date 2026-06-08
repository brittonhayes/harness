package tools

import (
	"context"
	"sync"

	"github.com/brittonhayes/vala/internal/brain"
	"github.com/brittonhayes/vala/internal/governance"
	"github.com/brittonhayes/vala/internal/policy"
)

// RunContext is the per-run state shared by the case and hunt brain tools
// (record_evidence, propose_action, submit_for_approval, write_case_page,
// record_finding, store_hunt, …). The harness session holds one for hunt/intel
// work; each governed case opens its own inside the respond engine.
type RunContext struct {
	Env    string
	CaseID string
	// HuntID is set instead of CaseID when this RunContext drives a hunt. In the
	// unified harness it is set at runtime by the open_hunt tool; record_finding
	// and store_hunt refuse to run until it is. HuntQuestion carries the question
	// open_hunt opened the hunt with, so store_hunt can title the page.
	HuntID       string
	HuntQuestion string
	Brain        *brain.Client
	Ledger       *governance.Ledger
	Policy       *policy.Set

	// Notifier sends comms for slack_notify; nil falls back to a no-op record.
	Notifier Notifier

	mu          sync.Mutex
	evidence    []brain.Evidence
	actions     map[string]*brain.Action
	rowIDs      map[string]string // action ID -> brain Actions row ID
	submitted   bool
	huntOutcome string // set by store_hunt
	huntPageURL string // set by store_hunt
}

// NewRunContext builds a RunContext. caseID is set for a governed case; the
// harness session passes an empty caseID and sets a hunt later via SetHunt.
func NewRunContext(env, caseID string, b *brain.Client, led *governance.Ledger, pol *policy.Set) *RunContext {
	return &RunContext{
		Env:     env,
		CaseID:  caseID,
		Brain:   b,
		Ledger:  led,
		Policy:  pol,
		actions: map[string]*brain.Action{},
		rowIDs:  map[string]string{},
	}
}

// SetHunt records the active hunt opened by the open_hunt tool so the hunt
// brain tools (record_finding, store_hunt) have a hunt to write to.
func (rc *RunContext) SetHunt(huntID, question string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.HuntID = huntID
	rc.HuntQuestion = question
}

func (rc *RunContext) addEvidence(e brain.Evidence) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.evidence = append(rc.evidence, e)
}

// Evidence returns the evidence collected so far.
func (rc *RunContext) Evidence() []brain.Evidence {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	out := make([]brain.Evidence, len(rc.evidence))
	copy(out, rc.evidence)
	return out
}

func (rc *RunContext) knownEvidence(id string) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	for _, e := range rc.evidence {
		if e.ID == id {
			return true
		}
	}
	return false
}

func (rc *RunContext) addAction(a *brain.Action, rowID string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.actions[a.ID] = a
	rc.rowIDs[a.ID] = rowID
}

// SetActionStatus updates a proposed action's status both in memory (so the case
// page reflects it) and on its Actions row in the brain.
func (rc *RunContext) SetActionStatus(ctx context.Context, actionID, status, by, result string) {
	rc.mu.Lock()
	a := rc.actions[actionID]
	if a != nil {
		a.Status = status
	}
	rowID := rc.rowIDs[actionID]
	rc.mu.Unlock()
	if rowID != "" {
		_ = rc.Brain.UpdateActionStatus(ctx, rowID, status, by, result)
	}
}

// Actions returns the proposed actions collected so far.
func (rc *RunContext) Actions() []brain.Action {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	out := make([]brain.Action, 0, len(rc.actions))
	for _, a := range rc.actions {
		out = append(out, *a)
	}
	return out
}

func (rc *RunContext) markSubmitted() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.submitted = true
}

// Submitted reports whether the model has submitted its proposals for approval.
func (rc *RunContext) Submitted() bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.submitted
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

// Notifier sends a notification for an approved action.
type Notifier interface {
	Notify(message string) (pointer string, err error)
}
