Set the inline `runbook:` on a Sigma rule — the response guidance a human
follows when the rule fires. Creates `runbook:` if absent and replaces each
section you provide; omitted sections are left untouched. The rule is
re-validated and only the change plus validation status is returned.

A good runbook makes a detection *respondable*: a responder should be able to
act from the rule alone. Write concrete, ordered steps, not platitudes.

Sections (each a list of step strings; all optional except `path`):
- `path` (string, required): the rule file to edit.
- `triage`: size up the alert — who/what/where, severity drivers.
- `investigate`: related events to pull, how to scope, how to read intent.
- `contain`: actions to stop or limit the activity.
- `escalate`: when to page and to whom.
- `references`: links supporting the response.
