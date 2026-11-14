package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
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

func TestWatchModelLoadsTasksImmediately(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{"abc": {ID: "abc", Text: "A watched task"}})

	model := newWatchModel(watchConfig{taskDir: dir, listName: "tasks"})
	msg := model.Init()()
	updated, _ := model.Update(msg)

	view := updated.(watchModel).View().Content

	if !strings.Contains(view, "a - A watched task") {
		t.Fatalf("watch view does not contain the task list: %q", view)
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

			model := newWatchModel(config)
			updated, _ := model.Update(model.Init()())
			got := updated.(watchModel).View().Content

			if want := renderTaskList(t, config); got != want {
				t.Fatalf("view = %q, want %q", got, want)
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
	tl.List(&rendered, "tasks", config.verbose, config.quiet, config.grep)

	return rendered.String()
}

func TestWatchModelSchedulesNextRefresh(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{"abc": {ID: "abc", Text: "A watched task"}})

	var got time.Duration

	model := newWatchModel(watchConfig{taskDir: dir, listName: "tasks"})
	model.schedule = func(d time.Duration) tea.Cmd {
		got = d

		return nil
	}
	model.Update(model.Init()())

	if got != 2*time.Second {
		t.Fatalf("schedule duration = %v, want 2s", got)
	}
}

func TestWatchModelTimerReloadsTaskFile(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{"abc": {ID: "abc", Text: "A watched task"}})
	model := newWatchModel(watchConfig{taskDir: dir, listName: "tasks"})

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
	model := newWatchModel(watchConfig{taskDir: dir, listName: "tasks"})

	updated, _ := model.Update(model.Init()())
	model = updated.(watchModel)

	if view := model.View().Content; !strings.Contains(view, "Original text") {
		t.Fatalf("initial view = %q, want original task", view)
	}

	writeWatchTasksTo(t, dir, map[string]Task{"abc": {ID: "abc", Text: "Changed text"}})

	_, load := model.Update(watchTimerMsg{})
	if load == nil {
		t.Fatal("timer did not return a reload command")
	}

	updated, _ = model.Update(load())
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

	model := newWatchModel(watchConfig{taskDir: dir, listName: "tasks"})
	updated, _ := model.Update(model.Init()())
	updated, _ = updated.(watchModel).Update(watchLoadResult{err: errors.New("refresh failed")})
	updated, _ = updated.(watchModel).Update(tea.WindowSizeMsg{Height: 3})

	view := updated.(watchModel).View().Content
	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")

	if len(lines) != 3 {
		t.Fatalf("view has %d lines, want 3: %q", len(lines), view)
	}

	if !strings.HasPrefix(lines[0], "error: refresh failed") {
		t.Fatalf("first line = %q, want the error", lines[0])
	}

	if !strings.Contains(lines[2], "+4 more") {
		t.Fatalf("last line = %q, want an overflow marker", lines[2])
	}
}

func TestWatchModelShowsWholeListWhenTerminalFits(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{
		"aaa": {ID: "aaa", Text: "First task"},
		"bbb": {ID: "bbb", Text: "Second task"},
	})

	model := newWatchModel(watchConfig{taskDir: dir, listName: "tasks"})
	updated, _ := model.Update(model.Init()())
	updated, _ = updated.(watchModel).Update(watchLoadResult{err: errors.New("refresh failed")})
	updated, _ = updated.(watchModel).Update(tea.WindowSizeMsg{Height: 10})

	view := updated.(watchModel).View().Content

	for _, text := range []string{"error: refresh failed", "First task", "Second task"} {
		if !strings.Contains(view, text) {
			t.Fatalf("view = %q, want %q", view, text)
		}
	}

	if strings.Contains(view, "more") {
		t.Fatalf("view = %q, want no overflow marker", view)
	}
}

func TestWatchModelInitialLoadErrorIsVisible(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}

	var scheduled time.Duration

	model := newWatchModel(watchConfig{taskDir: dir, listName: "tasks"})
	model.schedule = func(d time.Duration) tea.Cmd {
		scheduled = d

		return nil
	}

	updated, _ := model.Update(model.Init()())
	view := updated.(watchModel).View().Content

	if !strings.Contains(view, "error:") {
		t.Fatalf("view = %q, want the load error", view)
	}

	if scheduled != watchRefreshInterval {
		t.Fatalf("scheduled retry = %v, want %v", scheduled, watchRefreshInterval)
	}
}

func TestWatchModelClearsErrorAfterSuccessfulRefresh(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{"abc": {ID: "abc", Text: "First task"}})
	model := newWatchModel(watchConfig{taskDir: dir, listName: "tasks"})
	model.schedule = func(time.Duration) tea.Cmd { return nil }

	updated, _ := model.Update(model.Init()())
	model = updated.(watchModel)

	path := filepath.Join(dir, "tasks")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	_, load := model.Update(watchTimerMsg{})
	updated, _ = model.Update(load())
	model = updated.(watchModel)

	if !strings.Contains(model.View().Content, "error:") {
		t.Fatalf("view = %q, want the load error", model.View().Content)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	writeWatchTasksTo(t, dir, map[string]Task{"def": {ID: "def", Text: "Second task"}})

	_, load = model.Update(watchTimerMsg{})
	updated, _ = model.Update(load())
	view := updated.(watchModel).View().Content

	if strings.Contains(view, "error:") {
		t.Fatalf("view = %q, want the error cleared", view)
	}

	if !strings.Contains(view, "Second task") {
		t.Fatalf("view = %q, want the refreshed task", view)
	}
}

func TestWatchModelRetainsOutputOnRefreshError(t *testing.T) {
	dir := writeWatchTasks(t, map[string]Task{"abc": {ID: "abc", Text: "A watched task"}})
	model := newWatchModel(watchConfig{taskDir: dir, listName: "tasks"})
	updated, _ := model.Update(model.Init()())
	model = updated.(watchModel)

	model.err = errors.New("refresh failed")
	view := model.View().Content

	if !strings.Contains(view, "A watched task") || !strings.Contains(view, "error: refresh failed") {
		t.Fatalf("error view = %q", view)
	}
}

func TestWatchModelRetriesAfterRefreshErrorAndRetainsOutput(t *testing.T) {
	var got time.Duration

	model := newWatchModel(watchConfig{})
	model.schedule = func(d time.Duration) tea.Cmd {
		got = d

		return nil
	}
	updated, _ := model.Update(watchLoadResult{output: "successful output"})
	model = updated.(watchModel)

	updated, cmd := model.Update(watchLoadResult{err: errors.New("refresh failed")})
	model = updated.(watchModel)

	if model.output != "successful output" {
		t.Fatalf("output = %q, want retained successful output", model.output)
	}

	if !strings.Contains(model.View().Content, "error: refresh failed") {
		t.Fatalf("view = %q, want visible refresh error", model.View().Content)
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

	if _, cmd := model.Update(watchLoadResult{output: "text"}); cmd == nil {
		t.Fatal("zero-value model did not schedule a refresh")
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
		"loaded":  {output: "A watched task\n"},
		"error":   {output: "A watched task\n", err: errors.New("refresh failed")},
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
