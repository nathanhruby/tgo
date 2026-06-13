# tgo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `tgo`, a Go port of sjl/t — a minimalist task manager that stores tasks in plain text files and is compatible with the original file format.

**Architecture:** Two source files: `tasks.go` owns all task logic (types, parsing, prefix algorithm, file I/O) and `main.go` owns the CLI wiring (urfave/cli v3 with subcommands). Two test files mirror the sources. No package split — everything lives in `package main`.

**Tech Stack:** Go, `github.com/urfave/cli/v3`, `crypto/sha1` (stdlib), `os`/`filepath`/`strings`/`sort` (stdlib)

---

## Task 1: Project Scaffolding

**Files:**
- Create: `tgo/go.mod`
- Create: `tgo/main.go`
- Create: `tgo/tasks.go`
- Create: `tgo/tasks_test.go`
- Create: `tgo/main_test.go`

- [ ] **Step 1: Initialize Go module and fetch dependency**

Run from `tgo/` directory:
```bash
go mod init github.com/nathanhruby/tgo
go get github.com/urfave/cli/v3
```

Expected: `go.mod` and `go.sum` created.

- [ ] **Step 2: Create stub source files**

Create `tgo/main.go`:
```go
package main

import (
	"context"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "tgo",
		Usage: "A simple task manager",
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		os.Exit(1)
	}
}
```

Create `tgo/tasks.go`:
```go
package main
```

Create `tgo/tasks_test.go`:
```go
package main

import "testing"

func TestPlaceholder(t *testing.T) {}
```

Create `tgo/main_test.go`:
```go
package main

import "testing"

func TestMainPlaceholder(t *testing.T) {}
```

- [ ] **Step 3: Verify it builds**

```bash
go build ./...
```

Expected: No errors, binary produced.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum main.go tasks.go tasks_test.go main_test.go
git commit -m "feat: project scaffolding"
```

---

## Task 2: Error Types and Hash Helper

**Files:**
- Modify: `tgo/tasks.go`
- Modify: `tgo/tasks_test.go`

- [ ] **Step 1: Write the failing tests**

Replace `tgo/tasks_test.go` with:
```go
package main

import (
	"errors"
	"testing"
)

func TestErrAmbiguousPrefix(t *testing.T) {
	err := &ErrAmbiguousPrefix{Prefix: "abc"}
	var target *ErrAmbiguousPrefix
	if !errors.As(err, &target) {
		t.Fatal("errors.As failed for ErrAmbiguousPrefix")
	}
	if target.Prefix != "abc" {
		t.Errorf("want prefix 'abc', got %q", target.Prefix)
	}
}

func TestErrUnknownPrefix(t *testing.T) {
	err := &ErrUnknownPrefix{Prefix: "xyz"}
	var target *ErrUnknownPrefix
	if !errors.As(err, &target) {
		t.Fatal("errors.As failed for ErrUnknownPrefix")
	}
}

func TestErrInvalidTaskFile(t *testing.T) {
	err := &ErrInvalidTaskFile{Path: "/some/path"}
	var target *ErrInvalidTaskFile
	if !errors.As(err, &target) {
		t.Fatal("errors.As failed for ErrInvalidTaskFile")
	}
}

func TestErrBadFile(t *testing.T) {
	err := &ErrBadFile{Path: "/some/path", Problem: "permission denied"}
	var target *ErrBadFile
	if !errors.As(err, &target) {
		t.Fatal("errors.As failed for ErrBadFile")
	}
}

func TestHashText(t *testing.T) {
	// SHA1 of "hello" is aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d
	got := hashText("hello")
	want := "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"
	if got != want {
		t.Errorf("hashText(\"hello\") = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./... -run "TestErr|TestHashText" -v
```

Expected: FAIL — types not defined.

- [ ] **Step 3: Implement error types and hashText in tasks.go**

Replace `tgo/tasks.go` with:
```go
package main

import (
	"crypto/sha1"
	"fmt"
)

// ErrAmbiguousPrefix is returned when a prefix matches more than one task.
type ErrAmbiguousPrefix struct{ Prefix string }

func (e *ErrAmbiguousPrefix) Error() string {
	return fmt.Sprintf("the ID %q matches more than one task", e.Prefix)
}

// ErrUnknownPrefix is returned when a prefix matches no tasks.
type ErrUnknownPrefix struct{ Prefix string }

func (e *ErrUnknownPrefix) Error() string {
	return fmt.Sprintf("the ID %q does not match any task", e.Prefix)
}

// ErrInvalidTaskFile is returned when a task file path is a directory.
type ErrInvalidTaskFile struct{ Path string }

func (e *ErrInvalidTaskFile) Error() string {
	return fmt.Sprintf("task file path is a directory: %s", e.Path)
}

// ErrBadFile is returned on I/O errors with a task file.
type ErrBadFile struct{ Path, Problem string }

func (e *ErrBadFile) Error() string {
	return fmt.Sprintf("%s: %s", e.Problem, e.Path)
}

// hashText returns the SHA1 hex digest of the given UTF-8 text.
func hashText(text string) string {
	h := sha1.New()
	h.Write([]byte(text))
	return fmt.Sprintf("%x", h.Sum(nil))
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./... -run "TestErr|TestHashText" -v
```

Expected: PASS for all 5 tests.

- [ ] **Step 5: Commit**

```bash
git add tasks.go tasks_test.go
git commit -m "feat: error types and hashText helper"
```

---

## Task 3: Task Struct, File Parsing, and Serialization

**Files:**
- Modify: `tgo/tasks.go`
- Modify: `tgo/tasks_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `tgo/tasks_test.go`:
```go
func TestTaskFromLine_StandardFormat(t *testing.T) {
	task, ok, err := taskFromLine("Buy more beer | id:abc123")
	if err != nil || !ok {
		t.Fatalf("unexpected: ok=%v err=%v", ok, err)
	}
	if task.ID != "abc123" || task.Text != "Buy more beer" {
		t.Errorf("got %+v", task)
	}
}

func TestTaskFromLine_BareText(t *testing.T) {
	task, ok, err := taskFromLine("Buy more beer")
	if err != nil || !ok {
		t.Fatalf("unexpected: ok=%v err=%v", ok, err)
	}
	want := hashText("Buy more beer")
	if task.ID != want || task.Text != "Buy more beer" {
		t.Errorf("got %+v, want ID=%s", task, want)
	}
}

func TestTaskFromLine_Comment(t *testing.T) {
	_, ok, err := taskFromLine("# this is a comment")
	if err != nil || ok {
		t.Errorf("expected comment to be skipped, got ok=%v err=%v", ok, err)
	}
}

func TestTaskFromLine_Empty(t *testing.T) {
	_, ok, err := taskFromLine("")
	if err != nil || ok {
		t.Errorf("expected empty to be skipped, got ok=%v err=%v", ok, err)
	}
}

func TestTaskFromLine_Whitespace(t *testing.T) {
	_, ok, err := taskFromLine("   ")
	if err != nil || ok {
		t.Errorf("expected whitespace-only to be skipped")
	}
}

func TestTaskToLine(t *testing.T) {
	task := Task{ID: "abc123", Text: "Buy more beer"}
	got := taskToLine(task)
	want := "Buy more beer | id:abc123\n"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestTaskRoundTrip(t *testing.T) {
	original := Task{ID: hashText("Clean the apartment"), Text: "Clean the apartment"}
	line := taskToLine(original)
	got, ok, err := taskFromLine(line[:len(line)-1]) // strip trailing newline
	if err != nil || !ok {
		t.Fatalf("round trip failed: ok=%v err=%v", ok, err)
	}
	if got != original {
		t.Errorf("want %v, got %v", original, got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./... -run "TestTask" -v
```

Expected: FAIL — Task type and functions not defined.

- [ ] **Step 3: Implement Task struct and parsing functions in tasks.go**

Append to `tgo/tasks.go`:
```go
// Task represents a single task entry.
type Task struct {
	ID   string
	Text string
}

// taskFromLine parses one line from a task file.
// Returns (task, true, nil) on success, (Task{}, false, nil) for blank/comment lines.
func taskFromLine(line string) (Task, bool, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return Task{}, false, nil
	}
	if idx := strings.LastIndex(line, "|"); idx >= 0 {
		text := strings.TrimSpace(line[:idx])
		meta := strings.TrimSpace(line[idx+1:])
		task := Task{Text: text}
		for _, piece := range strings.Split(meta, ",") {
			piece = strings.TrimSpace(piece)
			parts := strings.SplitN(piece, ":", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == "id" {
				task.ID = strings.TrimSpace(parts[1])
			}
		}
		if task.ID == "" {
			task.ID = hashText(task.Text)
		}
		return task, true, nil
	}
	return Task{ID: hashText(line), Text: line}, true, nil
}

// taskToLine serializes a task to a file line.
func taskToLine(t Task) string {
	return fmt.Sprintf("%s | id:%s\n", t.Text, t.ID)
}
```

Add `"strings"` to the imports in `tasks.go`. The full imports block should be:
```go
import (
	"crypto/sha1"
	"fmt"
	"strings"
)
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./... -run "TestTask" -v
```

Expected: PASS for all 7 tests.

- [ ] **Step 5: Commit**

```bash
git add tasks.go tasks_test.go
git commit -m "feat: Task struct, parsing, and serialization"
```

---

## Task 4: Prefix Algorithm

**Files:**
- Modify: `tgo/tasks.go`
- Modify: `tgo/tasks_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `tgo/tasks_test.go`:
```go
func TestPrefixes_Single(t *testing.T) {
	id := "abcdef1234567890abcdef1234567890abcdef12"
	got := prefixes([]string{id})
	if got[id] != "a" {
		t.Errorf("single id: want prefix 'a', got %q", got[id])
	}
}

func TestPrefixes_DistinctFirstChar(t *testing.T) {
	ids := []string{
		"abcdef1234567890abcdef1234567890abcdef12",
		"bbcdef1234567890abcdef1234567890abcdef12",
	}
	got := prefixes(ids)
	if got[ids[0]] != "a" {
		t.Errorf("first id: want 'a', got %q", got[ids[0]])
	}
	if got[ids[1]] != "b" {
		t.Errorf("second id: want 'b', got %q", got[ids[1]])
	}
}

func TestPrefixes_CommonPrefix(t *testing.T) {
	ids := []string{
		"abcdef1234567890abcdef1234567890abcdef12",
		"abcxyz1234567890abcdef1234567890abcdef12",
	}
	got := prefixes(ids)
	if got[ids[0]] != "abcd" {
		t.Errorf("first id: want 'abcd', got %q", got[ids[0]])
	}
	if got[ids[1]] != "abcx" {
		t.Errorf("second id: want 'abcx', got %q", got[ids[1]])
	}
}

func TestPrefixes_Empty(t *testing.T) {
	got := prefixes([]string{})
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestPrefixes_ThreeIDs(t *testing.T) {
	ids := []string{
		"aaa0001234567890abcdef1234567890abcdef12",
		"aab0001234567890abcdef1234567890abcdef12",
		"bbb0001234567890abcdef1234567890abcdef12",
	}
	got := prefixes(ids)
	if got[ids[0]] != "aaa" {
		t.Errorf("first id: want 'aaa', got %q", got[ids[0]])
	}
	if got[ids[1]] != "aab" {
		t.Errorf("second id: want 'aab', got %q", got[ids[1]])
	}
	if got[ids[2]] != "b" {
		t.Errorf("third id: want 'b', got %q", got[ids[2]])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./... -run "TestPrefixes" -v
```

Expected: FAIL — `prefixes` not defined.

- [ ] **Step 3: Implement the prefix algorithm in tasks.go**

Append to `tgo/tasks.go`:
```go
// prefixes returns a mapping of id -> shortest unique prefix for each id in O(n).
// Ports the _prefixes function from sjl/t verbatim.
func prefixes(ids []string) map[string]string {
	ps := make(map[string]string) // prefix -> id ("" means collision marker)

	for _, id := range ids {
		idLen := len(id)
		var prefix string
		var i int
		for i = 1; i <= idLen; i++ {
			prefix = id[:i]
			existing, found := ps[prefix]
			if !found || (existing != "" && prefix != existing) {
				break
			}
		}

		if otherID, found := ps[prefix]; found {
			// collision: walk forward until they diverge
			broke := false
			for j := i; j <= idLen; j++ {
				if otherID[:j] == id[:j] {
					ps[id[:j]] = ""
				} else {
					ps[otherID[:j]] = otherID
					ps[id[:j]] = id
					broke = true
					break
				}
			}
			if !broke {
				// ids share a prefix of length idLen; store both at idLen+1
				ps[otherID[:idLen+1]] = otherID
				ps[id] = id
			}
		} else {
			ps[prefix] = id
		}
	}

	// Flip: id -> shortest prefix
	result := make(map[string]string)
	for prefix, id := range ps {
		if id != "" {
			result[id] = prefix
		}
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./... -run "TestPrefixes" -v
```

Expected: PASS for all 5 tests.

- [ ] **Step 5: Commit**

```bash
git add tasks.go tasks_test.go
git commit -m "feat: prefix algorithm"
```

---

## Task 5: TaskList — NewTaskList and Write

**Files:**
- Modify: `tgo/tasks.go`
- Modify: `tgo/tasks_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `tgo/tasks_test.go`:
```go
import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewTaskList_Empty(t *testing.T) {
	dir := t.TempDir()
	tl, err := NewTaskList(dir, "tasks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tl.Tasks) != 0 || len(tl.Done) != 0 {
		t.Errorf("expected empty task list")
	}
}

func TestNewTaskList_InvalidTaskFile(t *testing.T) {
	dir := t.TempDir()
	// Create a directory where the task file should be
	if err := os.Mkdir(filepath.Join(dir, "tasks"), 0755); err != nil {
		t.Fatal(err)
	}
	_, err := NewTaskList(dir, "tasks")
	var target *ErrInvalidTaskFile
	if !errors.As(err, &target) {
		t.Errorf("expected ErrInvalidTaskFile, got %v", err)
	}
}

func TestWriteAndRead_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	tl := &TaskList{
		Tasks:   map[string]Task{"abc123": {ID: "abc123", Text: "Buy more beer"}},
		Done:    map[string]Task{},
		Name:    "tasks",
		TaskDir: dir,
	}
	if err := tl.Write(false); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	tl2, err := NewTaskList(dir, "tasks")
	if err != nil {
		t.Fatalf("NewTaskList failed: %v", err)
	}
	task, ok := tl2.Tasks["abc123"]
	if !ok || task.Text != "Buy more beer" {
		t.Errorf("expected task after round trip, got %+v", tl2.Tasks)
	}
}

func TestWrite_DeleteIfEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks")

	// Create the file first
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	tl := &TaskList{
		Tasks:   map[string]Task{},
		Done:    map[string]Task{},
		Name:    "tasks",
		TaskDir: dir,
	}
	if err := tl.Write(true); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file to be deleted, but it still exists")
	}
}

func TestWrite_SortedByID(t *testing.T) {
	dir := t.TempDir()
	tl := &TaskList{
		Tasks: map[string]Task{
			"bbb": {ID: "bbb", Text: "Second"},
			"aaa": {ID: "aaa", Text: "First"},
		},
		Done:    map[string]Task{},
		Name:    "tasks",
		TaskDir: dir,
	}
	if err := tl.Write(false); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "tasks"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	firstIdx := strings.Index(content, "First")
	secondIdx := strings.Index(content, "Second")
	if firstIdx >= secondIdx {
		t.Errorf("expected 'First' (id:aaa) before 'Second' (id:bbb) in sorted output")
	}
}
```

Note: `tasks_test.go` already imports `"testing"`. Add `"errors"`, `"os"`, `"path/filepath"` to the import block in `tasks_test.go`. The file should have one combined import block at the top:
```go
import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./... -run "TestNewTaskList|TestWrite" -v
```

Expected: FAIL — `TaskList` and `NewTaskList` not defined.

- [ ] **Step 3: Implement TaskList, expandPath, NewTaskList, and Write in tasks.go**

Append to `tgo/tasks.go` (also add `"os"`, `"path/filepath"`, `"sort"` to imports):

Full updated imports for `tasks.go`:
```go
import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)
```

Append the following to `tasks.go`:
```go
// TaskList holds open and finished tasks for a named list.
type TaskList struct {
	Tasks   map[string]Task // id -> open task
	Done    map[string]Task // id -> finished task
	Name    string
	TaskDir string
}

// expandPath expands a leading ~ to the user's home directory.
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

// NewTaskList reads both task files from disk and returns a populated TaskList.
func NewTaskList(taskDir, name string) (*TaskList, error) {
	tl := &TaskList{
		Tasks:   make(map[string]Task),
		Done:    make(map[string]Task),
		Name:    name,
		TaskDir: taskDir,
	}

	type fileSpec struct {
		dest     map[string]Task
		filename string
	}
	files := []fileSpec{
		{tl.Tasks, name},
		{tl.Done, "." + name + ".done"},
	}

	for _, f := range files {
		path := filepath.Join(expandPath(taskDir), f.filename)
		fi, err := os.Stat(path)
		if err == nil && fi.IsDir() {
			return nil, &ErrInvalidTaskFile{Path: path}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, &ErrBadFile{Path: path, Problem: err.Error()}
		}
		for _, line := range strings.Split(string(data), "\n") {
			task, ok, err := taskFromLine(line)
			if err != nil {
				return nil, &ErrBadFile{Path: path, Problem: err.Error()}
			}
			if ok {
				f.dest[task.ID] = task
			}
		}
	}
	return tl, nil
}

// Write flushes both task files to disk.
// If deleteIfEmpty is true, removes the file instead of writing an empty one.
func (tl *TaskList) Write(deleteIfEmpty bool) error {
	type fileSpec struct {
		source   map[string]Task
		filename string
	}
	files := []fileSpec{
		{tl.Tasks, tl.Name},
		{tl.Done, "." + tl.Name + ".done"},
	}

	for _, f := range files {
		path := filepath.Join(expandPath(tl.TaskDir), f.filename)

		fi, err := os.Stat(path)
		if err == nil && fi.IsDir() {
			return &ErrInvalidTaskFile{Path: path}
		}

		tasks := make([]Task, 0, len(f.source))
		for _, t := range f.source {
			tasks = append(tasks, t)
		}
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].ID < tasks[j].ID
		})

		if len(tasks) == 0 && deleteIfEmpty {
			if _, err := os.Stat(path); err == nil {
				if err := os.Remove(path); err != nil {
					return &ErrBadFile{Path: path, Problem: err.Error()}
				}
			}
			continue
		}

		var sb strings.Builder
		for _, t := range tasks {
			sb.WriteString(taskToLine(t))
		}
		if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
			return &ErrBadFile{Path: path, Problem: err.Error()}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./... -run "TestNewTaskList|TestWrite" -v
```

Expected: PASS for all 5 tests.

- [ ] **Step 5: Commit**

```bash
git add tasks.go tasks_test.go
git commit -m "feat: TaskList, NewTaskList, and Write"
```

---

## Task 6: TaskList — Add, getTask, Finish, Remove, Edit

**Files:**
- Modify: `tgo/tasks.go`
- Modify: `tgo/tasks_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `tgo/tasks_test.go`:
```go
func newTestTaskList() *TaskList {
	return &TaskList{
		Tasks:   make(map[string]Task),
		Done:    make(map[string]Task),
		Name:    "tasks",
		TaskDir: "",
	}
}

func TestAdd(t *testing.T) {
	tl := newTestTaskList()
	prefix, err := tl.Add("Buy more beer")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if prefix == "" {
		t.Error("expected non-empty prefix")
	}
	id := hashText("Buy more beer")
	task, ok := tl.Tasks[id]
	if !ok || task.Text != "Buy more beer" {
		t.Errorf("task not in map after Add")
	}
}

func TestAdd_PrefixIsShortestUnique(t *testing.T) {
	tl := newTestTaskList()
	p1, _ := tl.Add("aaaa task one")
	p2, _ := tl.Add("bbbb task two")
	if len(p1) == 0 || len(p2) == 0 {
		t.Error("expected non-empty prefixes")
	}
	// prefixes must be different
	if p1 == p2 {
		t.Errorf("expected distinct prefixes, both got %q", p1)
	}
}

func TestGetTask_AmbiguousPrefix(t *testing.T) {
	tl := newTestTaskList()
	id1 := hashText("task one")
	id2 := hashText("task two")
	tl.Tasks[id1] = Task{ID: id1, Text: "task one"}
	tl.Tasks[id2] = Task{ID: id2, Text: "task two"}

	// Use empty prefix which matches all
	_, err := tl.getTask("")
	var ae *ErrAmbiguousPrefix
	if !errors.As(err, &ae) {
		t.Errorf("expected ErrAmbiguousPrefix, got %v", err)
	}
}

func TestGetTask_UnknownPrefix(t *testing.T) {
	tl := newTestTaskList()
	_, err := tl.getTask("zzz")
	var ue *ErrUnknownPrefix
	if !errors.As(err, &ue) {
		t.Errorf("expected ErrUnknownPrefix, got %v", err)
	}
}

func TestFinish(t *testing.T) {
	tl := newTestTaskList()
	id := hashText("Buy more beer")
	tl.Tasks[id] = Task{ID: id, Text: "Buy more beer"}

	ps := prefixes([]string{id})
	if err := tl.Finish(ps[id]); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
	if _, ok := tl.Tasks[id]; ok {
		t.Error("task should be removed from Tasks after Finish")
	}
	if _, ok := tl.Done[id]; !ok {
		t.Error("task should be in Done after Finish")
	}
}

func TestRemove(t *testing.T) {
	tl := newTestTaskList()
	id := hashText("Buy more beer")
	tl.Tasks[id] = Task{ID: id, Text: "Buy more beer"}

	ps := prefixes([]string{id})
	if err := tl.Remove(ps[id]); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if _, ok := tl.Tasks[id]; ok {
		t.Error("task should be gone after Remove")
	}
}

func TestEdit(t *testing.T) {
	tl := newTestTaskList()
	oldID := hashText("Buy more beer")
	tl.Tasks[oldID] = Task{ID: oldID, Text: "Buy more beer"}

	ps := prefixes([]string{oldID})
	if err := tl.Edit(ps[oldID], "Buy a lot more beer"); err != nil {
		t.Fatalf("Edit failed: %v", err)
	}
	if _, ok := tl.Tasks[oldID]; ok {
		t.Error("old task should be gone after Edit")
	}
	newID := hashText("Buy a lot more beer")
	if task, ok := tl.Tasks[newID]; !ok || task.Text != "Buy a lot more beer" {
		t.Errorf("new task not found after Edit: %+v", tl.Tasks)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./... -run "TestAdd|TestGetTask|TestFinish|TestRemove|TestEdit" -v
```

Expected: FAIL — methods not defined.

- [ ] **Step 3: Implement getTask, Add, Finish, Remove, Edit in tasks.go**

Append to `tgo/tasks.go`:
```go
// getTask resolves a prefix to a single open task.
// Returns ErrAmbiguousPrefix or ErrUnknownPrefix as appropriate.
func (tl *TaskList) getTask(prefix string) (Task, error) {
	var matched []string
	for id := range tl.Tasks {
		if strings.HasPrefix(id, prefix) {
			matched = append(matched, id)
		}
	}
	switch len(matched) {
	case 1:
		return tl.Tasks[matched[0]], nil
	case 0:
		return Task{}, &ErrUnknownPrefix{Prefix: prefix}
	default:
		for _, id := range matched {
			if id == prefix {
				return tl.Tasks[id], nil
			}
		}
		return Task{}, &ErrAmbiguousPrefix{Prefix: prefix}
	}
}

// Add creates a new open task and returns its shortest prefix.
func (tl *TaskList) Add(text string) (string, error) {
	id := hashText(text)
	tl.Tasks[id] = Task{ID: id, Text: text}
	ids := make([]string, 0, len(tl.Tasks))
	for id := range tl.Tasks {
		ids = append(ids, id)
	}
	ps := prefixes(ids)
	return ps[id], nil
}

// Finish moves the task matching prefix from Tasks to Done.
func (tl *TaskList) Finish(prefix string) error {
	task, err := tl.getTask(prefix)
	if err != nil {
		return err
	}
	delete(tl.Tasks, task.ID)
	tl.Done[task.ID] = task
	return nil
}

// Remove deletes the task matching prefix from Tasks.
func (tl *TaskList) Remove(prefix string) error {
	task, err := tl.getTask(prefix)
	if err != nil {
		return err
	}
	delete(tl.Tasks, task.ID)
	return nil
}

// Edit replaces the text of the task matching prefix.
// The task is re-inserted with a new ID derived from the new text.
func (tl *TaskList) Edit(prefix, newText string) error {
	task, err := tl.getTask(prefix)
	if err != nil {
		return err
	}
	delete(tl.Tasks, task.ID)
	newID := hashText(newText)
	tl.Tasks[newID] = Task{ID: newID, Text: newText}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./... -run "TestAdd|TestGetTask|TestFinish|TestRemove|TestEdit" -v
```

Expected: PASS for all 8 tests.

- [ ] **Step 5: Commit**

```bash
git add tasks.go tasks_test.go
git commit -m "feat: TaskList CRUD methods"
```

---

## Task 7: TaskList — List

**Files:**
- Modify: `tgo/tasks.go`
- Modify: `tgo/tasks_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `tgo/tasks_test.go`:
```go
func TestList_PrintsTasksToStdout(t *testing.T) {
	tl := newTestTaskList()
	tl.Tasks["aaa"] = Task{ID: "aaa", Text: "First task"}
	tl.Tasks["bbb"] = Task{ID: "bbb", Text: "Second task"}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	tl.List("tasks", false, false, "")

	w.Close()
	os.Stdout = old

	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "First task") {
		t.Errorf("expected 'First task' in output, got: %s", output)
	}
	if !strings.Contains(output, "Second task") {
		t.Errorf("expected 'Second task' in output, got: %s", output)
	}
}

func TestList_Grep(t *testing.T) {
	tl := newTestTaskList()
	tl.Tasks["aaa"] = Task{ID: "aaa", Text: "Buy groceries"}
	tl.Tasks["bbb"] = Task{ID: "bbb", Text: "Walk the dog"}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	tl.List("tasks", false, false, "groceries")

	w.Close()
	os.Stdout = old

	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Buy groceries") {
		t.Errorf("expected 'Buy groceries' in output")
	}
	if strings.Contains(output, "Walk the dog") {
		t.Errorf("'Walk the dog' should be filtered out by grep")
	}
}

func TestList_Quiet(t *testing.T) {
	tl := newTestTaskList()
	tl.Tasks["aaa"] = Task{ID: "aaa", Text: "A task"}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	tl.List("tasks", false, true, "")

	w.Close()
	os.Stdout = old

	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	if strings.Contains(output, " - ") {
		t.Errorf("quiet mode should not include ' - ' separator")
	}
	if !strings.Contains(output, "A task") {
		t.Errorf("expected task text in quiet output")
	}
}

func TestList_Done(t *testing.T) {
	tl := newTestTaskList()
	tl.Tasks["aaa"] = Task{ID: "aaa", Text: "Open task"}
	tl.Done["bbb"] = Task{ID: "bbb", Text: "Done task"}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	tl.List("done", false, false, "")

	w.Close()
	os.Stdout = old

	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	if strings.Contains(output, "Open task") {
		t.Errorf("open task should not appear in done list")
	}
	if !strings.Contains(output, "Done task") {
		t.Errorf("expected done task in done list output")
	}
}
```

Add `"io"` to the imports in `tasks_test.go`. Full import block:
```go
import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./... -run "TestList" -v
```

Expected: FAIL — `List` method not defined.

- [ ] **Step 3: Implement List in tasks.go**

Append to `tgo/tasks.go`. Also add `"fmt"` is already in imports; also need to verify `"sort"` and `"strings"` are there. No new imports needed.

```go
// List prints the task list to stdout.
// kind is "tasks" for open tasks or "done" for finished tasks.
// verbose shows full IDs; quiet suppresses the ID prefix; grep filters by substring.
func (tl *TaskList) List(kind string, verbose, quiet bool, grep string) {
	var tasks map[string]Task
	if kind == "done" {
		tasks = tl.Done
	} else {
		tasks = tl.Tasks
	}

	if len(tasks) == 0 {
		return
	}

	ids := make([]string, 0, len(tasks))
	for id := range tasks {
		ids = append(ids, id)
	}

	var labelFn func(string) string
	var maxLen int

	if verbose {
		labelFn = func(id string) string { return id }
		for _, id := range ids {
			if len(id) > maxLen {
				maxLen = len(id)
			}
		}
	} else {
		ps := prefixes(ids)
		labelFn = func(id string) string { return ps[id] }
		for _, id := range ids {
			if l := len(ps[id]); l > maxLen {
				maxLen = l
			}
		}
	}

	sort.Strings(ids)
	for _, id := range ids {
		task := tasks[id]
		if grep != "" && !strings.Contains(strings.ToLower(task.Text), strings.ToLower(grep)) {
			continue
		}
		if quiet {
			fmt.Println(task.Text)
		} else {
			fmt.Printf("%-*s - %s\n", maxLen, labelFn(id), task.Text)
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./... -run "TestList" -v
```

Expected: PASS for all 4 tests.

- [ ] **Step 5: Run all tests to confirm nothing is broken**

```bash
go test ./... -v
```

Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add tasks.go tasks_test.go
git commit -m "feat: TaskList.List method"
```

---

## Task 8: CLI Wiring (main.go) and Integration Tests

**Files:**
- Modify: `tgo/main.go`
- Modify: `tgo/main_test.go`

- [ ] **Step 1: Write the failing integration tests**

Replace `tgo/main_test.go` with:
```go
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	tl, _ := NewTaskList(dir, "tasks")
	var taskID string
	for id := range tl.Tasks {
		taskID = id
	}
	ps := prefixes([]string{taskID})

	if err := runApp(t, dir, "finish", ps[taskID]); err != nil {
		t.Fatalf("finish failed: %v", err)
	}
	tl2, _ := NewTaskList(dir, "tasks")
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
	tl, _ := NewTaskList(dir, "tasks")
	var taskID string
	for id := range tl.Tasks {
		taskID = id
	}
	ps := prefixes([]string{taskID})

	if err := runApp(t, dir, "remove", ps[taskID]); err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	tl2, _ := NewTaskList(dir, "tasks")
	if len(tl2.Tasks) != 0 {
		t.Errorf("expected 0 tasks after remove, got %d", len(tl2.Tasks))
	}
}

func TestCLI_Edit(t *testing.T) {
	dir := t.TempDir()
	if err := runApp(t, dir, "add", "Buy more beer"); err != nil {
		t.Fatal(err)
	}
	tl, _ := NewTaskList(dir, "tasks")
	var taskID string
	for id := range tl.Tasks {
		taskID = id
	}
	ps := prefixes([]string{taskID})

	if err := runApp(t, dir, "edit", ps[taskID], "Buy a lot more beer"); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	tl2, _ := NewTaskList(dir, "tasks")
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
	tl, _ := NewTaskList(dir, "tasks")
	var taskID string
	for id := range tl.Tasks {
		taskID = id
	}
	ps := prefixes([]string{taskID})
	runApp(t, dir, "finish", ps[taskID])

	// done subcommand should not error
	if err := runApp(t, dir, "done"); err != nil {
		t.Fatalf("done failed: %v", err)
	}
}

func TestCLI_CustomList(t *testing.T) {
	dir := t.TempDir()
	if err := buildApp().Run(context.Background(), []string{"tgo", "--task-dir", dir, "--list", "groceries", "add", "Milk"}); err != nil {
		t.Fatalf("add to custom list failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "groceries")); os.IsNotExist(err) {
		t.Error("expected groceries file to exist")
	}
	_ = strings.Contains // suppress unused import
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./... -run "TestCLI" -v
```

Expected: FAIL — `buildApp` not defined.

- [ ] **Step 3: Implement full main.go**

Replace `tgo/main.go` with:
```go
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	if err := buildApp().Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// buildApp constructs and returns the CLI app.
// Extracted so tests can call it directly.
func buildApp() *cli.Command {
	return &cli.Command{
		Name:           "tgo",
		Usage:          "A simple task manager",
		DefaultCommand: "list",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "task-dir",
				Aliases: []string{"t"},
				Usage:   "work on the lists in `DIR`",
				Value:   "",
			},
			&cli.StringFlag{
				Name:    "list",
				Aliases: []string{"l"},
				Usage:   "work on `LIST`",
				Value:   "tasks",
			},
			&cli.BoolFlag{
				Name:    "delete-if-empty",
				Aliases: []string{"d"},
				Usage:   "delete the task file if it becomes empty",
			},
		},
		Commands: []*cli.Command{
			{
				Name:      "add",
				Usage:     "Add a new task",
				ArgsUsage: "TEXT",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					text := cmd.Args().First()
					if text == "" {
						return fmt.Errorf("task text is required")
					}
					tl, err := loadTaskList(cmd)
					if err != nil {
						return handleTaskError(err)
					}
					prefix, err := tl.Add(text)
					if err != nil {
						return handleTaskError(err)
					}
					if err := tl.Write(cmd.Root().Bool("delete-if-empty")); err != nil {
						return handleTaskError(err)
					}
					fmt.Println(prefix)
					return nil
				},
			},
			{
				Name:  "list",
				Usage: "List open tasks",
				Flags: listFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					tl, err := loadTaskList(cmd)
					if err != nil {
						return handleTaskError(err)
					}
					tl.List("tasks", cmd.Bool("verbose"), cmd.Bool("quiet"), cmd.String("grep"))
					return nil
				},
			},
			{
				Name:  "done",
				Usage: "List finished tasks",
				Flags: listFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					tl, err := loadTaskList(cmd)
					if err != nil {
						return handleTaskError(err)
					}
					tl.List("done", cmd.Bool("verbose"), cmd.Bool("quiet"), cmd.String("grep"))
					return nil
				},
			},
			{
				Name:      "finish",
				Usage:     "Mark a task as finished",
				ArgsUsage: "TASK",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					prefix := cmd.Args().First()
					if prefix == "" {
						return fmt.Errorf("task prefix is required")
					}
					tl, err := loadTaskList(cmd)
					if err != nil {
						return handleTaskError(err)
					}
					if err := tl.Finish(prefix); err != nil {
						return handleTaskError(err)
					}
					return tl.Write(cmd.Root().Bool("delete-if-empty"))
				},
			},
			{
				Name:      "remove",
				Usage:     "Remove a task from the list",
				ArgsUsage: "TASK",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					prefix := cmd.Args().First()
					if prefix == "" {
						return fmt.Errorf("task prefix is required")
					}
					tl, err := loadTaskList(cmd)
					if err != nil {
						return handleTaskError(err)
					}
					if err := tl.Remove(prefix); err != nil {
						return handleTaskError(err)
					}
					return tl.Write(cmd.Root().Bool("delete-if-empty"))
				},
			},
			{
				Name:      "edit",
				Usage:     "Edit a task's text",
				ArgsUsage: "TASK NEW_TEXT",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					args := cmd.Args()
					if args.Len() < 2 {
						return fmt.Errorf("usage: edit TASK NEW_TEXT")
					}
					prefix := args.Get(0)
					newText := args.Get(1)
					tl, err := loadTaskList(cmd)
					if err != nil {
						return handleTaskError(err)
					}
					if err := tl.Edit(prefix, newText); err != nil {
						return handleTaskError(err)
					}
					return tl.Write(cmd.Root().Bool("delete-if-empty"))
				},
			},
		},
	}
}

// listFlags returns the shared flags for the list and done subcommands.
func listFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "grep",
			Aliases: []string{"g"},
			Usage:   "print only tasks containing `WORD`",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "print full task IDs",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "suppress task IDs in output",
		},
	}
}

// loadTaskList creates a TaskList using the global flags from the root command.
func loadTaskList(cmd *cli.Command) (*TaskList, error) {
	root := cmd.Root()
	return NewTaskList(root.String("task-dir"), root.String("list"))
}

// handleTaskError formats task-specific errors for CLI output.
func handleTaskError(err error) error {
	var ae *ErrAmbiguousPrefix
	var ue *ErrUnknownPrefix
	var ie *ErrInvalidTaskFile
	var be *ErrBadFile
	switch {
	case errors.As(err, &ae):
		return fmt.Errorf("the ID %q matches more than one task", ae.Prefix)
	case errors.As(err, &ue):
		return fmt.Errorf("the ID %q does not match any task", ue.Prefix)
	case errors.As(err, &ie):
		return fmt.Errorf("task file path is a directory: %s", ie.Path)
	case errors.As(err, &be):
		return fmt.Errorf("%s: %s", be.Problem, be.Path)
	default:
		return err
	}
}
```

- [ ] **Step 4: Run integration tests to verify they pass**

```bash
go test ./... -run "TestCLI" -v
```

Expected: PASS for all 8 integration tests.

- [ ] **Step 5: Run all tests**

```bash
go test ./... -v
```

Expected: All tests PASS.

- [ ] **Step 6: Build and smoke-test the binary**

```bash
go build -o tgo .
./tgo add "Buy more beer"
./tgo
./tgo finish $(./tgo -q list | head -1 | awk '{print $1}')
```

Actually, just do a simple build and help check:
```bash
go build -o tgo_bin .
./tgo_bin --help
./tgo_bin add "Test task"
./tgo_bin list
./tgo_bin done
rm tgo_bin
```

Expected: Help text prints cleanly, add/list/done all work.

- [ ] **Step 7: Commit**

```bash
git add main.go main_test.go
git commit -m "feat: CLI wiring with urfave/cli v3 subcommands"
```

---

## Task 9: README Update

**Files:**
- Modify: `tgo/README.md`

- [ ] **Step 1: Update README**

Replace `tgo/README.md` with:
```markdown
# tgo

A minimalist command-line task manager, ported from [sjl/t](https://github.com/sjl/t) to Go.
File-format compatible with the original `t`.

## Install

```bash
go install github.com/nathanhruby/tgo@latest
```

## Usage

```bash
# Add a task
tgo add "Clean the apartment"
tgo add "Buy more beer"

# List open tasks (also: just run `tgo`)
tgo list

# Finish a task
tgo finish 9

# Edit a task
tgo edit 30 "Clean the entire apartment"

# Remove a task
tgo remove 9

# List finished tasks
tgo done

# Use a different list or directory
tgo --task-dir ~/tasks --list groceries add "Oat milk"
```

## Setup (alias)

Add to your shell config:

```bash
alias t='tgo --task-dir ~/tasks --list tasks'
```

## File Format

Compatible with `t`. Tasks are stored as plain text:

```
task text | id:<sha1hex>
```

- Open tasks: `<taskdir>/<listname>`
- Done tasks: `<taskdir>/.<listname>.done`
- Files are sorted by ID on write (VCS-friendly)
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: update README with usage"
```

---

## Self-Review

**Spec coverage check:**
- ✅ File format (read/write, comments, bare lines, sort by ID)
- ✅ Task struct (`ID`, `Text`)
- ✅ TaskList struct with `Tasks`/`Done` maps
- ✅ Prefix algorithm (O(n), faithful port)
- ✅ All 4 error types with `error` interface
- ✅ `NewTaskList`, `Write(deleteIfEmpty)`
- ✅ `Add` (returns prefix), `getTask`, `Finish`, `Remove`, `Edit`
- ✅ `List(kind, verbose, quiet, grep)`
- ✅ CLI: `add`, `list`, `done`, `finish`, `remove`, `edit` subcommands
- ✅ Global flags: `--task-dir`, `--list`, `--delete-if-empty`
- ✅ Per-list flags: `--grep`, `--verbose`, `--quiet`
- ✅ Default command: `list`
- ✅ `~` expansion in task dir path
- ✅ Unit tests (`tasks_test.go`) and integration tests (`main_test.go`)
- ✅ No sed-style substitution (correctly omitted)

**Placeholder scan:** No TBDs, no incomplete sections.

**Type consistency:** All method signatures consistent across tasks. `loadTaskList` correctly reads from `cmd.Root()`. `buildApp()` is exported from `main.go` for test use.

**Scope:** Single implementation plan, no decomposition needed.
