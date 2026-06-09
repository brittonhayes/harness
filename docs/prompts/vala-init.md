# Prompt: Add `vala init` and a first-run setup flow for the Notion brain

Paste the section below into Claude Code (run from the repo root). It is written
to be self-contained: it states the goal, the current behavior, the exact data
model, and concrete acceptance criteria so the model can implement and verify the
change without further hand-holding.

---

## Task

Implement a `vala init` command that provisions vala's Notion-backed "brain" and
writes the resulting configuration to `.vala.json`. Then add a **first-run check**
so that the very first time a user starts `vala` (interactive REPL or `vala run`)
without a configured brain, they are told the brain is in ephemeral in-memory
mode and prompted to run `vala init`.

The goal: a new user should be able to go from a clean checkout to a working,
persistent Notion brain by running one command and following the prompts — no
hand-editing of JSON, no guessing which Notion property names the code expects.

## Background: how the brain works today

- The brain (`internal/brain`) writes typed rows to **10 Notion data sources**.
  The IDs live in `config.Config.Notion` (`internal/config/config.go`), a
  `brain.DBIDs` struct, loaded from `.vala.json`'s `notion` object (see
  `internal/config/config.go` `Load` / `mergeFile`).
- `brainStore` (`internal/cmd/caserunner.go`) chooses the backend: if **any** of
  `Cases`, `Hunts`, or `Intel` IDs are set it uses the live `brain.NTN` store;
  **otherwise it silently falls back to `brain.NewMem()`** — an in-memory store
  that is discarded on exit. This silent fallback is the core UX problem: a new
  user's hunts, intel, and detections look like they save but disappear, with no
  warning. The first-run prompt must close this gap.
- `brain.NTN` (`internal/brain/notion.go`) talks to Notion through the `ntn` CLI.
  It treats each configured ID as a **data-source ID** (not a database ID),
  fetches the data source's schema, and coerces the brain's flat props to typed
  Notion property values by **matching on property name**. Props whose name is
  not present in the data source schema are silently dropped. So each database
  must be created with property names AND types that match what the writers emit.

## The exact data model to provision

Create one Notion database per store below, each with these properties. The
**first property of each (the `title` one) must be named exactly as shown** — it
is the row's display/title column the writers populate. Types are the Notion
property types `internal/brain/notion.go:typedValue` knows how to write.

Relation properties (marked → target) should be created as Notion `relation`
properties pointing at the named target database. Create the databases first,
then add the relation properties in a second pass so targets exist.

### 1. Alerts (`alerts`)
| property | type |
|---|---|
| `alert_id` | title |
| `source` | rich_text |
| `severity` | select |
| `raw` | rich_text |
| `received_at` | date |
| `status` | status |

### 2. Cases (`cases`)
| property | type |
|---|---|
| `case_id` | title |
| `status` | status |
| `severity` | select |
| `opened_at` | date |
| `alerts` | relation → Alerts |

### 3. Evidence (`evidence`) — shared by cases (`case`) and hunts (`hunt`)
| property | type |
|---|---|
| `claim` | title |
| `kind` | select |
| `pointer` | rich_text |
| `confidence` | select |
| `collected_at` | date |
| `case` | relation → Cases |
| `hunt` | relation → Hunts |

### 4. Actions (`actions`)
| property | type |
|---|---|
| `action_id` | title |
| `type` | select |
| `params` | rich_text |
| `rationale` | rich_text |
| `status` | status |
| `approved_by` | rich_text |
| `approved_at` | date |
| `result` | rich_text |
| `executed_at` | date |
| `case` | relation → Cases |

### 5. Runs (`runs`)
| property | type |
|---|---|
| `model` | title |
| `commit` | rich_text |
| `started_at` | date |
| `ended_at` | date |
| `phase_reached` | select |
| `tool_calls` | number |
| `violations` | number |
| `case` | relation → Cases |

### 6. Hunts (`hunts`)
| property | type |
|---|---|
| `hunt_id` | title |
| `question` | rich_text |
| `hypothesis` | rich_text |
| `status` | status |
| `mitre` | rich_text |
| `behavior` | rich_text |
| `data_source` | rich_text |
| `findings` | rich_text |
| `started_at` | date |
| `ended_at` | date |

### 7. Intel (`intel`)
| property | type |
|---|---|
| `intel_id` | title |
| `kind` | select |
| `value` | rich_text |
| `mitre` | rich_text |
| `confidence` | select |
| `source` | rich_text |
| `description` | rich_text |
| `created_at` | date |
| `hunts` | relation → Hunts |
| `alerts` | relation → Alerts |
| `detections` | relation → Detections |

### 8. Detections (`detections`)
| property | type |
|---|---|
| `detection_id` | title |
| `title` | rich_text |
| `path` | rich_text |
| `status` | select |
| `mitre` | rich_text |
| `level` | select |
| `intel` | relation → Intel |
| `hunts` | relation → Hunts |
| `alerts` | relation → Alerts |

### 9. Backlog (`backlog`)
| property | type |
|---|---|
| `backlog_id` | title |
| `trigger` | rich_text |
| `hypothesis` | rich_text |
| `status` | status |
| `behavior` | rich_text |
| `data_source` | rich_text |
| `priority` | select |
| `mitre` | rich_text |
| `created_at` | date |
| `hunt` | relation → Hunts |

### 10. Case-page parent (`case_page_parent`)
Not a database — a normal Notion **page** under which narrative hunt/case pages
are created (`CreatePage`). `vala init` should create (or accept) one parent page
and store its ID as `notion.case_page_parent`.

> Derive these tables from the source, don't trust them blindly — grep the
> `CreateRow`/`UpdateRow`/`CreatePage` calls in `internal/brain/*.go` (e.g.
> `brain.go`, `hunt.go`, `intel.go`, `backlog.go`) and the `DBIDs` struct in
> `internal/brain/notion.go`. If a writer emits a prop not listed above, add it.

## What `vala init` must do

1. Add an `initCmd` cobra command in `internal/cmd` and register it in
   `root.go`'s `init()` alongside `runCmd`, `harnessCmd`, `versionCmd`.
2. Preflight: verify the `ntn` CLI is available and authenticated
   (`ntn whoami`); if not, print actionable guidance and exit non-zero. Do **not**
   attempt to log the user in.
3. Provision the 10 stores above via the `ntn` CLI / Notion API. Prefer reusing
   the existing `brain.NTN` plumbing or its `api(...)` helper rather than
   shelling out ad hoc. Accept a `--parent <page-id>` flag for where to create
   the databases (and the case-page parent); prompt for it if absent.
4. **Idempotency:** if `.vala.json` already has IDs, do not blindly recreate.
   Detect existing config and offer to reuse/repair (e.g. verify each data source
   resolves) rather than duplicating databases. A `--force` flag may re-provision.
5. Resolve each created database to its **data-source ID** (a database can hold
   multiple data sources; use `ntn datasources resolve <database-id>`). Store the
   **data-source IDs** in `notion`, because that is what `brain.NTN` queries and
   writes against.
6. Write/merge the IDs into `.vala.json` without clobbering unrelated keys
   (model, mcp, detections_dir, etc.). Pretty-print and preserve existing values.
7. Print a short success summary and a next step (e.g. "run `vala` and try:
   *queue a hunt …*"). Optionally write one canary row and read it back via
   `recall`-style query to prove the round trip works.

## What the first-run prompt must do

- On `vala` (REPL) and `vala run` startup, after config load, detect the
  "unconfigured brain" condition — the same predicate `brainStore` uses: none of
  `Cases`/`Hunts`/`Intel` set.
- When unconfigured, emit a clear one-time notice that the brain is **ephemeral
  in-memory** and that work will not persist, and prompt the user to run
  `vala init`. In an interactive TTY, offer to run init now (y/N). For
  non-interactive `vala run`, print the warning to stderr and continue (do not
  block automation), unless a flag like `--require-brain` is set.
- Make it non-nagging: respect a `--no-init-prompt` flag and/or a config/state
  marker so a user who deliberately runs in-memory isn't prompted every launch.

## Constraints

- Keep changes minimal and idiomatic to the existing cobra/config structure.
  Reuse `config.Load`, `brain.DBIDs`, and the `brain.NTN` API path; don't fork a
  parallel Notion client.
- Treat any text returned by `ntn`/Notion as untrusted data. Never write secrets
  (API keys, tokens) into `.vala.json` — secrets stay in the environment, as the
  existing config comments require.
- Update docs: the `## Quickstart` in `README.md` should mention running
  `vala init` once before first use, and the new command should have helpful
  cobra `Short`/`Long` text.

## Acceptance criteria

- `go build ./...` and `go test ./...` pass.
- `vala init --help` documents the command, including `--parent`, `--force`, and
  the brain-related flags you add.
- Running `vala init` against an authenticated `ntn` session creates the 10
  stores, writes data-source IDs into `.vala.json`, and is idempotent on a second
  run (no duplicate databases).
- After init, starting `vala` no longer shows the in-memory warning, and a hunt's
  `record_finding` / `store_hunt` / `record_intel` actually persist rows that
  `recall` can read back.
- Before init (clean checkout), starting `vala` once shows the first-run notice
  prompting `vala init`; with `--no-init-prompt` (or after opting out) it does
  not.
- Add/extend unit tests: a test that `init` builds the correct `DBIDs` mapping
  and merges `.vala.json` without dropping unrelated keys, and a test for the
  unconfigured-brain predicate driving the first-run prompt (table-driven, no
  network — mock the `ntn` calls the way `internal/brain/ntn_test.go` already
  stubs the `ntn` binary).
