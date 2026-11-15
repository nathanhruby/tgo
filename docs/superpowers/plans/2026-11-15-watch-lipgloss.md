# Watch Mode: Lipgloss List and Status Bar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render `tgo watch`'s open tasks as a Lipgloss bulleted list and pin a full-width status bar with a live clock, list name, and open count to the bottom of the screen.

**Architecture:** `tasks.go` gains one unexported `taskRow` type and a `taskRows` function that both `List` (plain text) and the watch view (styled list items) render from, so `tgo list` output is unchanged by construction. `watch.go` replaces its pre-rendered output string with rows and composes three layers per frame: an error row, Lipgloss list items, and a reverse-video bar sized to the terminal, with the body padded to `height-1`.

**Tech Stack:** Go 1.26 module (toolchain 1.27.4 via mise), `charm.land/bubbletea/v2 v2.1.1`, `charm.land/lipgloss/v2 v2.1.1` (new direct dependency), `github.com/charmbracelet/x/ansi v0.11.9` (promoted from indirect).

**Spec:** `docs/superpowers/specs/2026-11-15-watch-lipgloss-design.md`

## Global Constraints

- Work on a branch cut from `main`; never commit to `main`. Push and open a PR at the end (`AGENTS.md`).
- Value receivers on `watchModel`; no test-only branches in production code.
- `TaskList.List` output must not change: same order, labels, padding, quiet/verbose precedence, grep behavior, and empty-list silence.
- Dependencies: `charm.land/lipgloss/v2 v2.1.1` and `github.com/charmbracelet/x/ansi v0.11.9`. No other new modules.
- Exact strings from the spec: clock layout `Mon Jan 2 2006 · 15:04:05`; bar right side `<list> · <N> open · q to quit`; overflow marker `… +N more`; separators ` - ` and ` · `.
- Tests never sleep and never read the wall clock: inject `now`, `clockTick`, and `schedule`.
- Verification commands: `mise run test`, `mise run lint`, `mise run check`. Conventional commit subjects, validated with `mise run pr-title -- "<subject>"`.
- `Style.Width` wraps text, it does not truncate: truncate content with `ansi.Truncate` before styling.

## Review Focus

1. **Terminal narrower than the bar.** At widths 20, 10, and 1 the view must not exceed the width and must not panic; the right side drops first, then the clock truncates with `…`. Pinned in Task 4.
2. **Heights 1 and 2.** One row shows the error if there is one, else the bar; two rows with an error show error + bar and drop the overflow marker. Pinned in Task 4.
3. **Wide and emoji task text.** CJK and emoji must be measured by cell width, not bytes, and truncation must not split a grapheme or leave an unterminated escape. Pinned in Task 4.
4. **Everything filtered out.** `--grep` matching nothing still leaves a padded body and a bar whose count is the whole open list, not zero. Pinned in Tasks 3 and 4.
5. **Clock tick during an in-flight load.** A clock message must not start a disk read and must not disturb the single-flight refresh. Pinned in Task 4.

---

### Task 1: Pin `tgo list` output before the refactor

Behavior does not change in this task. These tests pass on arrival by design: they are the guard for Task 2, and the documented exception to red-first in the spec.

**Files:**
- Modify: `tasks_test.go` (append tests after `TestList_Done`, around line 428)

**Interfaces:**
- Consumes: `TaskList{Tasks, Done, Name, TaskDir}`, `TaskList.List(w io.Writer, kind string, verbose, quiet bool, grep string)`.
- Produces: nothing. Test-only commit.

- [ ] **Step 1: Write the exact-output characterization test**

Fixture labels are determined by `prefixes`: three ids `aaa`, `aab`, `bbb` yield labels `aaa`, `aab`, `b`, so the column is three wide and `b` is padded to `"b  "`.

```go
func TestList_ExactOutput(t *testing.T) {
	longID := "0123456789abcdef0123456789abcdef01234567"

	aligned := map[string]Task{
		"aaa": {ID: "aaa", Text: "First"},
		"aab": {ID: "aab", Text: "Second"},
		"bbb": {ID: "bbb", Text: "Third"},
	}

	tests := []struct {
		name   string
		tasks  map[string]Task
		done   map[string]Task
		kind   string
		verb   bool
		quiet  bool
		grep   string
		want   string
	}{
		{name: "aligned", tasks: aligned, want: "aaa - First\naab - Second\nb   - Third\n"},
		{name: "verbose", tasks: map[string]Task{longID: {ID: longID, Text: "Only"}}, verb: true, want: longID + " - Only\n"},
		{name: "quiet", tasks: aligned, quiet: true, want: "First\nSecond\nThird\n"},
		{name: "quiet beats verbose", tasks: aligned, quiet: true, verb: true, want: "First\nSecond\nThird\n"},
		{name: "grep keeps padding", tasks: aligned, grep: "third", want: "b   - Third\n"},
		{name: "grep matches nothing", tasks: aligned, grep: "nope", want: ""},
		{name: "empty list", tasks: map[string]Task{}, want: ""},
		{name: "done", tasks: map[string]Task{}, done: map[string]Task{"aaa": {ID: "aaa", Text: "Done task"}}, kind: "done", want: "aaa - Done task\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := &TaskList{Tasks: tt.tasks, Done: tt.done, Name: "tasks", TaskDir: t.TempDir()}
			var buf strings.Builder
			tl.List(&buf, tt.kind, tt.verb, tt.quiet, tt.grep)
			if got := buf.String(); got != tt.want {
				t.Fatalf("List output = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run it and confirm it passes**

Run: `mise run test`
Expected: PASS, including the new subtests. A failure here means the fixture expectation is wrong about `prefixes`, not that the code is broken; fix the expectation, not the code.

- [ ] **Step 3: Commit**

```bash
git add tasks_test.go
git commit -m "test: pin exact tgo list output before row refactor"
mise run pr-title -- "test: pin exact tgo list output before row refactor"
```

---

### Task 2: Extract `taskRows` in `tasks.go`

Behavior-preserving refactor. Task 1 is the regression proof.

**Files:**
- Modify: `tasks.go:307-377` (`buildLabelFn` stays, `printTasks` and `List` are replaced)
- Test: `tasks_test.go` (append)

**Interfaces:**
- Consumes: `buildLabelFn(ids []string, verbose bool) (func(string) string, int)`, `Task`.
- Produces: `type taskRow struct{ Label, Text string }`, `func taskRows(tasks map[string]Task, verbose, quiet bool, grep string) ([]taskRow, int)`. Later tasks use both, plus `taskRow.Label` and `taskRow.Text` by name.

- [ ] **Step 1: Write the failing tests**

```go
func TestTaskRows_LabelsAndWidth(t *testing.T) {
	rows, width := taskRows(map[string]Task{
		"aaa": {ID: "aaa", Text: "First"},
		"aab": {ID: "aab", Text: "Second"},
		"bbb": {ID: "bbb", Text: "Third"},
	}, false, false, "")

	want := []taskRow{{Label: "aaa", Text: "First"}, {Label: "aab", Text: "Second"}, {Label: "b", Text: "Third"}}
	if !slices.Equal(rows, want) {
		t.Fatalf("rows = %+v, want %+v", rows, want)
	}
	if width != 3 {
		t.Fatalf("width = %d, want 3", width)
	}
}

func TestTaskRows_GrepKeepsPadding(t *testing.T) {
	rows, width := taskRows(map[string]Task{
		"aaa": {ID: "aaa", Text: "First"},
		"aab": {ID: "aab", Text: "Second"},
		"bbb": {ID: "bbb", Text: "Third"},
	}, false, false, "THIRD")

	if len(rows) != 1 || rows[0].Label != "b" || rows[0].Text != "Third" {
		t.Fatalf("rows = %+v, want only the matching row", rows)
	}
	if width != 3 {
		t.Fatalf("width = %d, want 3 from all ids, not the filtered subset", width)
	}
}

func TestTaskRows_QuietBeatsVerbose(t *testing.T) {
	rows, width := taskRows(map[string]Task{"aaa": {ID: "aaa", Text: "First"}}, true, true, "")

	if len(rows) != 1 || rows[0].Label != "" || rows[0].Text != "First" {
		t.Fatalf("rows = %+v, want a text-only row", rows)
	}
	if width != 0 {
		t.Fatalf("width = %d, want 0 in quiet mode", width)
	}
}

func TestTaskRows_Empty(t *testing.T) {
	rows, width := taskRows(map[string]Task{}, false, false, "")

	if len(rows) != 0 || width != 0 {
		t.Fatalf("rows = %+v width = %d, want none", rows, width)
	}
}
```

Add `slices` to the import block of `tasks_test.go`.

- [ ] **Step 2: Run to verify failure**

Run: `mise run test`
Expected: FAIL, `undefined: taskRows` / `undefined: taskRow`.

- [ ] **Step 3: Add `taskRow` and `taskRows`; rewrite `List`; delete `printTasks`**

`taskRow` goes above `TaskList`:

```go
// taskRow is one display row: an optional label (shortest unique id prefix, full id when
// verbose, empty when quiet) and the task text.
type taskRow struct {
	Label string
	Text  string
}
```

`taskRows` replaces `printTasks` at `tasks.go:333`:

```go
// taskRows returns display rows for tasks in id order, applying quiet/verbose labelling and
// case-insensitive grep filtering. labelWidth is the widest label across every task, before
// grep filtering, so filtered output keeps the alignment of the unfiltered list.
func taskRows(tasks map[string]Task, verbose, quiet bool, grep string) ([]taskRow, int) {
```

Order of operations inside, which is what preserves output: return no rows for an empty map; collect ids and compute `buildLabelFn` over *all* ids; `sort.Strings(ids)`; skip tasks whose lowered text does not contain the lowered grep; emit `taskRow{Text: task.Text}` with width 0 when quiet (quiet wins over verbose), else `taskRow{Label: labelFn(id), Text: task.Text}`.

`List` keeps its signature and becomes a formatter:

```go
rows, width := taskRows(tasks, verbose, quiet, grep)
for _, row := range rows {
	if width == 0 {
		fmt.Fprintln(w, row.Text)
		continue
	}
	fmt.Fprintf(w, "%-*s - %s\n", width, row.Label, row.Text)
}
```

Keep the existing kind selection (`"done"` → `tl.Done`) and drop the manual id collection and early return, which now live in `taskRows`. Delete `printTasks` entirely.

- [ ] **Step 4: Run the whole suite**

Run: `mise run test`
Expected: PASS, including Task 1's exact-output test unchanged — that is the proof the refactor is behavior-preserving.

- [ ] **Step 5: Lint**

Run: `mise run lint`
Expected: no issues. Remove any now-unused helper the compiler or linter flags.

- [ ] **Step 6: Commit**

```bash
git add tasks.go tasks_test.go
git commit -m "refactor(tasks): render task rows from one source"
mise run pr-title -- "refactor(tasks): render task rows from one source"
```

---

### Task 3: Watch body as a Lipgloss list

Adds the dependency and swaps watch's stored string for rows. No bar yet; height behavior is unchanged in this task.

**Files:**
- Modify: `go.mod`, `go.sum` (dependency added)
- Modify: `watch.go` (whole file: `watchLoadResult`, `load`, `View`)
- Test: `watch_test.go`

**Interfaces:**
- Consumes: `taskRows(tasks map[string]Task, verbose, quiet bool, grep string) ([]taskRow, int)`.
- Produces: `watchLoadResult{rows []taskRow; labelWidth int; openCount int; err error}`; method `renderBody() string`; `watchModel` no longer has `output`. Task 4 extends `watchLoadResult` usage and reads `openCount`.

- [ ] **Step 1: Add the dependencies**

Run: `go get charm.land/lipgloss/v2@v2.1.1 github.com/charmbracelet/x/ansi@v0.11.9`
Expected: `go.mod` lists both as direct requires; `go.sum` gains lipgloss entries.

- [ ] **Step 2: Write the failing tests**

```go
// watchBody returns the view with styling stripped, as a list of lines.
func watchBody(view string) []string {
	view = strings.TrimSuffix(ansi.Strip(view), "\n")
	if view == "" {
		return nil
	}
	return strings.Split(view, "\n")
}

func TestWatchModelRendersLipglossList(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "First"},
		"aab": {ID: "aab", Text: "Second"},
		"bbb": {ID: "bbb", Text: "Third"},
	})

	config := watchConfig{taskDir: dir, listName: "tasks"}
	model := newWatchModel(config)
	updated, _ := model.Update(model.Init()())

	want := []string{"• aaa - First", "• aab - Second", "• b   - Third"}
	if got := watchBody(updated.(watchModel).View().Content); !slices.Equal(got, want) {
		t.Fatalf("view lines = %q, want %q", got, want)
	}
}

func TestWatchModelQuietRendersBulletsWithoutLabels(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{"aaa": {ID: "aaa", Text: "First"}})

	config := watchConfig{taskDir: dir, listName: "tasks", quiet: true}
	model := newWatchModel(config)
	updated, _ := model.Update(model.Init()())

	if got, want := watchBody(updated.(watchModel).View().Content), []string{"• First"}; !slices.Equal(got, want) {
		t.Fatalf("view lines = %q, want %q", got, want)
	}
}

func TestWatchModelBodyMatchesListOutput(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "First"},
		"aab": {ID: "aab", Text: "Second"},
	})

	config := watchConfig{taskDir: dir, listName: "tasks"}
	model := newWatchModel(config)
	updated, _ := model.Update(model.Init()())

	body := watchBody(updated.(watchModel).View().Content)
	listLines := strings.Split(strings.TrimSuffix(renderTaskList(t, config), "\n"), "\n")

	if len(body) != len(listLines) {
		t.Fatalf("watch body has %d lines, list has %d", len(body), len(listLines))
	}

	for i := range listLines {
		if want := "• " + listLines[i]; body[i] != want {
			t.Fatalf("line %d = %q, want %q", i, body[i], want)
		}
	}
}
```

Add `slices` and `"github.com/charmbracelet/x/ansi"` to `watch_test.go` imports.

- [ ] **Step 3: Run to verify failure**

Run: `mise run test`
Expected: FAIL, `undefined: watchBody` is not it — these compile, and the assertions fail because the view has no bullets and no styling (`view lines = ["aaa - First", ...]`). A compile error means `newWatchModel`/`Init` signatures drifted; investigate before continuing.

- [ ] **Step 4: Implement rows and the list body**

In `watch.go`:

```go
type watchLoadResult struct {
	rows      []taskRow
	labelWidth int
	openCount int
	err       error
}
```

`load()` builds both:

```go
tasks, err := NewTaskList(m.config.taskDir, m.config.listName)
if err != nil {
	return watchLoadResult{err: err}
}
rows, labelWidth := taskRows(tasks.Tasks, m.config.verbose, m.config.quiet, m.config.grep)
return watchLoadResult{rows: rows, labelWidth: labelWidth, openCount: len(tasks.Tasks)}
```

Add package-level styles:

```go
var (
	watchLabelStyle = lipgloss.NewStyle().Faint(true)
	watchErrorStyle = lipgloss.NewStyle().Bold(true)
)
```

`renderBody()` on the model renders the rows: for each row, `watchLabelStyle.Width(m.labelWidth).Render(row.Label) + " - " + row.Text` when the label is present, otherwise `row.Text`; pass the items to `list.New(items...).Enumerator(list.Bullet).String()` (the bullet adds `• ` and joins with newlines). `watchModel` keeps `rows`, `labelWidth`, `openCount` instead of `output`; `Update` assigns them from the result. `View` builds `lines` as: the error line `watchErrorStyle.Render("error: " + err.Error())` first when set, then `strings.Split(m.renderBody(), "\n")` when the body is non-empty, then applies the existing height truncation to `lines`, and returns `strings.Join(lines, "\n") + "\n"` when non-empty. The alternative-screen flag is unchanged.

- [ ] **Step 5: Update the tests that read the old string field**

These tests must be rewritten to the new contract, not loosened:

- `TestWatchModelLoadsTasksImmediately` — compare `watchBody(view)` against `[]string{"• a - A watched task"}` (single id `abc` yields label `a`).
- `TestWatchModelForwardsListOptions` — compare against the bulleted form of `renderTaskList`; keep the quiet and verbose assertions on the stripped body.
- `TestWatchModelRetainsOutputOnRefreshError` — assert `watchBody(view)` is `[]string{"error: refresh failed", "• a - A watched task"}` instead of comparing against `model.output`.

`TestWatchModelEmptyListRendersNothing`, the overflow tests, and the refresh-cycle tests keep their current expectations in this task.

- [ ] **Step 6: Run the whole suite**

Run: `mise run test`
Expected: PASS.

- [ ] **Step 7: Lint and commit**

```bash
mise run lint
git add go.mod go.sum watch.go watch_test.go
git commit -m "feat(watch): render tasks as a lipgloss list"
```

---

### Task 4: Status bar, clock, and screen layout

The bulk of the visible change. Two test/implement cycles inside one task, then the existing view tests move to the final contract.

**Files:**
- Modify: `watch.go`
- Test: `watch_test.go`

**Interfaces:**
- Consumes: `watchLoadResult{rows, labelWidth, openCount, err}`, `taskRow`.
- Produces: `watchClockInterval = time.Second`, `watchClockMsg struct{}`; `watchModel` fields `clock time.Time`, `now func() time.Time`, `clockTick func(time.Duration) tea.Cmd`, `width int`; methods `nextClockTick() tea.Cmd` and `statusBar() string`.

- [ ] **Step 1: Write the failing clock and bar-content tests**

```go
var watchTestTime = time.Date(2026, 11, 15, 14, 3, 22, 0, time.UTC)

// newTestWatchModel returns a model with a fixed clock, no real timers, and no scheduler.
func newTestWatchModel(t *testing.T, config watchConfig) watchModel {
	t.Helper()

	model := newWatchModel(config)
	model.clock = watchTestTime
	model.now = func() time.Time { return watchTestTime }
	model.clockTick = func(time.Duration) tea.Cmd { return func() tea.Msg { return watchClockMsg{} } }
	model.schedule = func(time.Duration) tea.Cmd { return nil }

	return model
}

// loadWatch runs Init and applies every resulting message, because Init batches the first
// load with the first clock tick.
func loadWatch(t *testing.T, model watchModel) watchModel {
	t.Helper()

	msg := model.Init()()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, cmd := range batch {
			updated, _ := model.Update(cmd())
			model = updated.(watchModel)
		}

		return model
	}

	updated, _ := model.Update(msg)

	return updated.(watchModel)
}

func TestWatchModelStatusBarContent(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "First"},
		"aab": {ID: "aab", Text: "Second"},
		"bbb": {ID: "bbb", Text: "Third"},
	})

	model := newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"})
	model.width = 80
	model = loadWatch(t, model)
	view := model.View().Content

	lines := watchBody(view)
	bar := lines[len(lines)-1]

	if want := 80; lipgloss.Width(bar) != want {
		t.Fatalf("bar width = %d, want %d: %q", lipgloss.Width(bar), want, bar)
	}
	if want := "Sun Nov 15 2026 · 14:03:22"; !strings.HasPrefix(bar, want) {
		t.Fatalf("bar = %q, want prefix %q", bar, want)
	}
	if want := "tasks · 3 open · q to quit"; !strings.HasSuffix(bar, want) {
		t.Fatalf("bar = %q, want suffix %q", bar, want)
	}
}

func TestWatchModelStatusBarCountsWholeListNotGrepMatches(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "First"},
		"bbb": {ID: "bbb", Text: "Second"},
	})

	model := newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks", grep: "first"})
	model.width = 80
	model = loadWatch(t, model)

	lines := watchBody(model.View().Content)
	if last := lines[len(lines)-1]; !strings.Contains(last, "2 open") {
		t.Fatalf("bar = %q, want the whole-list count", last)
	}
}

func TestWatchModelStatusBarWithoutWidthJoinsWithSpace(t *testing.T) {
	model := newTestWatchModel(t, watchConfig{listName: "tasks"})
	model = loadWatch(t, model)

	lines := watchBody(model.View().Content)
	if want, got := "Sun Nov 15 2026 · 14:03:22 tasks · 0 open · q to quit", lines[len(lines)-1]; got != want {
		t.Fatalf("bar = %q, want %q", got, want)
	}
}

func TestWatchModelClockTickUpdatesTimeAndReschedulesOneSecond(t *testing.T) {
	var got []time.Duration

	model := newTestWatchModel(t, watchConfig{})
	model.now = func() time.Time { return watchTestTime.Add(time.Minute) }
	model.clockTick = func(d time.Duration) tea.Cmd {
		got = append(got, d)
		return func() tea.Msg { return watchClockMsg{} }
	}

	updated, cmd := model.Update(watchClockMsg{})
	if cmd == nil {
		t.Fatal("clock message did not schedule the next tick")
	}
	if len(got) != 1 || got[0] != time.Second {
		t.Fatalf("scheduled %v, want exactly one 1s tick", got)
	}
	if msg := cmd(); msg != (watchClockMsg{}) {
		t.Fatalf("tick produced %T, want watchClockMsg", msg)
	}
	if want := watchTestTime.Add(time.Minute); !updated.(watchModel).clock.Equal(want) {
		t.Fatalf("clock = %v, want %v", updated.(watchModel).clock, want)
	}
}

func TestWatchModelInitSchedulesLoadAndClock(t *testing.T) {
	model := newTestWatchModel(t, watchConfig{taskDir: t.TempDir(), listName: "tasks"})

	msg := model.Init()()
	batch, ok := msg.(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("Init produced %T with %d commands, want 2", msg, len(batch))
	}
}

func TestWatchModelClockTickDuringLoadDoesNotStartLoad(t *testing.T) {
	model := newTestWatchModel(t, watchConfig{taskDir: t.TempDir(), listName: "tasks"})

	updated, load := model.Update(watchTimerMsg{})
	if load == nil {
		t.Fatal("timer did not start a load")
	}

	model = updated.(watchModel)

	clocked, tick := model.Update(watchClockMsg{})
	if tick == nil {
		t.Fatal("clock message scheduled nothing")
	}
	if _, isLoad := tick().(watchLoadResult); isLoad {
		t.Fatal("clock tick started a disk read")
	}
	if !clocked.(watchModel).loading {
		t.Fatal("clock tick cleared the in-flight load flag")
	}}
```

- [ ] **Step 2: Run to verify failure**

Run: `mise run test`
Expected: FAIL to compile: `model.clock`, `model.now`, `model.clockTick`, `model.width`, and `watchClockMsg` do not exist.

- [ ] **Step 3: Implement the clock and the bar line**

Add `watchClockInterval = time.Second` and `watchClockMsg struct{}` next to `watchRefreshInterval`. Add fields `clock`, `now`, `clockTick`, `width` to `watchModel`; `newWatchModel` seeds `now = time.Now`, `clock = now()`, and

```go
clockTick: func(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return watchClockMsg{} })
},
```

`nextClockTick()` returns `m.clockTick(watchClockInterval)`. `Init` returns `tea.Batch(m.load(), m.nextClockTick())`. `Update` handles `watchClockMsg` by setting `m.clock = m.now()` and returning `m.nextClockTick()`; it never touches `loading` or `load`. Handle `tea.WindowSizeMsg` by storing `msg.Width` alongside the existing height.

`statusBar()` composes `left := m.clock.Format("Mon Jan 2 2006 · 15:04:05")` and `right := fmt.Sprintf("%s · %d open · q to quit", m.config.listName, m.openCount)`, with `bar := lipgloss.NewStyle().Reverse(true)`, then applies exactly one branch:

- width unknown → `bar.Render(left + " " + right)`;
- `lipgloss.Width(left)+lipgloss.Width(right)+2 > m.width` → `bar.Width(m.width).Render(ansi.Truncate(left, m.width, "…"))`, which drops the right side and pads or truncates the clock to exactly the width;
- otherwise → `bar.Width(m.width).Render(left + strings.Repeat(" ", m.width-lipgloss.Width(left)-lipgloss.Width(right)) + right)`.

`View` appends `m.statusBar()` as the last line.

- [ ] **Step 4: Run the clock and bar tests**

Run: `go test ./... -run 'TestWatchModelStatusBar|TestWatchModelClock|TestWatchModelInit'`
Expected: PASS.

- [ ] **Step 5: Write the failing layout tests**

```go
func TestWatchModelPinsStatusBarToLastRow(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "First"},
		"bbb": {ID: "bbb", Text: "Second"},
	})

	model := newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"})
	model.width = 40
	model = loadWatch(t, model)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 40, Height: 6})

	lines := watchBody(updated.(watchModel).View().Content)
	if len(lines) != 6 {
		t.Fatalf("view has %d lines, want 6: %q", len(lines), lines)
	}
	if !strings.HasPrefix(lines[5], "Sun Nov 15 2026") {
		t.Fatalf("last line = %q, want the bar", lines[5])
	}
	for i := 2; i < 5; i++ {
		if strings.TrimSpace(lines[i]) != "" {
			t.Fatalf("line %d = %q, want blank padding", i, lines[i])
		}
	}
}

func TestWatchModelHeightBudgetShowsOverflowMarker(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "One"}, "bbb": {ID: "bbb", Text: "Two"},
		"ccc": {ID: "ccc", Text: "Three"}, "ddd": {ID: "ddd", Text: "Four"},
		"eee": {ID: "eee", Text: "Five"},
	})

	model := newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"})
	model = loadWatch(t, model)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 40, Height: 4})

	lines := watchBody(updated.(watchModel).View().Content)
	want := []string{"• aaa - One", "• bbb - Two", "… +3 more"}
	if len(lines) != 4 || !slices.Equal(lines[:3], want) || !strings.HasPrefix(lines[3], "Sun") {
		t.Fatalf("lines = %q, want %q then the bar", lines, want)
	}
}

func TestWatchModelErrorDisplacesBarAtHeightOne(t *testing.T) {
	model := newTestWatchModel(t, watchConfig{})
	updated, _ := model.Update(watchLoadResult{err: errors.New("refresh failed")})
	updated, _ = updated.(watchModel).Update(tea.WindowSizeMsg{Width: 40, Height: 1})

	lines := watchBody(updated.(watchModel).View().Content)
	if len(lines) != 1 || lines[0] != "error: refresh failed" {
		t.Fatalf("lines = %q, want only the error", lines)
	}

	model = newTestWatchModel(t, watchConfig{})
	updated, _ = model.Update(tea.WindowSizeMsg{Width: 40, Height: 1})
	lines = watchBody(updated.(watchModel).View().Content)
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "Sun") {
		t.Fatalf("lines = %q, want only the bar", lines)
	}
}

func TestWatchModelMarkerYieldsToErrorAtHeightTwo(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "One"}, "bbb": {ID: "bbb", Text: "Two"},
		"ccc": {ID: "ccc", Text: "Three"},
	})

	model := newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"})
	model = loadWatch(t, model)
	updated, _ := model.Update(watchLoadResult{err: errors.New("refresh failed")})
	updated, _ = updated.(watchModel).Update(tea.WindowSizeMsg{Width: 40, Height: 2})

	lines := watchBody(updated.(watchModel).View().Content)
	if len(lines) != 2 || lines[0] != "error: refresh failed" || !strings.HasPrefix(lines[1], "Sun") {
		t.Fatalf("lines = %q, want error then the bar", lines)
	}
}

func TestWatchModelTruncatesBarAndBodyToWidth(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{"aaa": {ID: "aaa", Text: "買い物 🛒 and a long tail"}})

	for _, width := range []int{20, 10, 1} {
		model := newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"})
		model = loadWatch(t, model)
		updated, _ := model.Update(tea.WindowSizeMsg{Width: width, Height: 4})

		lines := watchBody(updated.(watchModel).View().Content)
		for i, line := range lines {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d: line %d is %d cells wide: %q", width, i, got, line)
			}
		}

		if width != 20 {
			continue
		}
		if !strings.Contains(lines[0], "…") {
			t.Fatalf("task line = %q, want an ellipsis", lines[0])
		}
		if strings.Contains(lines[3], "q to quit") {
			t.Fatalf("bar = %q, want the right side dropped at width 20", lines[3])
		}
	}
}

func TestWatchModelEmptyListShowsBlankBodyAndBar(t *testing.T) {
	model := newTestWatchModel(t, watchConfig{taskDir: t.TempDir(), listName: "tasks"})
	model = loadWatch(t, model)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 30, Height: 4})

	lines := watchBody(updated.(watchModel).View().Content)
	if len(lines) != 4 || !strings.HasPrefix(lines[3], "Sun") {
		t.Fatalf("lines = %q, want 3 blank rows and the bar", lines)
	}
}
```

- [ ] **Step 6: Run to verify failure**

Run: `mise run test`
Expected: FAIL, a mix of the new layout assertions and older tests that still feed `tea.BatchMsg` straight into `Update`; Step 8 converts those. The clock and bar tests from Step 4 still pass.

- [ ] **Step 7: Implement the layout contract**

Implement the precedence table from the spec exactly:

- Body budget is `height-1` when height is known, `0` at height 1. With unknown height, render everything and no padding.
- Body order: error row (when set), then task rows, then `… +N more` where `N` counts task rows hidden by the budget only.
- Drop the marker before any task row, and drop task rows before the error. At height 1 an error replaces the bar; otherwise the bar is always the last line.
- Pad the body with blank lines to exactly `height-1` rows when the height is known.
- Truncate every body line with `ansi.Truncate(line, m.width, "…")` when width is known, after styling.
- Return `strings.Join(lines, "\n") + "\n"`; with a bar present the view is never empty, so `TestWatchModelEmptyListRendersNothing` is replaced by `TestWatchModelEmptyListShowsBlankBodyAndBar` (delete the old test in this step and note why in the commit body).

- [ ] **Step 8: Rewrite the remaining view tests to the final contract**

- `TestWatchModelLoadsTasksImmediately` — expected lines: `["• a - A watched task", bar]` with no height, so no padding; assert the bar with `strings.HasPrefix`.
- `TestWatchModelForwardsListOptions` — same shape per option set; keep the quiet/verbose body assertions.
- `TestWatchModelKeepsErrorVisibleWhenOutputOverflowsTerminal` — height 3 and five tasks with a
  refresh error now yields exactly `["error: refresh failed", "… +5 more", bar]`: the marker counts
  hidden task rows only, and the error is not one of them.
- `TestWatchModelShowsWholeListWhenTerminalFits` — height 10: error, two tasks, six blanks, bar.
- `TestWatchModelRetainsOutputOnRefreshError` — `["error: refresh failed", "• a - A watched task", bar]`.
- `TestWatchModelInitialLoadErrorIsVisible` — error row plus bar.
- `TestWatchModelClearsErrorAfterSuccessfulRefresh` — assert on stripped lines, not substrings of a raw view.
- `TestWatchModelEmptyListRendersNothing` — delete; covered by `TestWatchModelEmptyListShowsBlankBodyAndBar`.

Also re-drive the three list tests added in Task 3 (`TestWatchModelRendersLipglossList`, `TestWatchModelQuietRendersBulletsWithoutLabels`, `TestWatchModelBodyMatchesListOutput`) through `loadWatch`: only the driving changes, and each expectation gains the trailing bar line, which those assertions must now account for by comparing only the body lines (`lines[:len(listLines)]`) against the bulleted list.

Every rewritten test injects the fixed clock through `newTestWatchModel` so bar expectations are stable.

- [ ] **Step 9: Run the whole suite and lint**

Run: `mise run test && mise run lint`
Expected: PASS, no lint issues.

- [ ] **Step 10: Commit**

```bash
git add watch.go watch_test.go
git commit -m "feat(watch): pin status bar with live clock to the last row"
```

---

### Task 5: Documentation, full checks, and PR

**Files:**
- Modify: `docs/superpowers/specs/2026-09-20-watch-command-design.md` (header only)
- Modify: `docs/superpowers/plans/2026-09-20-watch-command.md` (header only, same note)

**Interfaces:**
- Consumes: nothing.
- Produces: the shipped branch and PR.

- [ ] **Step 1: Point the old spec at the new one**

Under the `**Status:** Approved` line of `2026-09-20-watch-command-design.md` add:

```markdown
**Superseded in part:** `2026-11-15-watch-lipgloss-design.md` replaces this document's
"no styling", "no additional UI dependency", and height-only rendering positions. The rest of
this specification still applies.
```

Add the same one-line note beneath the plan document's title.

- [ ] **Step 2: Run every repository check**

Run: `mise run test && mise run lint && mise run check`
Expected: all three pass. If `mise run check` reports a pre-existing failure, report it explicitly instead of claiming success.

- [ ] **Step 3: Commit the docs**

```bash
git add docs/superpowers/specs/2026-09-20-watch-command-design.md docs/superpowers/plans/2026-09-20-watch-command.md
git commit -m "docs: point watch spec at the lipgloss design"
```

- [ ] **Step 4: Push and open the PR**

```bash
mise run pr-title -- "feat(watch): render list with lipgloss and add status bar"
git push -u origin "$(git branch --show-current)"
gh pr create --title "feat(watch): render list with lipgloss and add status bar" --body "$(git log --oneline main..HEAD)"
```

Expected: `gh pr view` shows the open PR for the branch.
