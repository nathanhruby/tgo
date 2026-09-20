# Watch Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a full-screen `tgo watch` command that immediately renders open tasks and reloads them every two seconds.

**Architecture:** A focused Bubble Tea model in `watch.go` owns refresh state, timer sequencing, error display, and quit handling while delegating task formatting to `TaskList.List`. `main.go` only registers the command and passes the existing CLI flags into the model.

**Tech Stack:** Go 1.26+, urfave/cli v3, Bubble Tea v1.3.10, Go's standard `testing` package

**Spec:** `docs/superpowers/specs/2026-09-20-watch-command-design.md`

## Global Constraints

- Render immediately, then refresh every two seconds.
- Never overlap refreshes or accumulate timers.
- Use Bubble Tea alternate-screen mode.
- Exit on `q` or `Ctrl+C`.
- Reuse `TaskList.List` with kind `"tasks"`; do not duplicate list formatting.
- Support the existing `--grep`, `--verbose`, and `--quiet` list flags.
- Display refresh errors, retain the last successful output, and retry.
- Do not add interval configuration, done-task watching, task mutation, filesystem events, styling, selection, or scrolling.
- Do not commit changes; the user requested implementation but did not request Git commits.
- Preserve the already-added Bubble Tea dependency files and the failing `watch_test.go` red test.

## File Structure

- Create `watch.go`: watch configuration, Bubble Tea model/messages/commands, rendering, and program runner.
- Modify `watch_test.go`: direct model tests with real temporary task files and an injected scheduler boundary.
- Modify `main.go`: register the `watch` command using `listFlags()` and implement its CLI action.
- Modify `main_test.go`: verify the command and shared flags are exposed by `buildApp()`.
- Modify `README.md`: add concise watch usage.
- Modify `go.mod` and `go.sum`: retain Bubble Tea v1.3.10 and run `go mod tidy` after it becomes a direct import.

---

### Task 1: Bubble Tea Watch Model

**Files:**
- Create: `watch.go`
- Modify: `watch_test.go`

**Interfaces:**
- Consumes: `NewTaskList(taskDir, name string) (*TaskList, error)` and `(*TaskList).List(io.Writer, kind string, verbose, quiet bool, grep string)`
- Produces: `watchConfig`, `newWatchModel(watchConfig) watchModel`, and a `watchModel` implementing `tea.Model`

- [ ] **Step 1: Keep the existing initialization test red**

Run:

```bash
go test ./... -run TestWatchModelLoadsTasksImmediately
```

Expected: compilation fails because `newWatchModel` and `watchConfig` do not exist.

- [ ] **Step 2: Implement the minimum immediate-load model**

Create `watch.go` with configuration fields `taskDir`, `listName`, `grep`, `verbose`, and `quiet`; a model that stores output and an error; a load command that calls `NewTaskList`, renders through `TaskList.List` into a `strings.Builder`, and returns a typed result message; and `Init`, `Update`, and `View` methods.

- [ ] **Step 3: Run the initialization test green**

```bash
go test ./... -run TestWatchModelLoadsTasksImmediately
```

Expected: PASS.

- [ ] **Step 4: Add failing behavior tests one at a time**

Add focused tests for:

1. `grep`, `verbose`, and `quiet` being forwarded to the real list renderer;
2. a successful load scheduling the next timer with a duration of exactly `2*time.Second`;
3. a timer message returning a command that reloads the task file;
4. a refresh error appearing in `View` while retaining prior output;
5. `q` and `Ctrl+C` returning the Bubble Tea quit command.

Use an injectable scheduler function on the model to observe the requested duration without sleeping. Use real temporary task files for rendering behavior. Run each new test before implementation and confirm it fails for the intended missing behavior.

- [ ] **Step 5: Implement timer sequencing, error rendering, and quit handling**

Use one result message and one timer message. A result schedules one timer; a timer starts one load. Keep the last successful output when a result contains an error. `View` appends `error: <message>` only while an error is present.

- [ ] **Step 6: Run model tests**

```bash
go test ./... -run 'TestWatchModel'
```

Expected: PASS with no real two-second waits.

### Task 2: CLI Integration

**Files:**
- Modify: `main.go`
- Modify: `main_test.go`
- Modify: `watch.go`

**Interfaces:**
- Consumes: `newWatchModel(watchConfig) watchModel`, root `--task-dir` and `--list`, and shared `listFlags()`
- Produces: `watchAction(context.Context, *cli.Command) error` and a registered `watch` command

- [ ] **Step 1: Add a failing CLI registration test**

In `main_test.go`, inspect `buildApp().Commands` and assert a command named `watch` exists with flags named `grep`, `verbose`, and `quiet`. The failure must identify the missing `watch` command.

- [ ] **Step 2: Run the registration test red**

```bash
go test ./... -run TestCLI_WatchCommand
```

Expected: FAIL because `buildApp()` has no `watch` command.

- [ ] **Step 3: Add the watch action and command**

Implement `watchAction` to build `watchConfig` from root and command flags, start `tea.NewProgram(newWatchModel(config), tea.WithAltScreen())`, and return the program error. Register `watch` in `buildApp()` with usage `Watch open tasks`, `listFlags()`, and `watchAction`.

- [ ] **Step 4: Run CLI and model tests green**

```bash
go test ./...
```

Expected: PASS.

### Task 3: Documentation and Dependency Finalization

**Files:**
- Modify: `README.md`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Consumes: completed `tgo watch` CLI behavior
- Produces: documented usage and a tidy module graph with Bubble Tea as a direct dependency

- [ ] **Step 1: Document watch usage**

Add an example after the existing list example:

```bash
# Continuously watch open tasks (press q or Ctrl+C to exit)
tgo watch
tgo watch --grep apartment
```

- [ ] **Step 2: Tidy the module graph**

```bash
go mod tidy
```

Confirm `github.com/charmbracelet/bubbletea v1.3.10` is a direct requirement and only its transitive packages remain indirect.

- [ ] **Step 3: Run focused and full validation**

```bash
go test ./...
go build ./...
golangci-lint run ./...
```

Expected: all available commands pass. If `golangci-lint` is unavailable, record that explicitly rather than claiming it passed.
