# Watch Mode: Lipgloss List and Status Bar

**Date:** 2026-11-15
**Status:** Draft — awaiting approval
**Supersedes in part:** `2026-09-20-watch-command-design.md` (its "no styling", "no extra UI
dependency", and "no width handling" positions are replaced by this document; everything else
in that spec still holds).

## Intent

`tgo watch` already refreshes the open task list every two seconds, but it renders whatever
`TaskList.List` produced: plain lines of `id - text` with no visual hierarchy and no sense of
when it last ran or what it is showing. It looks like `tgo list` in a loop.

This change makes watch a small read-only dashboard: the open tasks rendered as a Lipgloss
list, and a full-width status bar pinned to the bottom showing the current date and time plus
what list is being watched. The goal is glanceability — the task text should be the loudest
thing on screen, ids should be findable but quiet, and the bar should answer "what am I looking
at and how do I leave" without a help screen.

## Goals

- Render open tasks with `charm.land/lipgloss/v2/list`, one bulleted item per task.
- Show a status bar on the last terminal row with the date, the time, the list name, the open
  task count, and the quit hint.
- Keep the clock live at one-second resolution, independent of the two-second disk refresh.
- Keep `tgo list` output byte-identical and derive both renderers from one row source.
- Stay theme-safe: no hardcoded palette, so the UI reads correctly on light and dark terminals.

## Non-Goals

- Selection, cursors, scrolling, in-UI filtering, finishing or editing tasks from watch.
- Styling `tgo list`, `tgo done`, or any other subcommand.
- Watching the done file, configurable refresh interval, file-system events.
- Configurable date/time format, locale, or timezone.
- Status-bar extras such as load state, spinner, error badge, or last-error timestamp.

## Anti-Goals

- A bar that pushes the list off a short terminal, or an overflow marker that disappears because
  the bar took its row.
- Drift between `tgo list` and the watch body: same label, same text, same order, only styling
  and the bullet may differ.
- Hiding a load error behind the bar.
- Tests that assert ANSI bytes or loosen to substring checks to accommodate styling.

## Constraints

- Bubble Tea v2 (`charm.land/bubbletea/v2 v2.1.1`) is already in use; the model keeps value
  receivers and the existing `load` / `schedule` / timer shape.
- New direct dependencies: `charm.land/lipgloss/v2 v2.1.1` and `github.com/charmbracelet/x/ansi`
  (already present indirectly) for width-aware truncation.
- `TaskList.List` is the contract other subcommands use; its output must not change.
- Tests must not sleep or read the wall clock: time enters through an injected `now` function and
  an injected tick function.

## Approaches Considered

**A. Wrap the existing rendered text in a list.** Split `watchModel.output` on newlines and feed
each line to `list.New`. Ten lines in one file and `tasks.go` untouched. Rejected: the bullet
stacks on top of the existing `id -` prefix, and the id cannot be styled separately from the task
text without parsing `" - "` out of user text that may legitimately contain it.

**B. Add a row renderer to `tasks.go` (chosen).** Extract the label/text pair that `printTasks`
already computes into an unexported `taskRow`, and have `List` format rows for the terminal while
watch builds list items from the same rows. Both outputs share one source of truth, so parity is
structural rather than a coincidence of two format strings.

**C. Adopt `charm.land/bubbles/v2/list`.** Gives a cursor, paging, filtering, and help. Rejected:
it buys interactivity that was explicitly ruled out, adds a second module, and would need the
refresh loop to reconcile external file changes with cursor and filter state.

B was chosen because it is the only option where styling has something to attach to (a quiet id,
a normal task text) without turning a dashboard into an editor.

## Design

### Row extraction (`tasks.go`)

```go
type taskRow struct {
    Label string // shortest unique prefix, full id when verbose, empty when quiet
    Text  string
}

// taskRows returns the rows for tasks in display order, plus the widest label,
// after applying case-insensitive grep filtering.
func taskRows(tasks map[string]Task, verbose, quiet bool, grep string) (rows []taskRow, labelWidth int)
```

- `taskRows` owns ordering (sort by id), label computation (`buildLabelFn` over *all* ids, so
  grep filtering never changes padding), grep filtering, and the quiet case.
- `List` becomes: `rows, width := taskRows(...)` then `fmt.Fprintf(w, "%-*s - %s\n", width, row.Label, row.Text)`,
  with a `fmt.Fprintln(w, row.Text)` branch when `width` is 0 (quiet), because `%-*s` at width 0
  would still emit the separator. Its output is unchanged and is locked down by the
  characterization tests added before the refactor (see Testing).
- `printTasks` is deleted; its sort, grep, and formatting duties split between `taskRows` (data)
  and `List` (plain text). `buildLabelFn` survives and is called only by `taskRows`.
- Empty input returns no rows and width 0, so the "print nothing" rule lives in one place.
- Quiet wins over verbose, matching today: quiet rows carry an empty label and width 0 even when
  `--verbose` is also set.

### Watch rendering (`watch.go`)

`watchLoadResult` carries `rows []taskRow`, `labelWidth int`, `openCount int`, and `err error`,
replacing the pre-rendered `output string`. `openCount` is `len(tl.Tasks)` — the whole open list,
not the grep-filtered subset, because the bar describes the list rather than the view.

`View` builds the body as:

1. The error line, when there is one, as a plain bold `error: <message>` row with no bullet.
2. One list item per row: `faint(label padded to labelWidth)` + `" - "` + text.

The body items are passed to `list.New(items...).Enumerator(list.Bullet)` and rendered with
`String()`, giving `• ` prefixes and one item per line. The padding lives in a
`lipgloss.NewStyle().Faint(true).Width(labelWidth)` so alignment survives styling. The separator
stays ` - ` rather than the two-space gutter in the sketch, so that stripping styling and the
bullet from a watch line yields exactly the corresponding `tgo list` line — that equality is the
parity test.

The status bar is a single line:

```
Sun Nov 15 2026 · 14:03:22                    tasks · 3 open · q to quit
```

- Left: the clock formatted `Mon Jan 2 2006 · 15:04:05`.
- Right: `<list name> · <open count> open · q to quit`.
- Styled `lipgloss.NewStyle().Reverse(true)` padded to the terminal width. Reverse video uses the
  terminal's own foreground/background, so it needs no palette knowledge and reads on both themes.
- Width rules, in order: if `width` is unknown (no `WindowSizeMsg` yet) join left and right with a
  single space and do not fill; if `width` can hold left, right, and at least two spaces, separate
  them with padding to exactly `width`; otherwise drop the right side, then truncate the left with
  an ellipsis if it still overflows.
- Pinning: the bar is the last row of the alternative screen, not the line after the last task.
  When the height is known the body area is exactly `height-1` rows and is padded with blank rows
  below the tasks, so a two-task list in a forty-row terminal shows the bar on row 40. An empty
  list is therefore a full-height blank body with the bar at the bottom, not a one-line view. When
  the height is unknown there is nothing to pad against, so the bar follows the content directly.
- Body rows wider than `width` are truncated with `ansi.Truncate(line, width, "…")`.

**Precedence when the terminal is too short for everything.** The bar normally wins the last row,
but it yields to a load error, because an error the user cannot see is worse than a missing clock.

| Height | Error | Result |
| --- | --- | --- |
| unknown | any | error, tasks, bar; no padding, no marker |
| ≥ 2 | none | up to `height-1` task rows, or `height-2` when `… +N more` is needed, then the bar |
| ≥ 2 | present | error as the first body row, then tasks and the marker in whatever rows remain, then the bar |
| 1 | none | the bar only |
| 1 | present | the error only; the bar is dropped |

Within the body the order is fixed: error, then task rows, then the overflow marker. The marker is
the first thing to go when the body budget is exhausted; the error is the last. `N` in
`… +N more` counts task rows hidden by the budget only, never the error or the marker.

### Clock

- New `watchClockInterval = time.Second` and `watchClockMsg struct{}`.
- `watchModel` gains `clock time.Time` (the time to display), `now func() time.Time`, and
  `clockTick func(time.Duration) tea.Cmd`. `newWatchModel` seeds `now` and `clockTick` with
  `time.Now` and `tea.Tick`, and initializes `clock` from `now()`.
- `Init` returns `tea.Batch(m.load(), m.nextClockTick())`.
- `watchClockMsg` sets `clock = now()` and returns the next clock tick. It never touches disk and
  never disturbs the refresh cycle, so an in-flight load is unaffected.
- `View` reads `m.clock` only. It never calls `time.Now`, which keeps rendering deterministic. A
  zero-value model (no seeded clock) renders an empty left-hand side; tests that build model
  literals set `clock` explicitly.
- The refresh cycle (two seconds, one load in flight, result schedules one timer) is unchanged.

## Consequences

- `watchLoadResult` and `watchModel` change shape, so `watch_test.go` assertions that compare the
  view against `TaskList.List` output move to the stripped-body-plus-bar form; they are rewritten,
  not loosened.
- The alternate-screen view grows one row for the bar and pads to full height, so the existing
  overflow test's expected line count, the empty-list test's `""` view, and the hidden-row
  arithmetic all change with the new height budget. Those tests are rewritten to the new contract.
- Commit order matters: characterization tests for `List` land first with no production change,
  then the red watch tests for the new view, then the row refactor and the feature. This keeps the
  `tgo list` guard in place before `tasks.go` moves.
- `go.mod` gains lipgloss as a direct dependency and promotes `x/ansi` to direct.
- The old spec keeps its historical text; a line at the top of
  `2026-09-20-watch-command-design.md` points here for the superseded positions.

## Testing

Unit level:

- `taskRows`: display order, shortest prefixes, verbose full ids, quiet empty labels, grep
  filtering case-insensitively without changing label width, and parity with `List` formatting.
- `List` output is protected by exact-output characterization tests added in the first commit:
  multi-task column alignment with differing label widths, verbose, quiet, grep, done, and the
  empty-list early return. Today every `TestList_*` test is a substring check, so nothing would go
  red if `taskRows` changed padding. Those tests pass on arrival by design — the behavior they
  lock down is explicitly not changing — which is the documented exception to the red-first rule
  in `AGENTS.md`; the red tests for this feature are the new watch assertions below.

Model level, driving `Init`/`Update`/`View` with a fixed clock and injected ticks:

- loaded view equals the bulleted, stripped rows followed by the exact bar line;
- bar shows injected date and time, list name, and open count, and ignores grep for the count;
- `watchClockMsg` updates the displayed second and schedules exactly one further clock tick,
  without producing a load command;
- `Init` schedules both a load and a clock tick, and a clock tick never starts a disk read;
- error line stays first, is not bulleted, and survives overflow and short terminals, including the
  height-1 case where it displaces the bar;
- height budget: the bar sits on the last of `height` rows with the body padded to `height-1`,
  `… +N more` accounting, the marker yielded before the error, height 1 showing the bar alone when
  there is no error, and unknown height showing everything plus the bar with no padding;
- width budget: bar fills exactly the width, right side drops before left truncates, long task text
  truncates with an ellipsis, unknown width joins with a single space;
- empty list renders a blank body with the bar at the bottom; finished tasks and done-file state
  remain invisible;
- `q` and `Ctrl+C` still quit and releases are still ignored.

Validation: `mise run test`, `mise run lint`, `mise run check`, per `AGENTS.md`. Work happens on a
branch off `main` and lands through a pull request.

## Left to the Builder

- Concrete style values that stay theme-safe: faint for labels, reverse for the bar, bold for the
  error line, bullet enumerator. No palette. If reverse video looks wrong in a real terminal,
  substitute another theme-neutral treatment (bold/underline bar) rather than fixed colors.
- Exact padding and spacing inside the bar, and whether the right-hand segments stay one string or
  are composed from labeled parts.
- Whether `taskRows` returns the width or a small struct; the signature above is a shape, not a
  mandate.
- Where the truncate helper lives and whether it wraps `ansi.Truncate`.
