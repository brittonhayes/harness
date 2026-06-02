Consult curated, gold-standard Sigma detections before authoring your own. These
exemplars (adapted from SigmaHQ) show what a respondable, review-proof rule looks
like: tight conditions, commented filter exclusions, populated `falsepositives`,
an inline `runbook:`, and executable `tests:`. Read one before writing a rule for
a similar log source — match its shape.

Two usage modes:
- `{"list": true}` — the index: each reference's name, title, level, MITRE
  techniques, and one-line use case. (This is also the default with no input.)
- `{"name": "aws_cloudtrail_disable_logging"}` — the full YAML of one exemplar.

The exemplars model two optional, schema-valid custom fields you should reuse:
- `runbook:` — a map with `triage`, `investigate`, `contain`, `escalate` (each a
  string or list of steps), plus optional `references`. Inline response guidance.
- `tests:` — a list of `{name, event, match}` cases the `test_detection` tool
  executes, so the rule's logic is verifiable, not just schema-valid.

Input:
- `list` (boolean): list all reference detections.
- `name` (string): return one reference detection's full YAML by name.
