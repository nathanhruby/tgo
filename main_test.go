package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// runApp runs the CLI with the given args and returns the exit error (nil on success).
// It uses a temp dir as the task directory.
func runApp(t *testing.T, dir string, args ...string) error {
	t.Helper()

	allArgs := append([]string{"tgo", "--task-dir", dir}, args...)

	return buildApp().Run(context.Background(), allArgs)
}

func TestCLI_WatchCommand(t *testing.T) {
	app := buildApp()
	for _, command := range app.Commands {
		if command.Name != "watch" {
			continue
		}

		for _, name := range []string{"grep", "verbose", "quiet"} {
			found := false

			for _, flag := range command.Flags {
				if flag.Names()[0] == name {
					found = true

					break
				}
			}

			if !found {
				t.Fatalf("watch command is missing %q flag", name)
			}
		}

		return
	}

	t.Fatal("watch command is missing")
}

func TestWatchConfigFromCommand(t *testing.T) {
	originalRunWatchProgram := runWatchProgram

	t.Cleanup(func() { runWatchProgram = originalRunWatchProgram })

	for name, tc := range map[string]struct {
		args []string
		want func(dir string) watchConfig
	}{
		"list options": {
			args: []string{"watch", "--grep", "milk", "--verbose"},
			want: func(dir string) watchConfig {
				return watchConfig{taskDir: dir, listName: "tasks", grep: "milk", verbose: true}
			},
		},
		"quiet and custom list": {
			args: []string{"--list", "work", "watch", "--quiet"},
			want: func(dir string) watchConfig {
				return watchConfig{taskDir: dir, listName: "work", quiet: true}
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()

			var got watchConfig

			runWatchProgram = func(_ context.Context, model tea.Model, _ ...tea.ProgramOption) error {
				got = model.(watchModel).config

				return nil
			}

			args := append([]string{"tgo", "--task-dir", dir}, tc.args...)
			if err := buildApp().Run(context.Background(), args); err != nil {
				t.Fatalf("run %v: %v", args, err)
			}

			if want := tc.want(dir); got != want {
				t.Fatalf("watch config = %+v, want %+v", got, want)
			}
		})
	}
}

func TestWatchActionPassesContext(t *testing.T) {
	type contextKey struct{}

	originalRunWatchProgram := runWatchProgram

	t.Cleanup(func() { runWatchProgram = originalRunWatchProgram })

	var got context.Context

	runWatchProgram = func(ctx context.Context, _ tea.Model, _ ...tea.ProgramOption) error {
		got = ctx

		return nil
	}

	ctx := context.WithValue(context.Background(), contextKey{}, "watch")

	if err := buildApp().Run(ctx, []string{"tgo", "watch"}); err != nil {
		t.Fatalf("run watch: %v", err)
	}

	if got == nil {
		t.Fatal("watch never started a program")
	}

	if value, ok := got.Value(contextKey{}).(string); !ok || value != "watch" {
		t.Fatalf("watch program context = %v, want the CLI context", got)
	}
}

func TestRunWatchProgramStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)

	go func() {
		done <- runWatchProgram(ctx, newWatchModel(watchConfig{taskDir: t.TempDir(), listName: "tasks"}),
			tea.WithInput(strings.NewReader("")), tea.WithoutRenderer())
	}()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "context canceled") {
			t.Fatalf("run watch = %v, want a context cancellation error", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watch did not stop when the context was canceled")
	}
}

func TestCLI_Add(t *testing.T) {
	dir := t.TempDir()
	if err := runApp(t, dir, "add", "Buy more beer"); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	tl, err := NewTaskList(dir, "tasks")
	if err != nil {
		t.Fatal(err)
	}

	if len(tl.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tl.Tasks))
	}
}

func TestCLI_Add_MessageFlag(t *testing.T) {
	dir := t.TempDir()
	if err := runApp(t, dir, "add", "-m", "Buy more beer"); err != nil {
		t.Fatalf("add with message failed: %v", err)
	}

	tl, err := NewTaskList(dir, "tasks")
	if err != nil {
		t.Fatal(err)
	}

	if len(tl.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tl.Tasks))
	}

	for _, task := range tl.Tasks {
		if task.Text != "Buy more beer" {
			t.Errorf("expected task text %q, got %q", "Buy more beer", task.Text)
		}
	}
}

func TestCLI_Add_RequiresText(t *testing.T) {
	dir := t.TempDir()
	if err := runApp(t, dir, "add"); err == nil {
		t.Fatal("expected add without text to fail")
	}
}

func TestCLI_Add_RejectsPositionalTextWithMessageFlag(t *testing.T) {
	dir := t.TempDir()
	if err := runApp(t, dir, "add", "Buy more beer", "-m", "Another task"); err == nil {
		t.Fatal("expected add with positional text and -m to fail")
	}
}

func TestCLI_Add_LongMessageFlag(t *testing.T) {
	dir := t.TempDir()
	if err := runApp(t, dir, "add", "--message", "Walk the dog"); err != nil {
		t.Fatalf("add with long message flag failed: %v", err)
	}

	tl, err := NewTaskList(dir, "tasks")
	if err != nil {
		t.Fatal(err)
	}

	if len(tl.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tl.Tasks))
	}

	for _, task := range tl.Tasks {
		if task.Text != "Walk the dog" {
			t.Errorf("expected task text %q, got %q", "Walk the dog", task.Text)
		}
	}
}

func TestCLI_Add_RejectsAdditionalPositionalText(t *testing.T) {
	dir := t.TempDir()
	if err := runApp(t, dir, "add", "First task", "Second task"); err == nil {
		t.Fatal("expected add with multiple positional arguments to fail")
	}
}

func TestCLI_Add_RejectsPositionalTextWithEmptyMessageFlag(t *testing.T) {
	dir := t.TempDir()
	if err := runApp(t, dir, "add", "Buy more beer", "--message", ""); err == nil {
		t.Fatal("expected add with positional text and an empty --message to fail")
	}
}

func TestCLI_List_Empty(t *testing.T) {
	dir := t.TempDir()
	if err := runApp(t, dir, "list"); err != nil {
		t.Fatalf("list failed: %v", err)
	}
}

func TestCLI_List_Default(t *testing.T) {
	// Running tgo with no subcommand should default to list
	dir := t.TempDir()
	tl := &TaskList{
		Tasks:   map[string]Task{"abc": {ID: "abc", Text: "A task"}},
		Done:    map[string]Task{},
		Name:    "tasks",
		TaskDir: dir,
	}

	if err := tl.Write(false); err != nil {
		t.Fatal(err)
	}
	// Just verify it runs without error (output goes to stdout)
	if err := runApp(t, dir /* no subcommand */); err != nil {
		t.Fatalf("default list failed: %v", err)
	}
}

func TestCLI_Finish(t *testing.T) {
	dir := t.TempDir()
	if err := runApp(t, dir, "add", "Buy more beer"); err != nil {
		t.Fatal(err)
	}

	tl, err := NewTaskList(dir, "tasks")
	if err != nil {
		t.Fatal(err)
	}

	var taskID string
	for id := range tl.Tasks {
		taskID = id
	}

	ps := prefixes([]string{taskID})

	if err := runApp(t, dir, "finish", ps[taskID]); err != nil {
		t.Fatalf("finish failed: %v", err)
	}

	tl2, err := NewTaskList(dir, "tasks")
	if err != nil {
		t.Fatal(err)
	}

	if len(tl2.Tasks) != 0 {
		t.Errorf("expected 0 open tasks after finish, got %d", len(tl2.Tasks))
	}

	if len(tl2.Done) != 1 {
		t.Errorf("expected 1 done task after finish, got %d", len(tl2.Done))
	}
}

func TestCLI_Remove(t *testing.T) {
	dir := t.TempDir()
	if err := runApp(t, dir, "add", "Buy more beer"); err != nil {
		t.Fatal(err)
	}

	tl, err := NewTaskList(dir, "tasks")
	if err != nil {
		t.Fatal(err)
	}

	var taskID string
	for id := range tl.Tasks {
		taskID = id
	}

	ps := prefixes([]string{taskID})

	if err := runApp(t, dir, "remove", ps[taskID]); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	tl2, err := NewTaskList(dir, "tasks")
	if err != nil {
		t.Fatal(err)
	}

	if len(tl2.Tasks) != 0 {
		t.Errorf("expected 0 tasks after remove, got %d", len(tl2.Tasks))
	}
}

func TestCLI_Edit(t *testing.T) {
	dir := t.TempDir()
	if err := runApp(t, dir, "add", "Buy more beer"); err != nil {
		t.Fatal(err)
	}

	tl, err := NewTaskList(dir, "tasks")
	if err != nil {
		t.Fatal(err)
	}

	var taskID string
	for id := range tl.Tasks {
		taskID = id
	}

	ps := prefixes([]string{taskID})

	if err := runApp(t, dir, "edit", ps[taskID], "Buy a lot more beer"); err != nil {
		t.Fatalf("edit failed: %v", err)
	}

	tl2, err := NewTaskList(dir, "tasks")
	if err != nil {
		t.Fatal(err)
	}

	if len(tl2.Tasks) != 1 {
		t.Errorf("expected 1 task after edit, got %d", len(tl2.Tasks))
	}

	for _, task := range tl2.Tasks {
		if task.Text != "Buy a lot more beer" {
			t.Errorf("expected edited text, got %q", task.Text)
		}
	}
}

func TestCLI_Done(t *testing.T) {
	dir := t.TempDir()
	if err := runApp(t, dir, "add", "Buy more beer"); err != nil {
		t.Fatal(err)
	}

	tl, err := NewTaskList(dir, "tasks")
	if err != nil {
		t.Fatal(err)
	}

	var taskID string
	for id := range tl.Tasks {
		taskID = id
	}

	ps := prefixes([]string{taskID})

	if err := runApp(t, dir, "finish", ps[taskID]); err != nil {
		t.Fatal(err)
	}

	// done subcommand should not error
	if err := runApp(t, dir, "done"); err != nil {
		t.Fatalf("done failed: %v", err)
	}
}

func TestCLI_CustomList(t *testing.T) {
	dir := t.TempDir()

	args := []string{"tgo", "--task-dir", dir, "--list", "groceries", "add", "Milk"}
	if err := buildApp().Run(context.Background(), args); err != nil {
		t.Fatalf("add to custom list failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "groceries")); os.IsNotExist(err) {
		t.Error("expected groceries file to exist")
	}
}
