Add or remove a single item in one of a Sigma rule's list fields, without
rewriting the file. The list is created on first add. The rule is re-validated
and only the change plus validation status is returned. You may add and remove
in one call (remove is applied first).

Editable fields:
- `references` — URLs backing the rule (ATT&CK technique pages, vendor docs).
- `falsepositives` — known benign causes a responder should rule out.
- `tags` — MITRE ATT&CK and other tags, e.g. `attack.t1078.004`.
- `fields` — log fields a responder should pull when the rule fires.

Inputs:
- `path` (string, required): the rule file to edit.
- `field` (string, required): references | falsepositives | tags | fields.
- `add` (string): value to append.
- `remove` (string): value to remove (must match exactly).
