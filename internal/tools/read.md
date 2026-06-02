Read a file from the local filesystem and return its contents with line
numbers (like `cat -n`). Use this to inspect detection rules, logs, configs,
source, and any other text file before editing it.

Input:
- `path` (string, required): absolute or working-directory-relative path.
- `offset` (integer, optional): 1-based line to start from.
- `limit` (integer, optional): max lines to return (default 2000).

Lines longer than 2000 characters are truncated. Prefer reading the specific
range you need for large files.
