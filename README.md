# todo

A personal CLI todo manager backed by JSONL.

The data file is a single `.jsonl` file — one JSON object per line. Point it at Dropbox, iCloud, or wherever you want.

## Install

```bash
go build -o todo .
```

## Configuration

The data file path is resolved in order:

1. `--file` flag
2. `TODO_FILE` environment variable
3. `~/.todo.jsonl` (default)

```bash
# Use a flag
todo --file ~/Dropbox/todo.jsonl list

# Or set the env var
export TODO_FILE=~/Dropbox/todo.jsonl
```

## Usage

### Add items

```bash
todo add "Buy groceries"
todo add "Write report" --category work --due 2026-03-01
todo add "Clean house" --category personal --desc "Deep clean kitchen and bath"
todo add "Water plants" --recurrence 3d --category home
todo add "Weekly review" --category work --recurrence 1w --due 2026-02-21
```

Without `--due`, items default to 14 days from now. Recurrence intervals: `d` (days), `w` (weeks), `m` (months), `y` (years).

### List items

Output is JSONL — one JSON object per line, sorted by due date. Each line contains the full todo object (same schema as the storage file), including the full 26-char ULID.

```bash
todo list                          # non-done items, sorted by due date
todo list --all                    # include done items
todo list --category work          # filter by category
todo list --status inprogress      # filter by status
todo list --overdue                # only past-due items
```

### Update items

Takes the full 26-char ULID from `todo list` output.

```bash
todo update 01KHT3ABCDEF01KHT3ABCDEF01 --name "Buy organic groceries"
todo update 01KHT3ABCDEF01KHT3ABCDEF01 --status inprogress
todo update 01KHT3ABCDEF01KHT3ABCDEF01 --due 2026-04-01
todo update 01KHT3ABCDEF01KHT3ABCDEF01 --category personal --desc "Updated description"
todo update 01KHT3ABCDEF01KHT3ABCDEF01 --recurrence 2w
```

### Mark done

Completed items are moved to an archive file alongside the data file — `<name>.archive.jsonl` (e.g. `~/.todo.jsonl` → `~/.todo.archive.jsonl`, `~/Dropbox/todo.jsonl` → `~/Dropbox/todo.archive.jsonl`).

```bash
todo done 01KHT3ABCDEF01KHT3ABCDEF01   # archived
todo done 01KHSYABCDEF01KHSYABCDEF01   # recurring: archived + new item created
```

### Remove items

```bash
todo rm 01KHT3ABCDEF01KHT3ABCDEF01     # prompts for confirmation
todo rm 01KHT3ABCDEF01KHT3ABCDEF01 --force  # no confirmation
```

### List categories

```bash
todo categories
```

## Simplifying decisions

**All todo items have a due date.** If something doesn't have a due date, you don't really want to do it. There is no "someday maybe" bucket — either commit to a date or don't track it.

**All recurrences start the next interval upon completion.** When you complete a recurring item, the next occurrence is due `now + interval`, not `previous_due + interval`. If you're late finishing something, the next one doesn't stack up behind it.
