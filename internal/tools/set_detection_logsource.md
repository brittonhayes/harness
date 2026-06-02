Set the `logsource` block of a Sigma rule — the fields that identify which data
the rule runs against. Creates `logsource` if absent and sets each provided
field; unspecified fields are left untouched. The rule is re-validated and only
the change plus validation status is returned.

A logsource is usually `product` + `service` (e.g. aws/cloudtrail) and/or a
`category` (e.g. process_creation). `definition` is a free-text note describing
the exact logging that must be enabled for the rule to fire.

Fields (all optional except `path`):
- `path` (string, required): the rule file to edit.
- `product` (string): e.g. aws, windows, okta, azure.
- `service` (string): e.g. cloudtrail, security, signinlogs.
- `category` (string): e.g. process_creation, network_connection.
- `definition` (string): note on the exact data required.
