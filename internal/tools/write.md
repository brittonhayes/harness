Write a file to the local filesystem, creating parent directories as needed
and overwriting any existing file. Use this to author new detection rules,
runbooks, scripts, or config.

Prefer `edit` for small changes to an existing file so you don't accidentally
discard content. Detection rules are Sigma YAML (`.yml`); after writing one,
validate it with the `validate_detection` tool.

Input:
- `path` (string, required): destination path.
- `content` (string, required): full file contents to write.
