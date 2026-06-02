# harness

An agentic harness for **security detection & response (D&R)** engineering.

`harness` drives an LLM agent (Anthropic Claude) that investigates suspicious
activity, authors and validates [Sigma](https://sigmahq.io) detection rules,
runs shell/file tools, and documents its work in Notion via the `ntn` CLI. It
ships as a single static Go binary with **no external detection toolchain** —
Sigma rules are validated *and unit-tested* natively and offline, inside the
binary.

The agent is framed as a detection engineer and given a focused toolset for the
whole detection lifecycle: read the logs, study gold-standard exemplars, author
a rule field by field, **prove it with embedded test events**, and write up the
runbook.

> The architecture follows [charmbracelet/crush](https://github.com/charmbracelet/crush)
> (one `Tool` type + one embedded `.md` description per tool, a permission gate,
> sessions) and the "small extensible core" stance of
> [earendil-works/pi](https://github.com/earendil-works/pi).

## Install

```sh
go install github.com/brittonhayes/harness/cmd/harness@latest
export ANTHROPIC_API_KEY=sk-ant-...
```

Or build from source:

```sh
git clone https://github.com/brittonhayes/harness
cd harness
go build -o harness ./cmd/harness
```

## Usage

Interactive session (REPL):

```sh
harness
```

One-shot, non-interactive:

```sh
harness run "validate and test every rule in detections/, and report failures"
harness run --yes "author a Sigma rule for an attacker disabling GuardDuty: \
  study the reference rules first, add a runbook and two tests, then validate it"
```

Flags: `--model <id>`, `--permission ask|allow|deny`.

## How it works

The agent works against a real workstation through a small set of tools. For
detection work the loop is: **study → author → validate → test → document.**

1. **Study.** `reference_detection` surfaces curated, gold-standard Sigma rules
   (adapted from [SigmaHQ](https://github.com/SigmaHQ/sigma)) embedded in the
   binary — each one carries an inline runbook and executable tests so the agent
   learns the shape of a respondable, review-proof rule before writing its own.
2. **Author.** Instead of rewriting whole YAML files, the agent edits one field
   at a time with the `set_detection_*` / `*_detection_*` tools. These preserve
   the rule's comments and key order and re-validate after every change.
3. **Validate.** `validate_detection` checks rules against the official Sigma
   JSON schema, embedded in the binary — no `sigma-cli`, `yq`, or Python.
4. **Test.** Rules carry inline `tests:` (sample events + expected outcome).
   `test_detection` runs them through a built-in Sigma evaluation engine, so a
   rule's *logic* is verified, not just its schema.
5. **Document.** `ntn` drives the official Notion CLI for runbooks and incident
   write-ups.

## Tools

| Tool | Read-only | Purpose |
|------|-----------|---------|
| `bash` | no | Run shell commands (git, jq, `aws`, …). |
| `read` / `write` / `edit` | read / no / no | File operations. |
| `ls` / `glob` / `grep` | yes | Navigate and search the workspace. |
| `reference_detection` | yes | Browse curated gold-standard Sigma exemplars. |
| `validate_detection` | yes | Validate Sigma rules against the embedded schema (offline). |
| `test_detection` | yes | Run a rule's inline `tests:` through the evaluation engine. |
| `set_detection_meta` | no | Set scalar metadata (title, id, status, level, …). |
| `set_detection_logsource` | no | Set the `logsource` block. |
| `edit_detection_logic` | no | Manage search identifiers and the `condition`. |
| `manage_detection_list` | no | Add/remove `references`, `falsepositives`, `tags`, `fields`. |
| `set_detection_runbook` | no | Set the inline response `runbook:`. |
| `manage_detection_tests` | no | Add/remove inline `tests:` cases. |
| `ntn` | no | Drive the official Notion CLI for runbooks & incident docs. |

The field-editing tools all funnel through one load → mutate → validate → write
pipeline: they change a single field, keep the file's comments intact, and
report only what changed plus the validation status — never the whole file.

## Detections

Detection rules are [Sigma](https://sigmahq.io) YAML files under `detections/`.
Sigma is the vendor-neutral detection-as-code standard; rules convert to many
SIEM backends (and platforms like [scanner.dev](https://scanner.dev) can ingest
Sigma directly), so you write once and stay portable.

A rule requires at least `title`, `logsource`, and `detection` (with a
`condition`). Harness rules also model two optional, schema-valid custom fields:

- **`runbook:`** — inline response guidance (`triage`, `investigate`, `contain`,
  `escalate`, `references`) so a detection is *respondable* from the rule alone.
- **`tests:`** — a list of `{name, event, match}` cases the evaluation engine
  runs, so a rule's logic is verifiable.

```yaml
detection:
  selection:
    eventName: ConsoleLogin
    userIdentity.type: Root
  condition: selection
tests:
  - name: root console login fires
    event: { eventName: ConsoleLogin, userIdentity.type: Root }
    match: true
  - name: iam user login is ignored
    event: { eventName: ConsoleLogin, userIdentity.type: IAMUser }
    match: false
```

See [`detections/aws_root_account_usage.yml`](detections/aws_root_account_usage.yml)
for a complete example with a runbook and tests, and the embedded exemplars under
[`internal/reference/sigma/`](internal/reference/sigma).

Validate and test the whole directory at once:

```sh
harness run "validate and test every rule in detections/"
```

### Built-in evaluation engine

The `test_detection` tool and the inline `tests:` field are backed by a
pragmatic, offline Sigma evaluation engine (`internal/detect`). It supports the
common modifiers (`contains`, `startswith`, `endswith`, `all`, `re`, `cidr`,
`lt|lte|gt|gte`), `*`/`?` wildcards, dotted/nested field lookups, and the
`1 of` / `all of` condition quantifiers. Unsupported constructs (e.g. aggregation
`| count() …` conditions) are reported as such rather than silently passing.

## Permissions

Every **non-read-only** tool call is gated.

- `ask` (default) — prompt the operator for each call. Answer `a` to allowlist a
  tool for the session.
- `allow` — auto-approve (trusted, unattended runs; `harness run --yes`).
- `deny` — block all writes (investigation / dry-run only).

Read-only tools (`read`, `ls`, `glob`, `grep`, `reference_detection`,
`validate_detection`, `test_detection`) always run.

## Configuration

Settings layer (lowest priority first): built-in defaults →
`~/.config/harness/config.json` → `./.harness.json` → environment variables
(`ANTHROPIC_API_KEY`, `HARNESS_MODEL`, `HARNESS_PERMISSION`).

```json
{
  "model": "claude-opus-4-8",
  "max_tokens": 8192,
  "permission": "ask",
  "allowlist": ["read", "ls", "glob", "grep"],
  "detections_dir": "detections",
  "max_steps": 50
}
```

Session transcripts are written to `~/.local/share/harness/sessions/`.

## Roadmap

The tool registry (`internal/tools/default.go`) is the single extension point.
Planned next:

- An `aws` tool for cloud investigation & response (read verbs gated tighter).
- Sigma → backend query conversion (e.g. via pySigma) for export to a SIEM.
- Agent Skills (`SKILL.md`) for reusable D&R playbooks.

## Development

```sh
go build ./...
go vet ./...
go test ./...

# build / run the binary
go build -o harness ./cmd/harness
./harness version
```

CI (GitHub Actions) runs build, vet, `go test -race`, and a `gofmt` check on
every push and pull request.

## License

[MIT](LICENSE) © Britton Hayes
