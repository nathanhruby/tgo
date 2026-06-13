# tgo Design Spec

**Date:** 2026-06-13  
**Project:** `tgo` — a Go port of [sjl/t](https://github.com/sjl/t)  
**Goal:** A minimal, idiomatic Go task manager that remains file-format compatible with the original `t`.

---

## Overview

`tgo` is a command-line todo list manager for people who want to finish tasks, not organize them. It is a port of Steve Losh's `t` (Python) to Go, preserving the file format and core behavior while adopting idiomatic Go structure and a modern subcommand-based CLI.

---

## File Format (Compatible with `t`)

Tasks are stored in plain text files, one task per line:

```
task text | id:<sha1hex>
```

- **Open tasks:** `<taskdir>/<name>` (e.g. `~/tasks/tasks`)
- **Done tasks:** `<taskdir>/.<name>.done` (e.g. `~/tasks/.tasks.done`)
- Lines beginning with `#` are treated as comments and ignored.
- Files are sorted by ID on every write, which keeps VCS diffs clean and minimizes merge conflicts when multiple people share a task file.
- Task IDs are SHA1 hashes of the task text (UTF-8 encoded), matching the original exactly.

A bare text line (no `|` separator) is also valid on read — an ID will be generated from the text. This allows hand-editing the file.

---

## Data Model

```go
// tasks.go

type Task struct {
    ID   string
    Text string
}

type TaskList struct {
    Tasks   map[string]Task // id -> open task
    Done    map[string]Task // id -> finished task
    Name    string          // list name, e.g. "tasks"
    TaskDir string          // directory containing the task files
}
```

---

## Prefix Algorithm

Tasks are referenced by the shortest unique prefix of their SHA1 ID. The algorithm computes these in O(n) time and is a faithful port of the original Python implementation.

```go
// tasks.go
func prefixes(ids []string) map[string]string // returns id -> shortest unique prefix
```

This is the mechanism behind short display IDs like `9`, `30`, `31` in the original's examples.

---

## Error Types

Errors are typed values implementing the `error` interface:

```go
type ErrAmbiguousPrefix struct{ Prefix string }
type ErrUnknownPrefix   struct{ Prefix string }
type ErrInvalidTaskFile struct{ Path string }
type ErrBadFile         struct{ Path, Problem string }
```

`main.go` uses `errors.As` to catch these, print a human-readable message to stderr, and exit with code 1.

---

## `TaskList` Methods

```go
func NewTaskList(taskDir, name string) (*TaskList, error)

func (tl *TaskList) Add(text string) (prefix string, err error)
func (tl *TaskList) Finish(prefix string) error
func (tl *TaskList) Remove(prefix string) error
func (tl *TaskList) Edit(prefix, newText string) error
func (tl *TaskList) Write(deleteIfEmpty bool) error
func (tl *TaskList) List(kind string, verbose, quiet bool, grep string)
```

- `NewTaskList` reads both task files from disk. Returns `ErrInvalidTaskFile` if a path is a directory, `ErrBadFile` on I/O failure.
- `Add` returns the new task's short prefix so the CLI can print it.
- `getTask(prefix string) (Task, error)` is an unexported helper that resolves a prefix to a single task, returning `ErrAmbiguousPrefix` or `ErrUnknownPrefix` as appropriate. Used by `Finish`, `Remove`, and `Edit`.
- `Edit` replaces the full task text (no sed-style substitution).
- `List` writes to stdout. `kind` is `"tasks"` or `"done"`.
- `Write` sorts tasks by ID before writing, and deletes the file if `deleteIfEmpty` is true and the list is empty.

---

## CLI (`main.go`)

Built with `urfave/cli` v3. Uses subcommands with global flags for configuration. Running `tgo` with no subcommand defaults to `list`.

### Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--task-dir DIR` | `-t` | `""` | Directory containing task files |
| `--list LIST` | `-l` | `tasks` | Task list name |
| `--delete-if-empty` | `-d` | false | Delete task file when it becomes empty |

### Subcommands

| Command | Args | Flags | Description |
|---------|------|-------|-------------|
| `add TEXT` | task text (required) | | Add a new task; prints its short prefix |
| `list` | | `-g/--grep`, `-v/--verbose`, `-q/--quiet` | List open tasks (default command) |
| `done` | | `-g/--grep`, `-v/--verbose`, `-q/--quiet` | List finished tasks |
| `finish TASK` | prefix (required) | | Mark task as finished |
| `remove TASK` | prefix (required) | | Remove task from list |
| `edit TASK TEXT` | prefix + new text (both required) | | Replace task text |

### Example Usage

```sh
# Add tasks
tgo add "Clean the apartment"
tgo add "Buy more beer"

# List open tasks
tgo

# Finish a task
tgo finish 9

# Edit a task
tgo edit 30 "Clean the entire apartment"

# Remove a task
tgo remove 9

# List done tasks
tgo done

# Use a different list in a specific directory
tgo -t ~/tasks -l groceries add "Oat milk"
```

---

## Project Structure

```
tgo/
├── main.go        # CLI setup, flag/subcommand wiring, error formatting
├── tasks.go       # Task, TaskList, prefix algorithm, file I/O
├── tasks_test.go  # Unit tests for core logic
├── main_test.go   # Integration tests via urfave/cli test helpers
├── go.mod
└── go.sum
```

---

## Testing

**`tasks_test.go`** — unit tests, no disk I/O:
- Prefix algorithm: unique prefixes, collisions, single-task list
- `Add`, `Finish`, `Remove`, `Edit` on in-memory `TaskList`
- File parsing round-trip: serialize → deserialize → compare
- Error cases: `ErrAmbiguousPrefix`, `ErrUnknownPrefix`, `ErrInvalidTaskFile`

**`main_test.go`** — integration tests against a temp directory:
- Each subcommand exercised end-to-end
- Assert stdout output, file contents after writes, exit codes on errors

Test dependencies: stdlib `testing` package only.

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/urfave/cli/v3` | CLI framework |

No other external dependencies.

---

## Explicitly Out of Scope

- Sed-style substitution in `edit` (dropped; full text replacement only)
- Priorities, tags, projects, or any other organizational metadata
- Colors or other terminal formatting
- Configuration files (all config via flags/aliases, same as original)
