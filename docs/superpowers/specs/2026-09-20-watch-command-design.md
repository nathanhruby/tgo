# Watch Command Design

**Date:** 2026-09-20
**Status:** Approved

**Superseded in part:** `2026-11-15-watch-lipgloss-design.md` replaces this document's
"no styling", "no additional UI dependency", and height-only rendering positions. The rest of
this specification still applies.

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

The error status is rendered before the task output so it stays visible on short terminals. When the view is taller than the terminal, it keeps the lines that fit and ends with a `… +N more` marker instead of letting the terminal clip the bottom.

## Architecture

A focused `watch.go` module owns:

- immutable watch configuration derived from CLI flags;
- a Bubble Tea model containing the latest rendered output and refresh error;
- commands for loading/rendering the list and scheduling the next refresh;
- key handling for `q` and `Ctrl+C`.

Refresh completion schedules exactly one next timer. Timer completion starts exactly one new load; a timer that arrives while a load is in flight is ignored, preventing concurrent disk reads and timer accumulation.

`main.go` registers `watch`, reuses `listFlags()`, and owns the Bubble Tea program entry point using alternate-screen mode.

## Error Handling

An initial or later task-file error is rendered inside the running UI rather than terminating the program. The watcher remains responsive to quit keys and retries automatically on the next interval. Errors starting the Bubble Tea program itself are returned through the existing CLI error path.

## Dependencies

Use `charm.land/bubbletea/v2` (v2.1.1). Alternate-screen mode is enabled through `tea.View.AltScreen`; the v1 `tea.WithAltScreen()` option no longer exists. No additional direct UI dependency, including Lipgloss, is needed; styling remains a non-goal.

## Testing

Tests exercise the model directly without launching an interactive terminal or waiting in real time:

- initialization loads and renders a real temporary task file;
- list flags produce the same observable filtering and formatting as `list`;
- successful refreshes schedule the next refresh for two seconds;
- timer messages trigger a new load, and a timer during an in-flight load is ignored;
- an on-disk change is visible after the next refresh;
- refresh errors are visible, preserve the previous successful output, and clear after a successful refresh;
- empty lists and finished tasks are handled as specified;
- overflow output keeps the error visible and reports hidden lines;
- `q` and `Ctrl+C` return Bubble Tea's quit command;
- the CLI registers `watch` with the shared list flags and passes its own context to the program.

Validation runs `go test ./...`, `go build ./...`, and `golangci-lint run ./...` when the configured linter is available.

## Non-Goals

- changing the refresh interval through a flag;
- watching finished tasks;
- editing or completing tasks inside the watch UI;
- file-system event watching;
- colors, selection, scrolling, or other interactive list controls.
