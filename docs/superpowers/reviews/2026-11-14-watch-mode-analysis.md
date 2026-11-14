# Watch Mode Analysis

**Date:** 2026-11-14\
**Branch:** `analysis/watch-mode-review`\
**Scope:** `watch.go`, watch-related code in `main.go`, `watch_test.go`, `main_test.go`, and the interactions with `tasks.go` that watch depends on.\
**References:** `docs/superpowers/specs/2026-09-20-watch-command-design.md`, `docs/superpowers/plans/2026-09-20-watch-command.md`

## Verdict

The implementation is small, readable, and broadly idiomatic Go. No correctness blockers were found.

The two assumptions in the request need correcting:

1. **Lipgloss is not used anywhere in this repository.** It is not a direct dependency, not in `go.sum`, and has no imports. This is consistent with the approved design, which explicitly makes "colors, selection, scrolling, or other interactive list controls" a non-goal. So there is no Lipgloss code to call unidiomatic — the question is whether the project *should* adopt Lipgloss, which is a spec change.
2. **The tests are a mix.** Several genuinely exercise behavior through `Init`/`Update`/`View`; several only assert fixture text or implementation details, and the core "a changed file is picked up" behavior is not tested at all.

Evidence run on this branch:

```plaintext
go test ./... -race -cover   # ok, coverage 82.4% of statements
go vet ./...                 # clean
golangci-lint run ./...      # 0 issues
```

## 1. Go idiom and design

### Clean

- Value receivers on `watchModel` are correct for the Bubble Tea model contract; `Update` mutates its copy and returns it.
- `load()` captures the value-receiver copy, so config fields are read safely from the command goroutine. No shared mutable state, no race.
- `tea.Tick` is the right primitive here: one-shot tick scheduled after a result completes prevents timer accumulation. `tea.Every` would allow queued ticks.
- `q`/`ctrl+c` handling returns `tea.Quit`; releases and modified keys are ignored.
- `tea.View` + `tea.NewView` + `view.AltScreen = true` is the correct Bubble Tea v2 idiom. `tea.WithAltScreen()` no longer exists in v2.
- Errors from loading are shown in the UI instead of terminating the program, as specified.

### Findings

| \# | Severity | Location | Finding | Recommendation |
| --- | --- | --- | --- | --- |
| G1 | Medium | `main.go:64-74` | `watchAction` discards `ctx` and never passes `tea.WithContext(ctx)`. Program cancellation cannot stop the watcher. Harmless today because `main` passes `context.Background()`, but wrong CLI contract. | `tea.NewProgram(model, tea.WithContext(ctx)).Run()`; map `tea.ErrInterrupted` to a clean exit if needed. |
| G2 | Low | `watch.go:21-26, 35-39, 52-61` | `schedule` is a nil-able function field installed only by `newWatchModel`. A zero-value `watchModel` that receives a load result panics. Tests already construct `watchModel` literals (`watch_test.go:169`). | Make scheduling a method that falls back to `tea.Tick` when `schedule == nil`, or make the seam a constructor-injected interface. |
| G3 | Low | `watch.go:45-67` | The "exactly one refresh in flight" invariant is conventional, not structural. A duplicate/delayed `watchTimerMsg` starts a second load; every result then schedules another timer. | Add an `inFlight` guard or a generation counter on load/timer messages and ignore stale ones. |
| G4 | Medium | `docs/.../2026-09-20-watch-command-design.md:49`, plan `:9,:118,:155` | Spec/plan are stale: they require Bubble Tea v1.3.10 and `tea.WithAltScreen()`. Code correctly uses `charm.land/bubbletea/v2 v2.1.1` and `View.AltScreen`. The spec also says `watch.go` owns the program entry point, while `main.go` does. | Update the docs to v2 and the actual module boundary; keep the code. |
| G5 | Low | `watch.go:83`, `tasks.go:210-213,412` | Watch reads both `tasks` and `.tasks.done` on every refresh, so a bad done file blocks watching open tasks. Writes use truncate-then-write, so a refresh racing `tgo add` can read a partial/empty file. Both are pre-existing. | Add an open-only loader for watch; write via temp file + rename. |
| G6 | Low | `watch.go:72` | `fmt.Sprintf("%serror: %v\n", output, m.err)` implicitly assumes `List` output ends in a newline. True today (`tasks.go:349`), fragile later. | Build view lines in a slice/`strings.Builder` and insert a separator only when needed. |
| G7 | Nit | `main.go:236`, `watch.go:21-43` | `listFlags` doc says "for the list and done subcommands" but watch uses it. Unexported watch types have no doc comments although the rest of the package does. | Update comment; add one-line docs. |
| G8 | Nit | `watch.go:48` | Calls `msg.Keystroke()` twice. Only lowercase `q` quits. | `switch msg.String() { case "q", "ctrl+c": ... }` for readability. |
| G9 | Nit | `main.go:249-253` vs `watch.go:48` | `-q` means `--quiet`, while pressing `q` quits. Potentially confusing. | Document it, or don't bind `-q` on `watch`. |

**Top Go priorities:** G1 (pass context), G2 (nil-safe scheduler), G3 (structural single-flight), G4 (stale spec), G5 (done-file coupling / atomic writes).

## 2. Lipgloss and TUI rendering

### Lipgloss status

`lipgloss` does not appear in `go.mod`, `go.sum`, or any `.go` file. Bubble Tea v1 historically pulled `lipgloss` in indirectly; the v2 migration removed it. The project has never used it.

The approved spec lists styling as a non-goal, so the absence is **compliant**, not a bug. If styling is desired:

- Revise the spec first (it currently says no colors/styling).
- Add `charm.land/lipgloss/v2` as a **direct** dependency (v2 module path).
- Minimal use: error style (red), faint IDs, bold header.
- Note: task lines are pre-formatted inside `TaskList.List` (`tasks.go:334-352`). Per-ID styling cannot live only in `watch.go`; it requires refactoring the shared renderer or accepting styled whole lines.

### Findings

| \# | Severity | Location | Finding | Recommendation |
| --- | --- | --- | --- | --- |
| T1 | Medium | `watch.go:69-79` | No `tea.WindowSizeMsg` handling. When the list is taller than the terminal, alt-screen clipping drops the bottom, including the `error:` line. Spec requires errors to be visible. | Handle `WindowSizeMsg`; render a status/error header row first, then as many body rows as fit; show `… +N more`. |
| T2 | Low/Medium | `watch.go:75` | No width handling. Long task text is hard-cut at the right edge without an ellipsis. | Track `msg.Width` and truncate with `ansi.Truncate(line, width, "…")` or `lipgloss.NewStyle().MaxWidth(w)`. |
| T3 | Low | `watch.go:69-79` | No title, hint, timestamp, or "live" indicator. Users cannot tell it is refreshing or how to exit. These are not prohibited by the spec. | One header row: `tgo watch · updated HH:MM:SS · q to quit`, replaced by the error line on failure. |
| T4 | Low | `watch.go:69-79` | No stale-content artifact risk in v2: the renderer clears and redraws its cell buffer each flush, so shrinking output is erased correctly. | No change. |
| T5 | Low | `watch.go:76` | `AltScreen` is used correctly for v2. | No change; fix the docs instead. |
| T6 | Low | `main.go:66` | Program is constructed without a context. Same issue as G1. | Pass `tea.WithContext(ctx)`. |
| T7 | Nit | `watch.go:48` | Key style (`Keystroke()` twice) is not the v2 upgrade-guide style. | Switch on `msg.String()`. |

**Top TUI priorities:** T1 (errors can be clipped / no height awareness), T3 (header/status line), T2 (width truncation), T6 (context).

A note on scope: the spec's non-goals (no scrolling, no colors, no selection) mean the UI is intentionally minimal. T1 and T3 are about legibility of required information, not styling; they can be addressed without violating the spec.

## 3. Test quality

Coverage: `watch.go` `Update`/`View` 100%, `load` 85.7%, `newWatchModel` 66.7% (the real `tea.Tick` closure is never executed).

### Classification

| Test | Location | Classification | Notes |
| --- | --- | --- | --- |
| `TestWatchModelLoadsTasksImmediately` | `watch_test.go:24` | Integration-ish | Real temp file, real `Init`/`Update`/`View`. Good. Hard-codes prefix `"a"`. |
| `TestWatchModelForwardsListOptions` | `watch_test.go:38` | Partial behavior | Only `grep` is observable. Sets `verbose: true` but never asserts full IDs; `quiet` is never exercised. Name overclaims. |
| `TestWatchModelSchedulesNextRefresh` | `watch_test.go:52` | Behavior (seam) | Valid: asserts 2s via injected scheduler. |
| `TestWatchModelTimerReloadsTaskFile` | `watch_test.go:70` | Partial integration | Drives timer→load, but reloads an unchanged file. Does not prove changes are picked up. |
| `TestWatchModelRetainsOutputOnRefreshError` | `watch_test.go:85` | Implementation detail | Sets `model.err` directly, bypassing `Update`. Only tests `View`. |
| `TestWatchModelRetriesAfterRefreshErrorAndRetainsOutput` | `watch_test.go:99` | Behavior | Genuinely good: success→error through `Update`, retention + error + schedule. One assertion (`cmd == nil`) tests the stub. |
| `TestWatchModelTimerCycleSchedulesExactlyOneSubsequentTimer` | `watch_test.go:131` | Behavior (logic) | Exercises production `Update`; uses a fake timer but that is legitimate. |
| `TestWatchModelViewUsesAltScreen` | `watch_test.go:166` | Spec/implementation assertion | Struct literals; checks only the bool. |
| `TestWatchModelIgnoresOtherKeysAndReleases` | `watch_test.go:178` | Behavior | Decent key coverage. |
| `TestWatchModelQuitsOnQAndCtrlC` | `watch_test.go:192` | Behavior | Good; verifies `tea.QuitMsg`. |
| `TestCLI_WatchCommand` | `main_test.go:20` | Implementation detail | Walks the command tree. Does not prove flags are accepted or wired into `watchConfig`. |

### Findings

| \# | Severity | Location | Finding | Recommendation |
| --- | --- | --- | --- | --- |
| Q1 | High | `watch_test.go:70` | **No test proves a changed file is picked up.** Reloading the same fixture would pass even if the code cached the first read. This is the feature's core promise. | `TestWatchModelReloadsChangedFile`: init with text A, rewrite the file to B, feed timer, assert B present and A absent. |
| Q2 | High | `main_test.go:20`, `main.go:64-75` | `watchAction` flag wiring is untested. Swapped `quiet`/`verbose` or a wrong `cmd.String` would pass. | Extract `watchConfigFromCommand(cmd)` and unit-test it; or inject a program runner. |
| Q3 | High/Medium | `watch_test.go:38` | `quiet` and `verbose` forwarding are never asserted. | Exact-output tests: quiet renders `"A watched task\n"` with no ID; verbose renders the full 40-char ID. |
| Q4 | Medium | `watch_test.go:33` | Hard-coded prefix `"a"` couples tests to `prefixes()` and sort order. A correct prefix algorithm change breaks the test for no behavioral reason. | Compute expected output by calling `NewTaskList` + `List` into a buffer (also enforces list/watch parity). |
| Q5 | Medium | `watch.go:84-86` | Initial-load error path is never executed. | `TestWatchModelInitialLoadErrorVisible` with a directory as the task file; assert `error:` in view and a 2s retry scheduled. |
| Q6 | Medium | `watch.go:59` | Error clearing after a successful refresh is untested. | success→error→success; assert no `error:` and new output. |
| Q7 | Medium | `watch_test.go:91` | `RetainsOutputOnRefreshError` tests the test harness by assigning `model.err`. | Drive it through `watchLoadResult{err: ...}`; merge with the retry test. |
| Q8 | Low | `watch_test.go:126` | Asserts `cmd == nil` because the stub returns nil; validates the fake. | Assert `schedules == 1` instead. |
| Q9 | Low | — | Empty list, done-task exclusion, and "output replaced not appended" are untested. | Add `TestWatchModelEmptyListRendersBlank`, `TestWatchModelExcludesDoneTasks`, and an exact view assertion after a second load. |
| Q10 | Low | `watch.go:36-38` | Real `tea.Tick` closure (66.7% of `newWatchModel`) is never executed; no `-race` concurrency test. | Add one test that calls the production scheduler command and asserts a `watchTimerMsg`; run relevant tests with `-race` in CI. |
| Q11 | Nit | `watch_test.go` | Substring-only assertions; `ViewUsesAltScreen` would pass even if `View` dropped all output. | Use exact view equality where practical. |

### Proposed test matrix

| Test | Setup / drive | Assertion |
| --- | --- | --- |
| `TestWatchModelReloadsChangedFile` | Init A; rewrite file to B; timer | View contains B only |
| `TestWatchModelQuietOutput` | `quiet: true` | Exact `"A watched task\n"` |
| `TestWatchModelVerboseOutput` | `verbose: true` | Full 40-char ID line |
| `TestWatchModelEmptyListRendersBlank` | empty dir | Exact `""` |
| `TestWatchModelInitialLoadErrorVisible` | task path is a directory; `Init` | `"error:"` + 2s schedule |
| `TestWatchModelClearsErrorAfterSuccess` | success→error→success | No `"error:"`, new output |
| `TestWatchModelRetainsOutputThroughUpdateError` | success; `watchLoadResult{err}` | Exact `output+"error: …\n"` |
| `TestWatchModelExcludesDoneTasks` | write done file | Done text absent |
| `TestWatchConfigFromCommand` | CLI command with flags | Config fields match |
| `TestCLI_WatchAcceptsFlags` | runner seam | Flags parsed and passed through |

## 4. Prioritized action list

1. **Q1** — Test that on-disk changes are actually picked up.
2. **Q2/Q3** — Test flag wiring and quiet/verbose output.
3. **G1/T6** — Pass the CLI context into Bubble Tea.
4. **T1/T3** — Add a status/error header and height awareness so required errors stay visible.
5. **Q5/Q6** — Cover initial-load errors and error clearing after success.
6. **G2/G3** — Make the scheduler nil-safe and the single-flight invariant structural.
7. **G4** — Update the spec/plan to Bubble Tea v2 and the actual module boundary.
8. **Q4/Q7-Q11** — Reduce fixture coupling, replace harness-only assertions.
9. **Lipgloss** — No action unless the spec is revised to allow styling; then add `charm.land/lipgloss/v2` directly.
