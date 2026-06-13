package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// runApp runs the CLI with the given args and returns the exit error (nil on success).
// It uses a temp dir as the task directory.
func runApp(t *testing.T, dir string, args ...string) error {
	t.Helper()

	allArgs := append([]string{"tgo", "--task-dir", dir}, args...)

	return buildApp().Run(context.Background(), allArgs)
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
