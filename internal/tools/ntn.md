Interact with Notion through the official `ntn` CLI. Use this to read and
write runbooks, incident timelines, detection write-ups, and other security
documentation that lives in Notion.

You pass the arguments exactly as you would on the command line, as an array.
Available subcommands (run with `--help` to discover flags):
- `api` — call the public Notion API directly (advanced).
- `pages` — manage pages (create, get, update, list).
- `datasources` — manage data sources / databases.
- `files` — manage file uploads.
- `workers` — manage workers.

Examples:
- List help:            {"args": ["pages", "--help"]}
- Get a page:           {"args": ["pages", "get", "<page-id>"]}
- Create an incident:   {"args": ["pages", "create", "--parent", "<id>", "--title", "INC-1234"]}

Notes:
- The CLI uses the operator's existing `ntn login` session/keychain. If it is
  not logged in, the error is surfaced to you — report it; do not try to log in.
- This tool can modify Notion content, so it is permission-gated.

Input:
- `args` (array of strings, required): arguments passed to `ntn`.
