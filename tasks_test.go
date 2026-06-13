package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

	if target.Prefix != "xyz" {
		t.Errorf("want prefix 'xyz', got %q", target.Prefix)
	}
}

func TestErrInvalidTaskFile(t *testing.T) {
	err := &ErrInvalidTaskFile{Path: "/some/path"}

	var target *ErrInvalidTaskFile
	if !errors.As(err, &target) {
		t.Fatal("errors.As failed for ErrInvalidTaskFile")
	}

	if target.Path != "/some/path" {
		t.Errorf("want path '/some/path', got %q", target.Path)
	}
}

func TestErrBadFile(t *testing.T) {
	err := &ErrBadFile{Path: "/some/path", Problem: "permission denied"}

	var target *ErrBadFile
	if !errors.As(err, &target) {
		t.Fatal("errors.As failed for ErrBadFile")
	}

	if target.Path != "/some/path" || target.Problem != "permission denied" {
		t.Errorf("want path='/some/path' problem='permission denied', got path=%q problem=%q", target.Path, target.Problem)
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

func TestTaskFromLine_PipeInText(t *testing.T) {
	task, ok, err := taskFromLine("Buy A | B stuff | id:abc456")
	if err != nil || !ok {
		t.Fatalf("unexpected: ok=%v err=%v", ok, err)
	}

	if task.ID != "abc456" || task.Text != "Buy A | B stuff" {
		t.Errorf("got ID=%q Text=%q, want ID='abc456' Text='Buy A | B stuff'", task.ID, task.Text)
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

func TestPrefixes_ThreeWayCascade(t *testing.T) {
	// Third ID forces re-resolution of a prior 2-way collision
	ids := []string{
		"abc1231234567890abcdef1234567890abcdef12",
		"abc1241234567890abcdef1234567890abcdef12",
		"abc4561234567890abcdef1234567890abcdef12",
	}
	got := prefixes(ids)

	if got[ids[0]] != "abc123" {
		t.Errorf("first id: want 'abc123', got %q", got[ids[0]])
	}

	if got[ids[1]] != "abc124" {
		t.Errorf("second id: want 'abc124', got %q", got[ids[1]])
	}

	if got[ids[2]] != "abc4" {
		t.Errorf("third id: want 'abc4', got %q", got[ids[2]])
	}
}

func TestPrefixes_DuplicateIDs_NoPanic(t *testing.T) {
	// Duplicate IDs should not panic (though they shouldn't occur in production)
	id := "abcdef1234567890abcdef1234567890abcdef12"
	got := prefixes([]string{id, id})
	// Result is undefined for duplicates, but must not panic
	_ = got
}

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
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
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

func TestList_PrintsTasksToStdout(t *testing.T) {
	tl := newTestTaskList()
	tl.Tasks["aaa"] = Task{ID: "aaa", Text: "First task"}
	tl.Tasks["bbb"] = Task{ID: "bbb", Text: "Second task"}

	var buf strings.Builder

	tl.List(&buf, "tasks", false, false, "")
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

	var buf strings.Builder

	tl.List(&buf, "tasks", false, false, "groceries")
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

	var buf strings.Builder

	tl.List(&buf, "tasks", false, true, "")
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

	var buf strings.Builder

	tl.List(&buf, "done", false, false, "")
	output := buf.String()

	if strings.Contains(output, "Open task") {
		t.Errorf("open task should not appear in done list")
	}

	if !strings.Contains(output, "Done task") {
		t.Errorf("expected done task in done list output")
	}
}

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

func TestAdd_RejectsNewline(t *testing.T) {
	tl := newTestTaskList()

	_, err := tl.Add("line1\nline2")
	if err == nil {
		t.Error("expected error for task text containing newline")
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
