Edit the `detection` block of a Sigma rule: add or replace a named search
identifier (a "selection"), remove one, and/or set the `condition`. Creates the
`detection` block if absent. The rule is re-validated and only the change plus
validation status is returned.

A search identifier is a map of `field: value` entries (all must match — AND);
a list value means OR; append a modifier to the field name with `|`, e.g.
`eventName|contains`. The `condition` combines identifiers with `and`/`or`/`not`,
parentheses, and quantifiers like `1 of selection*` or `all of them`.

Set a selection and condition:
```json
{"path": "detections/r.yml",
 "selection": "selection",
 "fields": {"eventSource": "iam.amazonaws.com", "eventName": ["CreateUser", "CreateAccessKey"]},
 "condition": "selection and not filter_known_admins"}
```
Remove a selection: `{"path": "...", "selection": "filter_known_admins", "remove": true}`

Inputs:
- `path` (string, required): the rule file to edit.
- `selection` (string): identifier name to set or remove.
- `fields` (object): the field map for that selection (required unless removing).
- `remove` (boolean): remove the named selection instead of setting it.
- `condition` (string): the condition expression.
