Interact with Notion through the official `ntn` CLI. This is a literal Notion
CLI/API wrapper, not the Notion MCP server and not the brain's semantic recall
path. Use it to read and write specific runbooks, incident timelines, detection
write-ups, and other security documentation when you know the page, data source,
or API operation you want.

For dynamic questions about what vala's brain already knows — prior hunts,
intel, coverage, detections, backlog items, or related work — use `recall`
instead. With a Notion brain, `recall` routes non-empty searches through Notion
MCP.

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
