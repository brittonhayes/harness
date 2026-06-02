Add or remove an inline `tests:` case on a Sigma rule. Each case is a sample
event plus the expected match outcome; the `test_detection` tool later runs them
through the evaluation engine. Adding a case with an existing name replaces it,
so this is safe to call repeatedly. The rule is re-validated and only the change
plus validation status is returned.

Give every rule at least one should-match and one should-NOT-match case — that
is what proves the logic, and a benign case is what guards against a noisy rule.

Add a case:
```json
{"path": "detections/r.yml",
 "name": "root console login fires",
 "event": {"eventName": "ConsoleLogin", "userIdentity.type": "Root"},
 "match": true}
```
Remove a case: `{"path": "...", "name": "root console login fires", "remove": true}`

Inputs:
- `path` (string, required): the rule file to edit.
- `name` (string, required): the case name (to add, or to identify for removal).
- `event` (object): the sample event (required unless removing). Keys may be
  dotted paths like `userIdentity.type` or nested maps.
- `match` (boolean): expected outcome — true if the rule should fire.
- `remove` (boolean): remove the named case instead of adding it.
