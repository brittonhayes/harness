Search vala's brain for what is already known before opening new work.

`recall` is the brain's dynamic search interface. With a Notion brain, non-empty
queries are answered through the configured Notion MCP search backend; this is
the path for semantic search, fuzzy matching, and "what already relates to this?"
questions. Empty queries are literal recent-row reads, not search.

`recall` returns compact context for artifacts vala has stored — prior `hunts`,
`intel`, `detections`, `coverage`, and `backlog`. It is the read counterpart to
the tools that write the brain, and it is how each hunt compounds on the last
instead of repeating settled ground.

Do not use `ntn` for dynamic brain recall. `ntn` is the Notion CLI for explicit,
literal page/API operations; it is not the Notion MCP server. If you need to know
what prior hunts, intel, coverage, or detections relate to a topic, call
`recall` with a meaningful query.

Run it at the start of the loop, before `open_hunt`:
- Has this behavior already been hunted? If a prior hunt settled the hypothesis,
  say so and stop rather than re-hunting it.
- Does a detection already cover the behavior? If so, the work may be done.
- What intel (indicators, TTPs, actors) already relates? Pull it forward instead
  of rediscovering it.

Inputs:
- `query` (required): free text to search — a behavior, MITRE technique, entity,
  or keyword. Prefer a meaningful query derived from the current hunt. An empty
  query lists the most recent artifacts and does not use search.
- `scope` (optional): `all` (default), `hunts`, `intel`, `detections`, or
  `coverage`, or `backlog`.
- `limit` (optional): max results per scope (default 5).

Read-only: it never modifies the brain, so it needs no approval.
