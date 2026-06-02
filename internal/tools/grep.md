Search file contents for a regular expression and return matching lines with
their file path and line number. Uses ripgrep (`rg`) when available, otherwise
a built-in Go regex walk.

Input:
- `pattern` (string, required): regular expression to search for.
- `path` (string, optional): file or directory to search (default: working
  directory).
- `glob` (string, optional): only search files matching this glob (e.g.
  `*.yml`).

Output is capped; narrow your pattern or path if you hit the limit.
