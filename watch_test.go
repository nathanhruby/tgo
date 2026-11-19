package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func writeWatchTasks(t *testing.T, tasks map[string]Task) string {
	t.Helper()
	dir := t.TempDir()
	writeWatchTasksTo(t, dir, tasks)

	return dir
}

func writeWatchTasksTo(t *testing.T, dir string, tasks map[string]Task) {
	t.Helper()

	tl := &TaskList{Tasks: tasks, Done: map[string]Task{}, Name: "tasks", TaskDir: dir}
	if err := tl.Write(false); err != nil {
		t.Fatal(err)
	}
}

// watchBody returns the view with styling stripped, as a list of lines.
func watchBody(view string) []string {
	view = strings.TrimSuffix(ansi.Strip(view), "\n")
	if view == "" {
		return nil
	}

	return strings.Split(view, "\n")
}

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
	}
}

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

	for i, want := range []string{"• a - First", "• b - Second"} {
		if !strings.HasPrefix(lines[i], "• ") || strings.TrimSpace(lines[i]) != want {
			t.Fatalf("line %d = %q, want the bulleted task %q", i, lines[i], want)
		}
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

	want := []string{"• a - One", "• b - Two", "… +3 more"}
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

		if got := lipgloss.Width(lines[len(lines)-1]); got != width {
			t.Fatalf("width %d: bar is %d cells wide, want exactly %d: %q", width, got, width, lines[len(lines)-1])
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

	for i, line := range lines[:3] {
		if strings.TrimSpace(line) != "" {
			t.Fatalf("line %d = %q, want a blank body row", i, line)
		}
	}
}

func TestWatchModelRendersLipglossList(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "First"},
		"aab": {ID: "aab", Text: "Second"},
		"bbb": {ID: "bbb", Text: "Third"},
	})

	model := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"}))

	want := []string{"• aaa - First", "• aab - Second", "• b   - Third"}

	got := watchBody(model.View().Content)
	if len(got) != len(want)+1 || !slices.Equal(got[:len(want)], want) || !strings.HasPrefix(got[len(want)], "Sun") {
		t.Fatalf("view lines = %q, want %q then the bar", got, want)
	}
}

func TestWatchModelQuietRendersBulletsWithoutLabels(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{"aaa": {ID: "aaa", Text: "First"}})

	model := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks", quiet: true}))

	got := watchBody(model.View().Content)
	if len(got) != 2 || got[0] != "• First" || !strings.HasPrefix(got[1], "Sun") {
		t.Fatalf("view lines = %q, want the bullet then the bar", got)
	}
}

func TestWatchModelBodyMatchesListOutput(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "First"},
		"aab": {ID: "aab", Text: "Second"},
	})

	config := watchConfig{taskDir: dir, listName: "tasks"}
	model := loadWatch(t, newTestWatchModel(t, config))

	lines := watchBody(model.View().Content)
	listLines := strings.Split(strings.TrimSuffix(renderTaskList(t, config), "\n"), "\n")

	if len(lines) != len(listLines)+1 {
		t.Fatalf("watch view has %d lines, want %d list lines plus the bar", len(lines), len(listLines))
	}

	body := lines[:len(listLines)]
	for i := range listLines {
		if want := "• " + listLines[i]; body[i] != want {
			t.Fatalf("line %d = %q, want %q", i, body[i], want)
		}
	}
}

func TestWatchModelLoadsTasksImmediately(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{"abc": {ID: "abc", Text: "A watched task"}})

	model := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"}))

	view := model.View().Content

	if !strings.Contains(view, "A watched task") {
		t.Fatalf("watch view does not contain the task list: %q", view)
	}

	got := watchBody(view)
	if len(got) != 2 || got[0] != "• a - A watched task" || !strings.HasPrefix(got[1], "Sun Nov 15 2026") {
		t.Fatalf("watch view lines = %q, want the task then the bar", got)
	}
}

func TestWatchModelForwardsListOptions(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"abc": {ID: "abc", Text: "A watched task"},
		"def": {ID: "def", Text: "A hidden task"},
	})

	for name, options := range map[string]watchConfig{
		"default": {},
		"quiet":   {quiet: true},
		"verbose": {verbose: true},
		"grep":    {grep: "watched"},
	} {
		t.Run(name, func(t *testing.T) {
			config := options
			config.taskDir = dir
			config.listName = "tasks"

			model := loadWatch(t, newTestWatchModel(t, config))
			lines := watchBody(model.View().Content)
			got := strings.Join(lines, "\n")

			var want []string
			for _, line := range strings.Split(strings.TrimSuffix(renderTaskList(t, config), "\n"), "\n") {
				want = append(want, "• "+line)
			}

			if len(lines) != len(want)+1 || !slices.Equal(lines[:len(want)], want) ||
				!strings.HasPrefix(lines[len(want)], "Sun") {
				t.Fatalf("view lines = %q, want %q then the bar", lines, want)
			}

			if options.quiet && strings.Contains(got, "abc") {
				t.Fatalf("quiet view = %q, want no task IDs", got)
			}

			if options.verbose && !strings.Contains(got, "abc") {
				t.Fatalf("verbose view = %q, want full task IDs", got)
			}
		})
	}
}

// renderTaskList renders the same list through TaskList.List so watch output
// can be compared against list output.
func renderTaskList(t *testing.T, config watchConfig) string {
	t.Helper()

	tl, err := NewTaskList(config.taskDir, config.listName)
	if err != nil {
		t.Fatal(err)
	}

	var rendered strings.Builder
	tl.List(&rendered, "tasks", config.verbose, config.quiet, false, config.grep)

	return rendered.String()
}

func TestWatchModelSchedulesNextRefresh(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{"abc": {ID: "abc", Text: "A watched task"}})

	var got time.Duration

	model := newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"})
	model.schedule = func(d time.Duration) tea.Cmd {
		got = d

		return nil
	}
	loadWatch(t, model)

	if got != 2*time.Second {
		t.Fatalf("schedule duration = %v, want 2s", got)
	}
}

func TestWatchModelTimerReloadsTaskFile(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{"abc": {ID: "abc", Text: "A watched task"}})
	model := newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"})

	_, cmd := model.Update(watchTimerMsg{})
	if cmd == nil {
		t.Fatal("timer did not return a reload command")
	}

	updated, _ := model.Update(cmd())
	if !strings.Contains(updated.(watchModel).View().Content, "A watched task") {
		t.Fatal("reload command did not load task file")
	}
}

func TestWatchModelReloadsChangedFile(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{"abc": {ID: "abc", Text: "Original text"}})
	model := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"}))

	if view := model.View().Content; !strings.Contains(view, "Original text") {
		t.Fatalf("initial view = %q, want original task", view)
	}

	writeWatchTasksTo(t, dir, map[string]Task{"abc": {ID: "abc", Text: "Changed text"}})

	_, load := model.Update(watchTimerMsg{})
	if load == nil {
		t.Fatal("timer did not return a reload command")
	}

	updated, _ := model.Update(load())
	view := updated.(watchModel).View().Content

	if !strings.Contains(view, "Changed text") {
		t.Fatalf("view after refresh = %q, want changed text", view)
	}

	if strings.Contains(view, "Original text") {
		t.Fatalf("view after refresh = %q, still shows stale text", view)
	}
}

func TestWatchModelKeepsErrorVisibleWhenOutputOverflowsTerminal(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "First task"},
		"bbb": {ID: "bbb", Text: "Second task"},
		"ccc": {ID: "ccc", Text: "Third task"},
		"ddd": {ID: "ddd", Text: "Fourth task"},
		"eee": {ID: "eee", Text: "Fifth task"},
	})

	model := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"}))
	updated, _ := model.Update(watchLoadResult{err: errors.New("refresh failed")})
	updated, _ = updated.(watchModel).Update(tea.WindowSizeMsg{Height: 3})

	lines := watchBody(updated.(watchModel).View().Content)

	want := []string{"error: refresh failed", "… +5 more"}
	if len(lines) != 3 || !slices.Equal(lines[:2], want) || !strings.HasPrefix(lines[2], "Sun") {
		t.Fatalf("lines = %q, want the error, an overflow marker for all 5 tasks, then the bar", lines)
	}
}

func TestWatchModelShowsWholeListWhenTerminalFits(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "First task"},
		"bbb": {ID: "bbb", Text: "Second task"},
	})

	model := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"}))
	updated, _ := model.Update(watchLoadResult{err: errors.New("refresh failed")})
	updated, _ = updated.(watchModel).Update(tea.WindowSizeMsg{Height: 10})

	lines := watchBody(updated.(watchModel).View().Content)

	want := []string{"error: refresh failed", "• a - First task", "• b - Second task", "", "", "", "", "", "", ""}
	if len(lines) != 10 || !slices.Equal(lines[:9], want[:9]) || !strings.HasPrefix(lines[9], "Sun") {
		t.Fatalf("lines = %q, want the error, two tasks, six blanks, then the bar", lines)
	}
}

func TestWatchModelInitialLoadErrorIsVisible(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}

	var scheduled time.Duration

	model := newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"})
	model.schedule = func(d time.Duration) tea.Cmd {
		scheduled = d

		return nil
	}

	model = loadWatch(t, model)

	lines := watchBody(model.View().Content)
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "error:") || !strings.HasPrefix(lines[1], "Sun") {
		t.Fatalf("lines = %q, want the load error then the bar", lines)
	}

	if scheduled != watchRefreshInterval {
		t.Fatalf("scheduled retry = %v, want %v", scheduled, watchRefreshInterval)
	}
}

func TestWatchModelClearsErrorAfterSuccessfulRefresh(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{"abc": {ID: "abc", Text: "First task"}})
	model := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"}))

	path := filepath.Join(dir, "tasks")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	_, load := model.Update(watchTimerMsg{})
	updated, _ := model.Update(load())
	model = updated.(watchModel)

	if lines := watchBody(model.View().Content); len(lines) == 0 || !strings.HasPrefix(lines[0], "error:") {
		t.Fatalf("view lines = %q, want the load error first", lines)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	writeWatchTasksTo(t, dir, map[string]Task{"def": {ID: "def", Text: "Second task"}})

	_, load = model.Update(watchTimerMsg{})
	updated, _ = model.Update(load())
	lines := watchBody(updated.(watchModel).View().Content)

	if len(lines) != 2 || lines[0] != "• d - Second task" || !strings.HasPrefix(lines[1], "Sun") {
		t.Fatalf("view lines = %q, want only the refreshed task and the bar", lines)
	}
}

func TestWatchModelRetainsOutputOnRefreshError(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{"abc": {ID: "abc", Text: "A watched task"}})
	model := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"}))

	updated, _ := model.Update(watchLoadResult{err: errors.New("refresh failed")})
	view := updated.(watchModel).View().Content

	got := watchBody(view)

	want := []string{"error: refresh failed", "• a - A watched task"}
	if len(got) != 3 || !slices.Equal(got[:2], want) || !strings.HasPrefix(got[2], "Sun") {
		t.Fatalf("error view lines = %q, want %q then the bar", got, want)
	}
}

func TestWatchModelExcludesFinishedTasks(t *testing.T) {
	dir := t.TempDir()

	tl := &TaskList{
		Tasks:   map[string]Task{"abc": {ID: "abc", Text: "Open task"}},
		Done:    map[string]Task{"def": {ID: "def", Text: "Finished task"}},
		Name:    "tasks",
		TaskDir: dir,
	}
	if err := tl.Write(false); err != nil {
		t.Fatal(err)
	}

	model := loadWatch(t, newTestWatchModel(t, watchConfig{taskDir: dir, listName: "tasks"}))
	view := model.View().Content

	if !strings.Contains(view, "Open task") {
		t.Fatalf("view = %q, want the open task", view)
	}

	if strings.Contains(view, "Finished task") {
		t.Fatalf("view = %q, want finished tasks excluded", view)
	}
}

func TestWatchModelRetriesAfterRefreshErrorAndRetainsOutput(t *testing.T) {
	var got time.Duration

	model := newTestWatchModel(t, watchConfig{})
	model.schedule = func(d time.Duration) tea.Cmd {
		got = d

		return nil
	}
	updated, _ := model.Update(watchLoadResult{rows: []taskRow{{Text: "successful output"}}})
	model = updated.(watchModel)

	updated, cmd := model.Update(watchLoadResult{err: errors.New("refresh failed")})
	model = updated.(watchModel)

	if len(model.rows) != 1 || model.rows[0].Text != "successful output" {
		t.Fatalf("rows = %v, want retained successful output", model.rows)
	}

	lines := watchBody(model.View().Content)

	want := []string{"error: refresh failed", "• successful output"}
	if len(lines) != 3 || !slices.Equal(lines[:2], want) || !strings.HasPrefix(lines[2], "Sun") {
		t.Fatalf("view lines = %q, want %q then the bar", lines, want)
	}

	if got != watchRefreshInterval {
		t.Fatalf("schedule duration = %v, want %v", got, watchRefreshInterval)
	}

	if cmd != nil {
		t.Fatal("test scheduler command should be nil")
	}
}

func TestWatchModelTimerCycleSchedulesExactlyOneSubsequentTimer(t *testing.T) {
	var schedules int

	model := newWatchModel(watchConfig{})

	model.schedule = func(time.Duration) tea.Cmd {
		schedules++

		return func() tea.Msg { return watchTimerMsg{} }
	}

	_, load := model.Update(watchTimerMsg{})
	if load == nil {
		t.Fatal("timer did not return a reload command")
	}

	updated, timer := model.Update(load())

	if timer == nil {
		t.Fatal("load result did not return a timer command")
	}

	if schedules != 1 {
		t.Fatalf("schedule count = %d, want 1", schedules)
	}

	if _, nextLoad := updated.(watchModel).Update(timer()); nextLoad == nil {
		t.Fatal("subsequent timer did not return a reload command")
	}

	if schedules != 1 {
		t.Fatalf("schedule count after timer = %d, want exactly 1", schedules)
	}
}

func TestWatchModelZeroValueSchedulesRefresh(t *testing.T) {
	var model watchModel

	if _, cmd := model.Update(watchLoadResult{rows: []taskRow{{Text: "text"}}}); cmd == nil {
		t.Fatal("zero-value model did not schedule a refresh")
	}
}

func TestWatchModelZeroValueClockRendersNoDate(t *testing.T) {
	model := watchModel{width: 80, config: watchConfig{listName: "tasks"}}

	lines := watchBody(model.View().Content)
	bar := lines[len(lines)-1]

	if strings.Contains(bar, "0001") || strings.Contains(bar, "Mon Jan") {
		t.Fatalf("bar = %q, want no date text for a zero clock", bar)
	}

	if want := "tasks · 0 open · q to quit"; !strings.HasSuffix(bar, want) {
		t.Fatalf("bar = %q, want suffix %q", bar, want)
	}

	if got := lipgloss.Width(bar); got != 80 {
		t.Fatalf("bar width = %d, want 80: %q", got, bar)
	}
}

func TestWatchModelZeroValueClockIsSafe(t *testing.T) {
	var model watchModel

	if model.nextClockTick() == nil {
		t.Fatal("zero-value model returned no clock tick")
	}

	if model.Init() == nil {
		t.Fatal("zero-value model returned no Init command")
	}

	updated, cmd := model.Update(watchClockMsg{})
	if cmd == nil {
		t.Fatal("clock message scheduled nothing")
	}

	if updated.(watchModel).clock.IsZero() {
		t.Fatal("clock message left the clock zero")
	}
}

func TestWatchModelIgnoresTimerWhileLoadIsInFlight(t *testing.T) {
	model := newWatchModel(watchConfig{taskDir: t.TempDir(), listName: "tasks"})

	updated, load := model.Update(watchTimerMsg{})
	if load == nil {
		t.Fatal("timer did not start a load")
	}

	model = updated.(watchModel)

	if _, cmd := model.Update(watchTimerMsg{}); cmd != nil {
		t.Fatal("a second timer started a concurrent load")
	}

	updated, cmd := model.Update(load())
	if cmd == nil {
		t.Fatal("load result did not schedule the next refresh")
	}

	if _, cmd := updated.(watchModel).Update(watchTimerMsg{}); cmd == nil {
		t.Fatal("timer after the load result did not start a load")
	}
}

func TestWatchModelViewUsesAltScreen(t *testing.T) {
	for name, model := range map[string]watchModel{
		"initial": newWatchModel(watchConfig{}),
		"loaded":  {rows: []taskRow{{Text: "A watched task"}}},
		"error":   {rows: []taskRow{{Text: "A watched task"}}, err: errors.New("refresh failed")},
	} {
		if !model.View().AltScreen {
			t.Fatalf("%s view does not enable the alternate screen", name)
		}
	}
}

func TestWatchModelIgnoresOtherKeysAndReleases(t *testing.T) {
	for name, msg := range map[string]tea.Msg{
		"other key":      tea.KeyPressMsg{Code: 'a', Text: "a"},
		"alt+q":          tea.KeyPressMsg{Code: 'q', Text: "q", Mod: tea.ModAlt},
		"q release":      tea.KeyReleaseMsg{Code: 'q', Text: "q"},
		"ctrl+c release": tea.KeyReleaseMsg{Code: 'c', Mod: tea.ModCtrl},
	} {
		model := newWatchModel(watchConfig{})
		if _, cmd := model.Update(msg); cmd != nil {
			t.Fatalf("%s unexpectedly returned a command", name)
		}
	}
}

func TestWatchModelQuitsOnQAndCtrlC(t *testing.T) {
	for name, key := range map[string]tea.KeyPressMsg{
		"q":      {Code: 'q', Text: "q"},
		"ctrl+c": {Code: 'c', Mod: tea.ModCtrl},
	} {
		model := newWatchModel(watchConfig{})
		_, cmd := model.Update(key)

		if cmd == nil {
			t.Fatalf("%s did not return quit command", name)
		}

		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%s did not produce a quit message", name)
		}
	}
}
