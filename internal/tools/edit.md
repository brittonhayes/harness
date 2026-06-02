Edit a file by replacing an exact string with a new one. This is the safe way
to make a targeted change without rewriting the whole file.

Rules:
- `old_string` must appear EXACTLY once in the file (including whitespace and
  indentation). If it appears zero or many times, the edit fails — add
  surrounding context to make it unique.
- To create a new file, use `write` instead.
- Set `replace_all` to true to replace every occurrence (e.g. renaming a field
  across a detection rule).

Input:
- `path` (string, required): file to edit.
- `old_string` (string, required): text to find.
- `new_string` (string, required): replacement text.
- `replace_all` (boolean, optional): replace all occurrences.
