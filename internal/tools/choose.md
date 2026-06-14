Ask the operator to choose from a compact set of options.

Use this instead of asking for "A/B/C?" in plain chat when the next step depends
on an operator decision. It renders as an interactive terminal selector in the
REPL:

- `mode: "single"` for exactly one option.
- `mode: "multi"` when the operator may select more than one.
- Mark recommended defaults with `default: true`; the selector preselects them.
- Keep options short but include enough detail for a confident decision.
- Leave `allow_chat` true unless a free-form answer would be unsafe or useless.

The result returns either the selected option IDs, a free-form operator message,
or a cancellation. Treat the returned choice as authoritative.
