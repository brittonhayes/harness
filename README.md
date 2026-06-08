# vala

An agentic security harness that hunts threats, builds detections, and works
alerts — as a single Go binary.

A SIEM is something you search by hand. vala works the other way: it runs
hypothesis-driven hunts, records the results in Notion, and turns them into
detections. Hand it an alert and it investigates, proposes actions, and writes an
auditable case — without taking a destructive action you didn't approve.

It runs on Anthropic's Claude and needs no external detection toolchain. Sigma
rules are validated and unit-tested natively and offline, inside the binary — no
`sigma-cli`, no `yq`, no Python.

## Quickstart

```sh
go install github.com/brittonhayes/vala/cmd/vala@latest
export ANTHROPIC_API_KEY=sk-ant-...
```

There is one surface: an interactive session with a toolbox. Start it and ask it
to do the work:

```sh
vala
```

```
› hunt whether anyone disabled GuardDuty in the last 24h, and store the hunt
› record the TTP attack.t1562.001 and link it to that hunt
› author a Sigma rule for an attacker disabling GuardDuty, with a runbook and tests
› work the alert in tests/ops/sample_alert.json
```

Run a one-shot task non-interactively (same toolbox, no TTY):

```sh
vala run "validate and test every rule in my detections directory, and report failures"
```

Common flags: `--model <id>`, `--permission ask|allow|deny`, `--yes`.

> **Build from source:** `git clone https://github.com/brittonhayes/vala && cd
> vala && go build -o vala ./cmd/vala`

## What it does

**Hunt threats.** Point it at a question and it states a hypothesis, explores
read-only data sources, records each fact as an immutable Finding pointer, and
stores the hunt — question, hypothesis, findings, and a Confirmed / Refuted /
Inconclusive verdict — into Notion. It records intelligence (indicators, TTPs,
actors) and links intel, hunts, alerts, and detections together. A hunt that
confirms a TTP flows straight into a detection.

**Author detections.** A tight loop — study → author → validate → test →
document. It studies curated reference Sigma rules embedded in the binary, edits
rules one field at a time (preserving comments and key order), validates against
the official Sigma JSON schema offline, and runs each rule's inline `tests:`
through a built-in evaluation engine. vala ships no detections of its own — point
it at your directory with `detections_dir` (default `detections`).

**Respond to alerts.** Hand it an alert and `open_case` drives it through a
phase-separated governance loop:

```
plan ─► evidence ─► propose ─► approval ─► execute ─► report
```

Each phase exposes a smaller set of tools. Investigation is read-only and records
immutable Evidence pointers; the agent proposes Actions but can't execute them; a
human or policy approves each one; only approved actions run, each at most once;
the final case page is rejected unless every claim cites evidence. The result is
an auditable case record in Notion. Without configured Notion database IDs, vala
runs in local mode and prints artifacts to stdout.

## How safety is enforced

Trust in the autonomous loop comes from code-level controls, not from asking the
model nicely in a prompt:

1. **Per-phase tool exposure.** Write tools don't exist for the agent during
   investigation, so it can't act early — and return-channel prompt injection
   can't reach a write tool.
2. **The permission gate.** `permission.Gate.Decide` is the authoritative
   backstop: only approved actions run.
3. **Evidence lint.** The case page is rejected unless every claim cites evidence.

Governance is driven by editable YAML under [`policies/`](policies): `tools.yaml`
classifies each tool and lists per-environment hard-deny rules (unknown tools fail
closed), and `decision.yaml` controls which actions need approval and which must
cite evidence.

`vala harness` replays adversarial scenario fixtures (`tests/`) through the real
governance machine in a deterministic, no-LLM mode and scores each on five safety
dimensions (approval compliance, no scope creep, evidence-backed claims, injection
resistance, schema validity). It exits non-zero on any failure or regression
versus a committed baseline, so a weakening change is caught in CI:

```sh
vala harness --fixtures tests --out report.json --baseline runner/baseline.json
```

## Writing detections

Rules are [Sigma](https://sigmahq.io) YAML — the vendor-neutral
detection-as-code standard, portable across SIEM backends. A rule needs at least
`title`, `logsource`, and `detection`. vala rules also model two optional,
schema-valid custom fields:

- **`runbook:`** — inline response guidance (`triage`, `investigate`, `contain`,
  `escalate`, `references`) so a detection is respondable from the rule alone.
- **`tests:`** — `{name, event, match}` cases the evaluation engine runs, so a
  rule's logic is verifiable.

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

See the embedded reference rules under
[`internal/reference/sigma/`](internal/reference/sigma) for complete examples. The
offline evaluation engine (`internal/detect`) supports the common modifiers
(`contains`, `startswith`, `endswith`, `all`, `re`, `cidr`, `lt|lte|gt|gte`),
`*`/`?` wildcards, dotted field lookups, and the `1 of` / `all of` quantifiers.

## Configuration

Settings layer (lowest priority first): built-in defaults →
`~/.config/vala/config.json` → `./.vala.json` → environment variables
(`ANTHROPIC_API_KEY`, `VALA_MODEL`, `VALA_PERMISSION`, `VALA_ENV`,
`SLACK_WEBHOOK_URL`).

```json
{
  "model": "claude-opus-4-8",
  "permission": "ask",
  "detections_dir": "detections",
  "env": "dev",
  "notion": {
    "alerts": "", "cases": "", "evidence": "", "actions": "", "runs": "",
    "hunts": "", "intel": "", "detections": "", "case_page_parent": ""
  }
}
```

`env` selects the policy environment (`dev`/`prod`). Notion database IDs enable
real Notion writes; leave them empty to run in local mode. Every non-read-only
tool call is gated by `--permission`: `ask` (default) prompts per call, `allow`
auto-approves for unattended runs, `deny` blocks all writes.

## Development

```sh
go build ./...
go vet ./...
go test ./...
go run ./cmd/vala harness --fixtures tests
```

CI runs build, vet, `go test -race`, the adversarial harness (diffed against
`runner/baseline.json`), and a `gofmt` check on every push and pull request.

The architecture follows [charmbracelet/crush](https://github.com/charmbracelet/crush)
(one `Tool` type + one embedded `.md` description per tool, a permission gate,
sessions). The tool registry (`internal/tools/default.go`) is the single
extension point.

## License

[MIT](LICENSE) © Britton Hayes
