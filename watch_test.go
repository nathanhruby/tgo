package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func writeWatchTasks(t *testing.T, tasks map[string]Task) string {
	t.Helper()
	dir := t.TempDir()

	tl := &TaskList{Tasks: tasks, Done: map[string]Task{}, Name: "tasks", TaskDir: dir}
	if err := tl.Write(false); err != nil {
		t.Fatal(err)
	}

	return dir
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
	model := newWatchModel(watchConfig{taskDir: dir, listName: "tasks", grep: "watched", verbose: true, quiet: false})
	updated, _ := model.Update(model.Init()())
	view := updated.(watchModel).View().Content

	if !strings.Contains(view, "A watched task") || strings.Contains(view, "A hidden task") {
		t.Fatalf("list options were not forwarded: %q", view)
	}
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
