Find files matching a glob pattern, returning matching paths. Supports `**`
for recursive matching (e.g. `detections/**/*.yml`, `**/*cloudtrail*.yaml`).

Input:
- `pattern` (string, required): the glob pattern.
- `path` (string, optional): base directory to search from (default: working
  directory).

Results are relative to the base directory and capped at 500 entries.
