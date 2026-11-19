# Group Tasks by Leading Emoji Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--group` (`-G`) so `tgo list`, `tgo done`, and `tgo watch` collect tasks under the emoji run that starts their text, with `list` printing headings and `watch` rendering a Lipgloss nested list.

**Architecture:** A new `splitEmojiGroup` uses `rivo/uniseg` grapheme clusters to split each task into a group key and display text. `taskRow` gains a `Group` field and `taskRows` gains a `group bool`; with grouping on it sorts by `(group key, id)` with the empty key last. Both renderers walk the same flat rows and start a block when `Group` changes: `List` prints headings, and watch maps each emoji block to a heading plus nested list. Watch truncation learns about heading lines so it never ends on a bare heading.

**Tech Stack:** Go 1.27, `charm.land/lipgloss/v2` list, `github.com/rivo/uniseg` (promoted from indirect), `urfave/cli/v3`.

**Spec:** `docs/superpowers/specs/2026-11-18-group-tasks-by-emoji-design.md`

## Global Constraints

- Work on `nrh/groups` off `main`; never commit to `main`, and deliver through a pull request.
- Standard library first; the only dependency change is promoting `github.com/rivo/uniseg` from indirect to direct. No new modules.
- Tests come first in every task, through the real entry point; never skip, loosen, or delete an existing test.
- `-g` stays `--grep`. Grouping is `--group` / `-G`.
- Grouping is display-only: no file-format, ID, or stored-text change.
- Watch tests inject the clock, ticks, and scheduler; never sleep or read the wall clock.
- Before each commit: `mise run test`, `mise run lint`, `mise run check` must pass. Conventional commits; validate the PR title with `mise run pr-title -- "<title>"`.

## Review Focus

- Task text that is only emoji (`🍕`): the row must show the task, never a blank bullet. Pinned by `TestTaskRows_GroupedEmojiOnlyKeepsText`.
- ZWJ families, flags, and skin tones: the key must be the whole cluster. Pinned by `TestSplitEmojiGroup`.
- `--group` with no emoji tasks: output must be byte-identical to `--group` off. Pinned by `TestList_GroupedWithoutEmojiMatchesUngrouped` and `TestWatchModelGroupingWithoutEmojiMatchesFlat`.
- Grep for an emoji that display text has stripped: it must still match. Pinned by `TestTaskRows_GroupedGrepMatchesStoredText` and `TestList_GroupedGrepOnEmoji`.
- Short-terminal truncation landing on a heading: no dangling heading, and `… +N more` counts only task rows. Pinned by `TestWatchModelGroupedHeightBudgetDropsDanglingHeading` and `TestWatchModelGroupedOverflowCountsOnlyTasks`.

## File Structure

- Modify `tasks.go`: emoji detection, `taskRow.Group`, grouped `taskRows`, grouped `List` rendering.
- Modify `watch.go`: `watchConfig.group`, `watchLine`, grouped `renderBody`, heading-aware truncation.
- Modify `main.go`: `--group` flag, action wiring, watch config wiring.
- Modify `tasks_test.go`, `watch_test.go`, `main_test.go`: red-first coverage and signature updates.
- Modify `README.md`: usage examples.
- Modify `go.mod`: `rivo/uniseg` becomes a direct requirement.

---

### Task 1: Detect the leading emoji run

**Files:**
- Modify: `tasks.go` (new helpers near `hashText`, around line 41)
- Modify: `go.mod`, `go.sum`
- Test: `tasks_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `func splitEmojiGroup(text string) (key, rest string)` and `func isEmojiCluster(cluster string) bool`.

- [ ] **Step 1: Write the failing table test**

Append to `tasks_test.go`:

```go
func TestSplitEmojiGroup(t *testing.T) {
	tests := []struct{ name, text, key, rest string }{
		{"no emoji", "Buy milk", "", "Buy milk"},
		{"one emoji", "🍕 Order pizza", "🍕", "Order pizza"},
		{"adjacent emoji", "🍕🚀 Pizza launch", "🍕🚀", "Pizza launch"},
		{"space ends run", "🍕 🚀 Pizza", "🍕", "🚀 Pizza"},
		{"ascii ends run", "🍕Pizza", "🍕", "Pizza"},
		{"cjk ends run", "🍕寿司", "🍕", "寿司"},
		{"zwj family", "👨‍👩‍👧 Family", "👨‍👩‍👧", "Family"},
		{"flag", "🇺🇸 Vote", "🇺🇸", "Vote"},
		{"skin tone", "👍🏽 Nice", "👍🏽", "Nice"},
		{"keycap stays ungrouped", "1️⃣ First", "", "1️⃣ First"},
		{"emoji only", "🍕", "🍕", "🍕"},
		{"emoji and space only", "🍕 ", "🍕", "🍕 "},
		{"variation selector", "©️ Legal", "©️", "Legal"},
		{"latin letter", "éclair", "", "éclair"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, rest := splitEmojiGroup(tt.text)
			if key != tt.key || rest != tt.rest {
				t.Fatalf("splitEmojiGroup(%q) = (%q, %q), want (%q, %q)", tt.text, key, rest, tt.key, tt.rest)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./... -run '^TestSplitEmojiGroup$'`
Expected: FAIL, `undefined: splitEmojiGroup`.

- [ ] **Step 3: Implement the split**

Add to `tasks.go` (imports: `unicode/utf8`, `github.com/rivo/uniseg`; `strings` is already imported):

```go
type runeRange struct{ lo, hi rune }

// emojiRanges covers the blocks whose default presentation is emoji. U+FE0F is handled
// separately so text-presentation symbols such as ❤️ and ©️ still count.
var emojiRanges = []runeRange{
	{0x1F000, 0x1FAFF}, // pictographs, emoticons, transport, flags
	{0x2600, 0x27BF},   // miscellaneous symbols and dingbats
	{0x2B00, 0x2BFF},   // miscellaneous symbols and arrows
	{0x231A, 0x231B},   // watch, hourglass
	{0x23E9, 0x23FA},   // media controls
	{0x23F0, 0x23F0},   // alarm clock
	{0x23F3, 0x23F3},   // hourglass with flowing sand
}

// isEmojiCluster reports whether a grapheme cluster can start an emoji group. ASCII
// first runes are rejected so a space or keycap ends the run.
func isEmojiCluster(cluster string) bool {
	first, _ := utf8.DecodeRuneInString(cluster)
	if first < utf8.RuneSelf {
		return false
	}

	for _, r := range cluster {
		if r == 0xFE0F {
			return true
		}

		for _, rg := range emojiRanges {
			if r >= rg.lo && r <= rg.hi {
				return true
			}
		}
	}

	return false
}

// splitEmojiGroup splits text into its leading run of emoji clusters and the display
// text that follows. One space after the run is removed; a task that is only emoji
// keeps its full text so the row is never blank.
func splitEmojiGroup(text string) (string, string) {
	rest := text
	state := -1

	var key strings.Builder

	for rest != "" {
		cluster, next, _, nextState := uniseg.StepString(rest, state)
		if !isEmojiCluster(cluster) {
			break
		}

		key.WriteString(cluster)
		rest, state = next, nextState
	}

	if key.Len() == 0 {
		return "", text
	}

	display := strings.TrimPrefix(rest, " ")
	if display == "" {
		display = text
	}

	return key.String(), display
}
```

Then promote the dependency: `go mod tidy` and confirm `github.com/rivo/uniseg` moved from the indirect block to the direct `require` block in `go.mod`.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./... -run '^TestSplitEmojiGroup$'`
Expected: PASS.

- [ ] **Step 5: Run the full checks and commit**

Run: `mise run test && mise run lint && mise run check`
Expected: all pass.

```bash
git add tasks.go tasks_test.go go.mod go.sum
git commit -m "feat: detect leading emoji groups"
```

---

### Task 2: Group rows in `taskRows`

**Files:**
- Modify: `tasks.go:50-53` (`taskRow`), `tasks.go:176-181`, `tasks.go:340-379` (`taskRows`)
- Modify: `tasks.go:384-402` and `watch.go:270-281` (call sites, passing `false` for now)
- Test: `tasks_test.go`

**Interfaces:**
- Consumes: `splitEmojiGroup(text string) (key, rest string)` from Task 1.
- Produces: `taskRow{Label, Group, Text string}` and `func taskRows(tasks map[string]Task, verbose, quiet, group bool, grep string) ([]taskRow, int)`.

- [ ] **Step 1: Write the failing group-row tests**

Append to `tasks_test.go`:

```go
func TestTaskRows_GroupedOrder(t *testing.T) {
	rows, width := taskRows(map[string]Task{
		"a1": {ID: "a1", Text: "🚀 Launch"},
		"b2": {ID: "b2", Text: "🍕 Pizza"},
		"c3": {ID: "c3", Text: "No emoji"},
		"d4": {ID: "d4", Text: "🍕 Pasta"},
		"e5": {ID: "e5", Text: "🏃 Run"},
	}, false, false, true, "")

	want := []taskRow{
		{Label: "b", Group: "🍕", Text: "Pizza"},
		{Label: "d", Group: "🍕", Text: "Pasta"},
		{Label: "e", Group: "🏃", Text: "Run"},
		{Label: "a", Group: "🚀", Text: "Launch"},
		{Label: "c", Text: "No emoji"},
	}

	if !slices.Equal(rows, want) {
		t.Fatalf("rows = %+v, want %+v", rows, want)
	}

	if width != 1 {
		t.Fatalf("width = %d, want 1 from all ids", width)
	}
}

func TestTaskRows_GroupedGrepMatchesStoredText(t *testing.T) {
	rows, width := taskRows(map[string]Task{
		"a1": {ID: "a1", Text: "🚀 Launch"},
		"b2": {ID: "b2", Text: "🍕 Pizza"},
		"d4": {ID: "d4", Text: "🍕 Pasta"},
	}, false, false, true, "🍕")

	want := []taskRow{
		{Label: "b", Group: "🍕", Text: "Pizza"},
		{Label: "d", Group: "🍕", Text: "Pasta"},
	}

	if !slices.Equal(rows, want) {
		t.Fatalf("rows = %+v, want %+v", rows, want)
	}

	if width != 1 {
		t.Fatalf("width = %d, want 1 from all ids", width)
	}
}

func TestTaskRows_GroupedQuiet(t *testing.T) {
	rows, width := taskRows(map[string]Task{"b2": {ID: "b2", Text: "🍕 Pizza"}}, true, true, true, "")

	if len(rows) != 1 || rows[0].Label != "" || rows[0].Group != "🍕" || rows[0].Text != "Pizza" {
		t.Fatalf("rows = %+v, want a text-only grouped row", rows)
	}

	if width != 0 {
		t.Fatalf("width = %d, want 0 in quiet mode", width)
	}
}

func TestTaskRows_GroupedEmojiOnlyKeepsText(t *testing.T) {
	rows, _ := taskRows(map[string]Task{"b2": {ID: "b2", Text: "🍕"}}, false, false, true, "")

	if len(rows) != 1 || rows[0].Group != "🍕" || rows[0].Text != "🍕" {
		t.Fatalf("rows = %+v, want the emoji kept as display text", rows)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run '^TestTaskRows_Grouped'`
Expected: FAIL to compile, `too many arguments in call to taskRows` (and `unknown field Group`).

- [ ] **Step 3: Implement grouped rows and fix call sites**

In `tasks.go`:

```go
type taskRow struct {
	Label string
	Group string
	Text  string
}
```

`taskRows` keeps today's empty-list early return and its `buildLabelFn(ids, verbose)` call before any sorting or filtering. Then branch:

```go
if !group {
	sort.Strings(ids)
	// existing loop, unchanged
	...
}

keys := make(map[string]string, len(ids))
display := make(map[string]string, len(ids))

for _, id := range ids {
	keys[id], display[id] = splitEmojiGroup(tasks[id].Text)
}

sort.Slice(ids, func(i, j int) bool {
	gi, gj := keys[ids[i]], keys[ids[j]]

	switch {
	case gi == gj:
		return ids[i] < ids[j]
	case gi == "":
		return false
	case gj == "":
		return true
	default:
		return gi < gj
	}
})

for _, id := range ids {
	task := tasks[id]
	if grep != "" && !strings.Contains(strings.ToLower(task.Text), strings.ToLower(grep)) {
		continue
	}

	row := taskRow{Group: keys[id], Text: display[id]}
	if !quiet {
		row.Label = labelFn(id)
	}

	rows = append(rows, row)
}

if quiet {
	return rows, 0
}

return rows, labelWidth
```

Keep the quiet and width rules identical to the ungrouped branch: grep tests the stored `task.Text`, labels come from all IDs, and quiet forces width 0.

Update the two call sites to pass `false` for the new argument:

- `TaskList.List`: `taskRows(tasks, verbose, quiet, false, grep)`
- `watchModel.load`: `taskRows(tasks.Tasks, m.config.verbose, m.config.quiet, false, m.config.grep)`

Update the four existing `TestTaskRows_*` calls to pass `false` before the grep argument.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -run '^TestTaskRows_'`
Expected: PASS, including the existing label, grep, quiet, and empty tests.

- [ ] **Step 5: Run the full checks and commit**

Run: `mise run test && mise run lint && mise run check`
Expected: all pass.

```bash
git add tasks.go watch.go tasks_test.go
git commit -m "feat: group task rows by leading emoji"
```

---

### Task 3: Render grouped output in `List`

**Files:**
- Modify: `tasks.go:384-402` (`List`)
- Modify: `main.go:90-99` and `main.go:101-110` (pass `false` for now)
- Modify: `watch_test.go:446-460` (`renderTaskList` helper)
- Test: `tasks_test.go`

**Interfaces:**
- Consumes: `taskRows(..., group bool, grep string)` and `taskRow.Group` from Task 2.
- Produces: `func (tl *TaskList) List(w io.Writer, kind string, verbose, quiet, group bool, grep string)`.

- [ ] **Step 1: Write the failing grouped-list tests**

Append to `tasks_test.go`:

```go
func TestList_GroupedExactOutput(t *testing.T) {
	tasks := map[string]Task{
		"a1": {ID: "a1", Text: "🚀 Launch"},
		"b2": {ID: "b2", Text: "🍕 Pizza"},
		"c3": {ID: "c3", Text: "No emoji"},
		"d4": {ID: "d4", Text: "🍕 Pasta"},
		"e5": {ID: "e5", Text: "🏃 Run"},
	}

	tests := []struct {
		name  string
		quiet bool
		grep  string
		want  string
	}{
		{
			name: "grouped",
			want: "🍕\n  b - Pizza\n  d - Pasta\n\n🏃\n  e - Run\n\n🚀\n  a - Launch\n\nc - No emoji\n",
		},
		{
			name:  "quiet",
			quiet: true,
			want:  "🍕\n  Pizza\n  Pasta\n\n🏃\n  Run\n\n🚀\n  Launch\n\nNo emoji\n",
		},
		{
			name: "grep on emoji",
			grep: "🍕",
			want: "🍕\n  b - Pizza\n  d - Pasta\n",
		},
		{
			name: "grep inside group",
			grep: "pasta",
			want: "🍕\n  d - Pasta\n",
		},
		{
			name: "grep matches nothing",
			grep: "nope",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := &TaskList{Tasks: tasks, Done: map[string]Task{}, Name: "tasks", TaskDir: t.TempDir()}

			var buf strings.Builder

			tl.List(&buf, "tasks", false, tt.quiet, true, tt.grep)

			if got := buf.String(); got != tt.want {
				t.Fatalf("List output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestList_GroupedWithoutEmojiMatchesUngrouped(t *testing.T) {
	tl := newTestTaskList()
	tl.Tasks["aaa"] = Task{ID: "aaa", Text: "First"}
	tl.Tasks["bbb"] = Task{ID: "bbb", Text: "Second"}

	var flat, grouped strings.Builder

	tl.List(&flat, "tasks", false, false, false, "")
	tl.List(&grouped, "tasks", false, false, true, "")

	if flat.String() != grouped.String() {
		t.Fatalf("grouped output = %q, want byte-identical to %q", grouped.String(), flat.String())
	}
}

func TestList_GroupedVerboseAndDone(t *testing.T) {
	longID := "0123456789abcdef0123456789abcdef01234567"
	tl := &TaskList{
		Tasks:   map[string]Task{},
		Done:    map[string]Task{longID: {ID: longID, Text: "🍕 Done task"}},
		Name:    "tasks",
		TaskDir: t.TempDir(),
	}

	var buf strings.Builder

	tl.List(&buf, "done", true, false, true, "")

	if want := "🍕\n  " + longID + " - Done task\n"; buf.String() != want {
		t.Fatalf("List output = %q, want %q", buf.String(), want)
	}
}
```

Update every existing `tl.List(...)` call in `tasks_test.go` and the `renderTaskList` helper in `watch_test.go` to pass `false` for the new `group` argument, immediately before `grep`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run '^TestList_Grouped'`
Expected: FAIL to compile, `too many arguments in call to List`.

- [ ] **Step 3: Implement grouped `List`**

Change the signature and delegate row printing to one helper:

```go
func writeTaskRow(w io.Writer, row taskRow, width int, indented bool) {
	indent := ""
	if indented {
		indent = "  "
	}

	if width == 0 {
		fmt.Fprintln(w, indent+row.Text)

		return
	}

	fmt.Fprintf(w, "%s%-*s - %s\n", indent, width, row.Label, row.Text)
}
```

In `List`:

```go
rows, width := taskRows(tasks, verbose, quiet, group, grep)

if !group {
	for _, row := range rows {
		writeTaskRow(w, row, width, false)
	}

	return
}

current := ""

for i, row := range rows {
	if i == 0 || row.Group != current {
		if i > 0 {
			fmt.Fprintln(w)
		}

		if row.Group != "" {
			fmt.Fprintln(w, row.Group)
		}

		current = row.Group
	}

	writeTaskRow(w, row, width, row.Group != "")
}
```

Keep the existing `kind == "done"` selection and the empty-result behavior. In `main.go`, pass `false` as the new `group` argument in `listAction` and `doneAction` until Task 5 wires the flag.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -run '^TestList_'`
Expected: PASS, including every pre-existing exact-output case.

- [ ] **Step 5: Run the full checks and commit**

Run: `mise run test && mise run lint && mise run check`
Expected: all pass.

```bash
git add tasks.go main.go tasks_test.go watch_test.go
git commit -m "feat: render grouped task lists"
```

---

### Task 4: Render grouped watch bodies

**Files:**
- Modify: `watch.go:24-30` (`watchConfig`), `watch.go:139-200` (`screenLines`, `renderBody`), `watch.go:270-281` (`load`)
- Modify: `watch_test.go:446-460` (`renderTaskList` helper)
- Test: `watch_test.go`

**Interfaces:**
- Consumes: `taskRows(..., group bool, grep string)`, `taskRow.Group`, and `splitEmojiGroup` indirectly.
- Produces: `watchConfig` gains `group bool`; `type watchLine struct { text string; heading bool }`; `func (m watchModel) renderBody() []watchLine`.

- [ ] **Step 1: Write the failing watch tests**

Append to `watch_test.go`:

```go
func TestWatchModelGroupsTasksIntoNestedList(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"a1": {ID: "a1", Text: "🚀 Launch"},
		"b2": {ID: "b2", Text: "🍕 Pizza"},
		"c3": {ID: "c3", Text: "No emoji"},
		"d4": {ID: "d4", Text: "🍕 Pasta"},
		"e5": {ID: "e5", Text: "🏃 Run"},
	})

	model := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks", group: true}))

	want := []string{
		"🍕", "  • b - Pizza", "  • d - Pasta",
		"🏃", "  • e - Run",
		"🚀", "  • a - Launch",
		"• c - No emoji",
	}

	got := watchBody(model.View().Content)
	if len(got) != len(want)+1 || !slices.Equal(got[:len(want)], want) ||
		!strings.HasPrefix(got[len(want)], "Sun") {
		t.Fatalf("view lines = %q, want %q then the bar", got, want)
	}
}

func TestWatchModelGroupingWithoutEmojiMatchesFlat(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "First"},
		"bbb": {ID: "bbb", Text: "Second"},
	})

	flat := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"}))
	grouped := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks", group: true}))

	if flat.View().Content != grouped.View().Content {
		t.Fatalf("grouped view = %q, want byte-identical to %q", grouped.View().Content, flat.View().Content)
	}
}

func TestWatchModelGroupedOverflowCountsOnlyTasks(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "🍕 One"},
		"aab": {ID: "aab", Text: "🍕 Two"},
		"bbb": {ID: "bbb", Text: "🚀 Three"},
		"bbc": {ID: "bbc", Text: "🚀 Four"},
	})

	model := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks", group: true}))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 40, Height: 4})

	want := []string{"🍕", "  • aaa - One", "… +3 more"}
	lines := watchBody(updated.(watchModel).View().Content)

	if len(lines) != 4 || !slices.Equal(lines[:3], want) || !strings.HasPrefix(lines[3], "Sun") {
		t.Fatalf("lines = %q, want %q then the bar", lines, want)
	}
}

func TestWatchModelGroupedHeightBudgetDropsDanglingHeading(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "🍕 One"},
		"bbb": {ID: "bbb", Text: "🚀 Two"},
		"ccc": {ID: "ccc", Text: "🚀 Three"},
	})

	model := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks", group: true}))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 40, Height: 5})

	want := []string{"🍕", "  • aaa - One", "… +2 more"}
	lines := watchBody(updated.(watchModel).View().Content)

	if len(lines) != 5 || !slices.Equal(lines[:3], want) ||
		strings.TrimSpace(lines[3]) != "" || !strings.HasPrefix(lines[4], "Sun") {
		t.Fatalf("lines = %q, want %q, a blank row, then the bar", lines, want)
	}
}

func TestWatchModelGroupedErrorDisplacesHeadingBudget(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "🍕 One"},
		"aab": {ID: "aab", Text: "🍕 Two"},
	})

	model := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks", group: true}))
	updated, _ := model.Update(watchLoadResult{err: errors.New("refresh failed")})
	updated, _ = updated.(watchModel).Update(tea.WindowSizeMsg{Width: 40, Height: 3})

	want := []string{"error: refresh failed", "… +2 more"}
	lines := watchBody(updated.(watchModel).View().Content)

	if len(lines) != 3 || !slices.Equal(lines[:2], want) || !strings.HasPrefix(lines[2], "Sun") {
		t.Fatalf("lines = %q, want the error, the marker, then the bar", lines)
	}
}

func TestWatchModelTruncatesGroupHeadingToWidth(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{"aaa": {ID: "aaa", Text: "🍕🍕🍕 Pizza"}})

	model := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks", group: true}))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 4, Height: 4})

	lines := watchBody(updated.(watchModel).View().Content)
	for i, line := range lines {
		if got := lipgloss.Width(line); got > 4 {
			t.Fatalf("line %d is %d cells wide: %q", i, got, line)
		}
	}

	if !strings.Contains(lines[0], "…") {
		t.Fatalf("heading line = %q, want an ellipsis", lines[0])
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run '^TestWatchModel(Group|Grouping|Groups|TruncatesGroup)'`
Expected: FAIL to compile, `unknown field group in struct literal`.

- [ ] **Step 3: Implement grouped watch bodies**

In `watch.go`:

1. Add `group bool` to `watchConfig`.
2. In `load`, call `taskRows(tasks.Tasks, m.config.verbose, m.config.quiet, m.config.group, m.config.grep)`.
3. Add:

```go
// watchLine is one body row; heading marks group emoji lines.
type watchLine struct {
	text    string
	heading bool
}
```

4. Replace `renderBody` with a `[]watchLine` builder. Factor the existing item construction into:

```go
// itemStrings renders the label/text cells for a block of rows.
func (m watchModel) itemStrings(rows []taskRow) []any {
	items := make([]any, 0, len(rows))

	for _, row := range rows {
		if row.Label == "" {
			items = append(items, row.Text)

			continue
		}

		items = append(items, watchLabelStyle.Width(m.labelWidth).Render(row.Label)+" - "+row.Text)
	}

	return items
}
```

Then walk consecutive rows with the same `Group`. For an emoji block, render one nested list and mark its first line as the heading:

```go
nested := list.New(m.itemStrings(block)...)
rendered := list.New(block[0].Group, nested).
	Enumerator(func(list.Items, int) string { return "" }).
	EnumeratorStyle(lipgloss.NewStyle()).
	String()

for i, line := range strings.Split(rendered, "\n") {
	lines = append(lines, watchLine{text: line, heading: i == 0})
}
```

For the ungrouped block (`Group == ""`), render `list.New(m.itemStrings(block)...)` with the default bullet and append every line with `heading: false`. When every row is ungrouped this is exactly today's body.

5. Teach `screenLines` about headings. Keep the existing shape and precedence, but operate on `[]watchLine`:

```go
body := m.renderBody()
...
if len(body) <= avail {
	lines = append(lines, textLines(body)...)
} else if avail > 0 {
	shown := body[:avail-1]

	hidden := 0

	for _, line := range body[avail-1:] {
		if !line.heading {
			hidden++
		}
	}

	if len(shown) > 0 && shown[len(shown)-1].heading {
		shown = shown[:len(shown)-1]
	}

	lines = append(lines, textLines(shown)...)
	lines = append(lines, fmt.Sprintf("… +%d more", hidden))
}
```

Add a small `textLines([]watchLine) []string` projector. Keep the unknown-height path (no padding, no marker), the height-1 paths, and the error-first ordering unchanged.

6. Update the `renderTaskList` helper in `watch_test.go` to pass `config.group`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -run '^TestWatchModel'`
Expected: PASS, including every pre-existing watch test.

- [ ] **Step 5: Run the full checks and commit**

Run: `mise run test && mise run lint && mise run check`
Expected: all pass.

```bash
git add watch.go watch_test.go
git commit -m "feat(watch): render emoji groups as nested lists"
```

---

### Task 5: Add the `--group` flag and docs

**Files:**
- Modify: `main.go:78-88` (`watchConfigFromCommand`), `main.go:90-110` (actions), `main.go:249-268` (`listFlags`)
- Test: `main_test.go`
- Modify: `README.md:35-63`

**Interfaces:**
- Consumes: grouped `List` and `watchConfig.group` from Tasks 3 and 4.
- Produces: `--group` / `-G` on `list`, `done`, and `watch`; `watchConfigFromCommand` sets `group`.

- [ ] **Step 1: Write the failing CLI tests**

Add `slices` and `github.com/urfave/cli/v3` to the `main_test.go` imports and append:

```go
func TestCLI_GroupFlag(t *testing.T) {
	app := buildApp()

	for _, name := range []string{"list", "done", "watch"} {
		var command *cli.Command

		for _, candidate := range app.Commands {
			if candidate.Name == name {
				command = candidate

				break
			}
		}

		if command == nil {
			t.Fatalf("%s command is missing", name)
		}

		var group, grep cli.Flag

		for _, flag := range command.Flags {
			switch flag.Names()[0] {
			case "group":
				group = flag
			case "grep":
				grep = flag
			}
		}

		if group == nil {
			t.Fatalf("%s command is missing the group flag", name)
		}

		if !slices.Contains(group.Names(), "G") {
			t.Fatalf("%s group flag has no -G alias: %v", name, group.Names())
		}

		if grep == nil || !slices.Contains(grep.Names(), "g") {
			t.Fatalf("%s grep flag lost its -g alias", name)
		}
	}
}
```

Add a case to `TestWatchConfigFromCommand`:

```go
"group": {
	args: []string{"watch", "--group"},
	want: func(dir string) watchConfig {
		return watchConfig{taskDir: dir, listName: "tasks", group: true}
	},
},
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run 'TestCLI_GroupFlag|TestWatchConfigFromCommand'`
Expected: FAIL, `list command is missing the group flag` (or the zero-value `watchConfig` mismatch).

- [ ] **Step 3: Add the flag and wire it**

Add to `listFlags()`:

```go
&cli.BoolFlag{
	Name:    "group",
	Aliases: []string{"G"},
	Usage:   "group tasks by leading emoji",
},
```

Then:

- `listAction`: pass `cmd.Bool("group")` to `List`.
- `doneAction`: pass `cmd.Bool("group")` to `List`.
- `watchConfigFromCommand`: add `group: cmd.Bool("group")` to the returned struct.

Update `README.md` usage with:

```bash
# Group tasks by their leading emoji
tgo list --group
tgo watch -G
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -run 'TestCLI_GroupFlag|TestWatchConfigFromCommand'`
Expected: PASS.

Manual smoke check in a scratch directory:

```bash
dir="$(mktemp -d)"
go run . --task-dir "$dir" add "🍕 Pizza"
go run . --task-dir "$dir" list --group
```

Expect a `🍕` heading, a `  <id> - Pizza` row, and no error.

- [ ] **Step 5: Run the full checks and commit**

Run: `mise run test && mise run lint && mise run check`
Expected: all pass.

```bash
git add main.go main_test.go README.md
git commit -m "feat: add --group flag"
```

---

## Final Verification and Pull Request

- [ ] Run `mise run test`, `mise run lint`, `mise run check` on the final branch.
- [ ] Validate the PR title: `mise run pr-title -- "feat: group tasks by leading emoji"`.
- [ ] Push: `git push -u origin nrh/groups`.
- [ ] Open the pull request:

```bash
gh pr create --title "feat: group tasks by leading emoji" --body "$(cat <<'EOF'
## Summary
- add `--group` / `-G` to list, done, and watch
- group tasks by their leading emoji run, sorted lexicographically with ungrouped tasks last
- render list headings and a lipgloss nested list in watch
- keep grouping display-only; file format and IDs unchanged

## Testing
- mise run test
- mise run lint
- mise run check
EOF
)"
```

- [ ] Confirm with `gh pr view`.
