# CLAUDE.md

## Project overview

`todo` is a personal CLI todo manager that stores items as JSONL (one JSON object per line). It's intentionally simple — three source files, no database, no config file.

## Structure

```
main.go        — CLI entry point, all cobra commands
todo.go        — Todo struct, JSONL read/write, filtering, sorting, recurrence, completion
todo_test.go   — Tests for core logic
```

Keep it to these three files. Don't add packages, directories, or abstractions.

## Build and test

```bash
go build -o todo .
go test ./...
```

## Key design decisions

- **JSONL storage**: One JSON object per line. Append for new items, full rewrite for updates/deletes.
- **Archive on done**: Completed items move from the main file to `<name>.archive.jsonl`. They are removed from the main file, not kept with a done status.
- **Recurrence starts from completion**: `now + interval`, not `previous_due + interval`.
- **Every item has a due date**: Default is 14 days from creation. No items without due dates.
- **Partial ID matching**: Commands accept a prefix of the ULID. The prefix must be unambiguous.
- **Config resolution**: `--file` flag > `TODO_FILE` env var > `~/.todo.jsonl`. No config file.

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `github.com/oklog/ulid/v2` — ID generation
- Standard library for everything else (JSON, file I/O, tabwriter, time)

Don't add dependencies without good reason.

## Conventions

- Statuses: `todo`, `inprogress`, `waiting`, `done`
- Dates: RFC3339 in the JSONL file, `YYYY-MM-DD` in CLI flags and display
- IDs: Full 26-char ULIDs stored, first 12 chars displayed
- Nullable fields (`description`, `category`, `recurrence`): Use `*string` with `nil` for absent values
