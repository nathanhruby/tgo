package main

import (
	"crypto/sha1" //nolint:gosec // used for task-ID generation, not cryptography
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/rivo/uniseg"
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
	h := sha1.New() //nolint:gosec // SHA1 used for stable content-addressing, not security
	h.Write([]byte(text))

	return fmt.Sprintf("%x", h.Sum(nil))
}

type runeRange struct{ lo, hi rune }

const variationSelector16 rune = 0xFE0F

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
		if r == variationSelector16 {
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

// Task represents a single task entry.
type Task struct {
	ID   string
	Text string
}

const metaSplitParts = 2

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
			parts := strings.SplitN(piece, ":", metaSplitParts)

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

// resolveCollision updates ps to assign unique prefixes to id and the existing otherID.
// It iterates forward from startAt until the two IDs diverge or idLen is exhausted.
// Returns true if distinct prefixes were found; false if the IDs are identical.
func resolveCollision(ps map[string]string, id, otherID string, startAt, idLen int) bool {
	for j := startAt; j <= idLen; j++ {
		if otherID[:j] == id[:j] {
			ps[id[:j]] = ""
		} else {
			ps[otherID[:j]] = otherID
			ps[id[:j]] = id

			return true
		}
	}

	return false
}

// findPrefixForID scans ps to find the shortest prefix of id that is either absent from ps
// or already uniquely assigned to a full ID. Returns the prefix and the scan index at which
// scanning stopped (used as startAt in resolveCollision).
func findPrefixForID(ps map[string]string, id string) (string, int) {
	idLen := len(id)

	var prefix string

	for i := 1; i <= idLen; i++ {
		prefix = id[:i]
		existing, found := ps[prefix]

		if !found || (existing != "" && prefix != existing) {
			return prefix, i
		}
	}

	return prefix, idLen + 1
}

// prefixes returns a mapping of id -> shortest unique prefix for each id in O(n·k) time,
// where k is the maximum ID length (constant 40 for SHA1 hex strings).
// Ported from sjl/t's _prefixes function. All IDs should be uniform length (SHA1 hex).
func prefixes(ids []string) map[string]string {
	ps := make(map[string]string) // prefix -> id ("" means collision marker)

	for _, id := range ids {
		idLen := len(id)
		prefix, i := findPrefixForID(ps, id)

		otherID, found := ps[prefix]
		if !found {
			ps[prefix] = id

			continue
		}

		if resolveCollision(ps, id, otherID, i, idLen) {
			continue
		}

		// Identical IDs — skip (SHA1 of distinct texts cannot collide in practice)
		if otherID != id {
			if idLen+1 <= len(otherID) {
				ps[otherID[:idLen+1]] = otherID
			}

			ps[id] = id
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

// taskRow is one display row: an optional label (shortest unique id prefix, full id when
// verbose, empty when quiet) and the task text.
type taskRow struct {
	Label string
	Group string
	Text  string
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

// buildLabelFn returns a function that maps a task ID to its display label, plus the
// maximum label width across all ids (for column alignment).
func buildLabelFn(ids []string, verbose bool) (func(string) string, int) {
	var maxLen int

	if verbose {
		for _, id := range ids {
			if len(id) > maxLen {
				maxLen = len(id)
			}
		}

		return func(id string) string { return id }, maxLen
	}

	ps := prefixes(ids)

	for _, id := range ids {
		if l := len(ps[id]); l > maxLen {
			maxLen = l
		}
	}

	return func(id string) string { return ps[id] }, maxLen
}

// splitGroups splits each task's leading emoji group from its display text and sorts ids by
// group key. Tasks without a group sort last; ties are broken by id.
func splitGroups(ids []string, tasks map[string]Task) (map[string]string, map[string]string) {
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

	return keys, display
}

// matchesGrep reports whether text contains grep, ignoring case. An empty grep matches all text.
func matchesGrep(text, grep string) bool {
	return grep == "" || strings.Contains(strings.ToLower(text), strings.ToLower(grep))
}

// taskRows returns display rows for tasks in id order, applying quiet/verbose labelling and
// case-insensitive grep filtering. labelWidth is the widest label across every task, before
// grep filtering, so filtered output keeps the alignment of the unfiltered list.
// When group is true, rows are ordered by leading emoji group (ungrouped tasks last) and
// each row carries its group key and the text with the group removed.
func taskRows(tasks map[string]Task, verbose, quiet, group bool, grep string) ([]taskRow, int) {
	if len(tasks) == 0 {
		return nil, 0
	}

	ids := make([]string, 0, len(tasks))
	for id := range tasks {
		ids = append(ids, id)
	}

	labelFn, labelWidth := buildLabelFn(ids, verbose)

	var keys, display map[string]string

	if group {
		keys, display = splitGroups(ids, tasks)
	} else {
		sort.Strings(ids)
	}

	var rows []taskRow

	for _, id := range ids {
		task := tasks[id]
		if !matchesGrep(task.Text, grep) {
			continue
		}

		row := taskRow{Text: task.Text}
		if group {
			row.Group, row.Text = keys[id], display[id]
		}

		if !quiet {
			row.Label = labelFn(id)
		}

		rows = append(rows, row)
	}

	if quiet {
		return rows, 0
	}

	return rows, labelWidth
}

// List prints the task list to w.
// kind is "tasks" for open tasks or "done" for finished tasks.
// verbose shows full IDs; quiet suppresses the ID prefix; grep filters by substring (case-insensitive).
func (tl *TaskList) List(w io.Writer, kind string, verbose, quiet, group bool, grep string) {
	var tasks map[string]Task
	if kind == "done" {
		tasks = tl.Done
	} else {
		tasks = tl.Tasks
	}

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
}

// writeTaskRow writes one task row, indented under its group heading when indented is true.
// A zero width means labels are hidden and only the text is written.
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

// sortedTasks returns a slice of tasks from source, sorted by ID.
func sortedTasks(source map[string]Task) []Task {
	tasks := make([]Task, 0, len(source))
	for _, t := range source {
		tasks = append(tasks, t)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})

	return tasks
}

// deleteFileIfExists removes path if it exists; it is a no-op when path does not exist.
func deleteFileIfExists(path string) error {
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return &ErrBadFile{Path: path, Problem: err.Error()}
		}
	}

	return nil
}

// writeTasksToFile serializes tasks to path with 0600 permissions.
func writeTasksToFile(path string, tasks []Task) error {
	var sb strings.Builder

	for _, t := range tasks {
		sb.WriteString(taskToLine(t))
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0600); err != nil { //nolint:mnd
		return &ErrBadFile{Path: path, Problem: err.Error()}
	}

	return nil
}

// writeTaskFile writes or deletes a single task file at path.
func writeTaskFile(path string, source map[string]Task, deleteIfEmpty bool) error {
	fi, err := os.Stat(path)
	if err == nil && fi.IsDir() {
		return &ErrInvalidTaskFile{Path: path}
	}

	tasks := sortedTasks(source)
	if len(tasks) == 0 && deleteIfEmpty {
		return deleteFileIfExists(path)
	}

	return writeTasksToFile(path, tasks)
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

		if err := writeTaskFile(path, f.source, deleteIfEmpty); err != nil {
			return err
		}
	}

	return nil
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
