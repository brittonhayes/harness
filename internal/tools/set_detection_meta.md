Set scalar metadata on a Sigma rule without rewriting the file. Pass any subset
of fields in one call; each is created if absent or replaced if present. The rule
is re-validated automatically and the tool reports only what changed plus the
validation status — it does not echo the whole file.

Prefer this over `write`/`edit` for metadata: it preserves the rule's existing
comments and key order and re-validates in one step.

Fields (all optional except `path`):
- `path` (string, required): the rule file to edit.
- `title` (string): rule title.
- `id` (string): rule UUID. Pass `"generate"` to mint a fresh UUID v4.
- `status` (string): experimental | test | stable | deprecated | unsupported.
- `description` (string): what the rule detects and why it matters.
- `author` (string).
- `date` (string): YYYY-MM-DD.
- `level` (string): informational | low | medium | high | critical.
