# Watch Command Design

**Date:** 2026-09-20
**Status:** Approved

## Goal

Add `tgo watch`, a full-screen terminal command that continuously displays the selected open-task list and refreshes it every two seconds.

## User Experience

```bash
tgo watch
tgo watch --grep groceries
tgo watch --verbose
tgo watch --quiet
```

The command renders the list immediately in Bubble Tea's alternate screen. It reloads the selected list every two seconds without overlapping refreshes. Pressing `q` or `Ctrl+C` exits and restores the previous terminal contents.

`watch` accepts the same `--grep`, `--verbose`, and `--quiet` flags as `list`. The global `--task-dir` and `--list` flags continue to select the task storage directory and list name.

## Rendering

Each refresh loads a new `TaskList` from disk and delegates task formatting to the existing `TaskList.List` method with kind `"tasks"`. This keeps sorting, ID formatting, quiet output, verbose output, and case-insensitive grep behavior identical to `tgo list`.

The view displays the latest successful list output. If loading fails, the view displays an `error: <message>` status while retaining any prior successful output, then retries after the next two-second interval. An empty list produces an empty task area rather than placeholder text.

## Architecture

A focused `watch.go` module owns:

- immutable watch configuration derived from CLI flags;
- a Bubble Tea model containing the latest rendered output and refresh error;
- commands for loading/rendering the list and scheduling the next refresh;
- key handling for `q` and `Ctrl+C`;
- the Bubble Tea program entry point using alternate-screen mode.

Refresh completion schedules exactly one next timer. Timer completion starts exactly one new load, preventing concurrent disk reads and timer accumulation.

`main.go` registers `watch` and reuses `listFlags()`.

## Error Handling

An initial or later task-file error is rendered inside the running UI rather than terminating the program. The watcher remains responsive to quit keys and retries automatically on the next interval. Errors starting the Bubble Tea program itself are returned through the existing CLI error path.

## Dependencies

Use `github.com/charmbracelet/bubbletea` v1.3.10, the current version resolved for this module when the feature was started. No additional direct UI dependency is needed.

## Testing

Tests exercise the model directly without launching an interactive terminal or waiting in real time:

- initialization loads and renders a real temporary task file;
- list flags produce the same observable filtering and formatting as `list`;
- successful refreshes schedule the next refresh for two seconds;
- timer messages trigger a new load;
- refresh errors are visible and preserve the previous successful output;
- `q` and `Ctrl+C` return Bubble Tea's quit command;
- the CLI registers `watch` with the shared list flags.

Validation runs `go test ./...`, `go build ./...`, and `golangci-lint run ./...` when the configured linter is available.

## Non-Goals

- changing the refresh interval through a flag;
- watching finished tasks;
- editing or completing tasks inside the watch UI;
- file-system event watching;
- colors, selection, scrolling, or other interactive list controls.
