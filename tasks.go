package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// Task represents a single task entry.
type Task struct {
	ID   string
	Text string
}

// taskFromLine parses one line from a task file.
// Returns (task, true, nil) on success, (Task{}, false, nil) for blank/comment lines.
// The error return is reserved for future format validation; callers should handle it.
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

// prefixes returns a mapping of id -> shortest unique prefix for each id in O(n·k) time,
// where k is the maximum ID length (constant 40 for SHA1 hex strings).
// Ported from sjl/t's _prefixes function. All IDs should be uniform length (SHA1 hex).
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
				// Identical IDs — skip (SHA1 of distinct texts cannot collide in practice)
				if otherID != id {
					if idLen+1 <= len(otherID) {
						ps[otherID[:idLen+1]] = otherID
					}
					ps[id] = id
				}
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
	if strings.Contains(text, "\n") {
		return "", fmt.Errorf("task text cannot contain newlines")
	}
	id := hashText(text)
	tl.Tasks[id] = Task{ID: id, Text: text}
	ids := make([]string, 0, len(tl.Tasks))
	for taskID := range tl.Tasks {
		ids = append(ids, taskID)
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

// List prints the task list to stdout.
// kind is "tasks" for open tasks or "done" for finished tasks.
// verbose shows full IDs; quiet suppresses the ID prefix; grep filters by substring (case-insensitive).
func (tl *TaskList) List(w io.Writer, kind string, verbose, quiet bool, grep string) {
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
			fmt.Fprintln(w, task.Text)
		} else {
			fmt.Fprintf(w, "%-*s - %s\n", maxLen, labelFn(id), task.Text)
		}
	}
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
