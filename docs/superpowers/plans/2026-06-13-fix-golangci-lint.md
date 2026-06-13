# Fix golangci-lint Errors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve all 47 golangci-lint errors in `main.go`, `tasks.go`, `main_test.go`, and `tasks_test.go`.

**Architecture:** Fix errors in dependency order — security/permission issues first, then mechanical style fixes (constants, line-length, blank-line rules), then function-order, then cyclomatic-complexity refactoring (which eliminates most of the remaining style issues inside the refactored functions), then sweep up any remaining wsl issues.

**Tech Stack:** Go 1.x, golangci-lint v2, urfave/cli v3

---

## Error inventory (47 total)

| Linter | Count | Affected files |
|--------|-------|----------------|
| wsl    | 29    | main.go, main_test.go, tasks.go, tasks_test.go |
| cyclop | 3     | main.go, tasks.go |
| gosec  | 4     | tasks.go, tasks_test.go |
| nlreturn | 5   | main.go, tasks.go |
| mnd    | 2     | main.go, tasks.go |
| gocognit | 1   | tasks.go |
| nestif | 1     | tasks.go |
| funcorder | 1  | tasks.go |
| lll    | 1     | main_test.go |

---

## Files touched

| File | Changes |
|------|---------|
| `tasks.go` | nolint sha1, permission 0600, metaSplitParts constant, reorder getTask, refactor prefixes/List/Write |
| `tasks_test.go` | permission 0600, blank lines for wsl |
| `main.go` | editMinArgs constant, extract action handlers, var block in handleTaskError, blank lines |
| `main_test.go` | wrap long line, blank line for wsl |

---

### Task 1: Fix gosec security warnings

**Files:**
- Modify: `tasks.go`
- Modify: `tasks_test.go`

The sha1 import is flagged as a blocked cryptographic primitive (G505, G401). SHA1 is used here for task-ID generation, not for any security purpose — a `//nolint:gosec` annotation with an explanatory comment is the correct fix. File permission 0644 must change to 0600 (G306).

- [ ] **Step 1: Add nolint annotations for sha1 in tasks.go**

Change the import and the hash call:

```go
// old import line:
	"crypto/sha1"

// new import line:
	"crypto/sha1" //nolint:gosec // used for task-ID generation, not cryptography
```

```go
// old hashText body:
func hashText(text string) string {
	h := sha1.New()

// new hashText body:
func hashText(text string) string {
	h := sha1.New() //nolint:gosec // SHA1 used for stable content-addressing, not security
```

- [ ] **Step 2: Fix WriteFile permission in tasks.go**

```go
// old:
		if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {

// new:
		if err := os.WriteFile(path, []byte(sb.String()), 0600); err != nil {
```

- [ ] **Step 3: Fix WriteFile permission in tasks_test.go**

```go
// old:
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {

// new:
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
```

- [ ] **Step 4: Verify gosec errors are gone**

```bash
cd /path/to/tgo
golangci-lint run ./... 2>&1 | grep gosec
```

Expected: no output (zero gosec errors).

- [ ] **Step 5: Run tests**

```bash
go test ./...
```

Expected: `ok github.com/nathanhruby/tgo`

- [ ] **Step 6: Commit**

```bash
git add tasks.go tasks_test.go
git commit -m "fix: suppress gosec sha1 warnings with nolint; use 0600 file permissions"
```

---

### Task 2: Add named constants for magic numbers (mnd)

**Files:**
- Modify: `tasks.go`
- Modify: `main.go`

- [ ] **Step 1: Add metaSplitParts constant in tasks.go**

Add a package-level constant just before `taskFromLine`, and use it in `strings.SplitN`:

```go
// Add after the hashText function, before taskFromLine:
const metaSplitParts = 2
```

Then in `taskFromLine`:
```go
// old:
			parts := strings.SplitN(piece, ":", 2)

// new:
			parts := strings.SplitN(piece, ":", metaSplitParts)
```

- [ ] **Step 2: Add editMinArgs constant in main.go**

Add a package-level constant near the top of main.go, after the `package main` declaration:

```go
// Add after the import block, before func main():
const editMinArgs = 2
```

Then in the `edit` command action:
```go
// old:
					if args.Len() < 2 {

// new:
					if args.Len() < editMinArgs {
```

- [ ] **Step 3: Verify mnd errors are gone**

```bash
golangci-lint run ./... 2>&1 | grep mnd
```

Expected: no output.

- [ ] **Step 4: Run tests**

```bash
go test ./...
```

Expected: `ok github.com/nathanhruby/tgo`

- [ ] **Step 5: Commit**

```bash
git add tasks.go main.go
git commit -m "fix: replace magic numbers with named constants (mnd)"
```

---

### Task 3: Fix long line in main_test.go (lll)

**Files:**
- Modify: `main_test.go`

Line 172 is 135 chars; limit is 120. Extract the args slice to a variable.

- [ ] **Step 1: Break the long line in TestCLI_CustomList**

```go
// old:
	if err := buildApp().Run(context.Background(), []string{"tgo", "--task-dir", dir, "--list", "groceries", "add", "Milk"}); err != nil {
		t.Fatalf("add to custom list failed: %v", err)
	}

// new:
	args := []string{"tgo", "--task-dir", dir, "--list", "groceries", "add", "Milk"}
	if err := buildApp().Run(context.Background(), args); err != nil {
		t.Fatalf("add to custom list failed: %v", err)
	}
```

- [ ] **Step 2: Verify lll error is gone**

```bash
golangci-lint run ./... 2>&1 | grep lll
```

Expected: no output.

- [ ] **Step 3: Run tests**

```bash
go test ./...
```

Expected: `ok github.com/nathanhruby/tgo`

- [ ] **Step 4: Commit**

```bash
git add main_test.go
git commit -m "fix: wrap long line in TestCLI_CustomList (lll)"
```

---

### Task 4: Fix method ordering — move getTask after Write (funcorder)

**Files:**
- Modify: `tasks.go`

`getTask` (unexported) must appear after all exported methods. Currently it is between `NewTaskList` and `Add`. Move the entire function to just after `Write`.

- [ ] **Step 1: Remove getTask from its current position**

Delete this block from its current location (between `NewTaskList` and `Add`):

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
```

- [ ] **Step 2: Paste getTask after the closing brace of Write**

Add the function at the end of the file, after `Write`:

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
```

- [ ] **Step 3: Verify funcorder error is gone**

```bash
golangci-lint run ./... 2>&1 | grep funcorder
```

Expected: no output.

- [ ] **Step 4: Run tests**

```bash
go test ./...
```

Expected: `ok github.com/nathanhruby/tgo`

- [ ] **Step 5: Commit**

```bash
git add tasks.go
git commit -m "fix: move unexported getTask after exported Write (funcorder)"
```

---

### Task 5: Reduce cyclomatic complexity of buildApp (cyclop)

**Files:**
- Modify: `main.go`

`buildApp` has cyclomatic complexity 16 because each inline action closure adds to its count. Extract each command action to a named top-level function. Each extracted function has low individual complexity.

The `editMinArgs` constant (from Task 2) is already defined at package level — use it in `editAction`.

- [ ] **Step 1: Add six action functions before buildApp**

Insert the following six functions just before `func buildApp()`. They must be written with blank lines that satisfy wsl and nlreturn rules:

```go
func addAction(_ context.Context, cmd *cli.Command) error {
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
}

func listAction(_ context.Context, cmd *cli.Command) error {
	tl, err := loadTaskList(cmd)
	if err != nil {
		return handleTaskError(err)
	}

	tl.List(os.Stdout, "tasks", cmd.Bool("verbose"), cmd.Bool("quiet"), cmd.String("grep"))

	return nil
}

func doneAction(_ context.Context, cmd *cli.Command) error {
	tl, err := loadTaskList(cmd)
	if err != nil {
		return handleTaskError(err)
	}

	tl.List(os.Stdout, "done", cmd.Bool("verbose"), cmd.Bool("quiet"), cmd.String("grep"))

	return nil
}

func finishAction(_ context.Context, cmd *cli.Command) error {
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
}

func removeAction(_ context.Context, cmd *cli.Command) error {
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
}

func editAction(_ context.Context, cmd *cli.Command) error {
	args := cmd.Args()
	if args.Len() < editMinArgs {
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
}
```

- [ ] **Step 2: Replace inline closures in buildApp with references to the new functions**

Replace the entire `Commands` slice in buildApp with:

```go
		Commands: []*cli.Command{
			{
				Name:      "add",
				Usage:     "Add a new task",
				ArgsUsage: "TEXT",
				Action:    addAction,
			},
			{
				Name:   "list",
				Usage:  "List open tasks",
				Flags:  listFlags(),
				Action: listAction,
			},
			{
				Name:   "done",
				Usage:  "List finished tasks",
				Flags:  listFlags(),
				Action: doneAction,
			},
			{
				Name:      "finish",
				Usage:     "Mark a task as finished",
				ArgsUsage: "TASK",
				Action:    finishAction,
			},
			{
				Name:      "remove",
				Usage:     "Remove a task from the list",
				ArgsUsage: "TASK",
				Action:    removeAction,
			},
			{
				Name:      "edit",
				Usage:     "Edit a task's text",
				ArgsUsage: "TASK NEW_TEXT",
				Action:    editAction,
			},
		},
```

- [ ] **Step 3: Verify cyclop error for buildApp is gone**

```bash
golangci-lint run ./... 2>&1 | grep cyclop
```

Expected: only tasks.go errors remain (List and Write — handled in Task 6), not main.go.

- [ ] **Step 4: Run tests**

```bash
go test ./...
```

Expected: `ok github.com/nathanhruby/tgo`

- [ ] **Step 5: Commit**

```bash
git add main.go
git commit -m "refactor: extract buildApp action closures to named functions (cyclop)"
```

---

### Task 6: Refactor tasks.go — reduce cyclop, gocognit, nestif

**Files:**
- Modify: `tasks.go`

Three functions need refactoring:
- `prefixes` (gocognit 34, nestif 9) — extract `resolveCollision` helper
- `List` (cyclop 13) — extract `buildLabelFn` and `printTasks` helpers
- `Write` (cyclop 11) — extract `writeTaskFile`, `sortedTasks`, `deleteFileIfExists`, `writeTasksToFile` helpers

#### 6a — Refactor `prefixes`

- [ ] **Step 1: Add resolveCollision before prefixes**

Insert this function just before `func prefixes(`:

```go
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
```

- [ ] **Step 2: Replace the collision-handling block in prefixes**

Replace the full body of `prefixes` with this refactored version:

```go
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
```

- [ ] **Step 3: Run prefixes tests to verify correctness**

```bash
go test ./... -run TestPrefixes
```

Expected: all TestPrefixes_* subtests pass.

#### 6b — Refactor `List`

- [ ] **Step 4: Add buildLabelFn and printTasks helpers**

Insert these two functions just before `func (tl *TaskList) List(`:

```go
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

// printTasks writes each task in ids to w, applying grep filtering and optional quiet/verbose formatting.
func printTasks(w io.Writer, tasks map[string]Task, ids []string, labelFn func(string) string, maxLen int, quiet bool, grep string) {
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
```

- [ ] **Step 5: Replace the body of List with delegation to helpers**

```go
// List prints the task list to w.
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

	labelFn, maxLen := buildLabelFn(ids, verbose)

	printTasks(w, tasks, ids, labelFn, maxLen, quiet, grep)
}
```

- [ ] **Step 6: Run list tests to verify correctness**

```bash
go test ./... -run TestList
```

Expected: all TestList_* subtests pass.

#### 6c — Refactor `Write`

- [ ] **Step 7: Add writeTaskFile, sortedTasks, deleteFileIfExists, writeTasksToFile helpers**

Insert these four functions just before `func (tl *TaskList) Write(`:

```go
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

	if err := os.WriteFile(path, []byte(sb.String()), 0600); err != nil {
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
```

- [ ] **Step 8: Replace the body of Write with delegation to helpers**

```go
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
```

- [ ] **Step 9: Verify all cyclop, gocognit, nestif errors are gone**

```bash
golangci-lint run ./... 2>&1 | grep -E "cyclop|gocognit|nestif"
```

Expected: no output.

- [ ] **Step 10: Run full test suite**

```bash
go test ./...
```

Expected: `ok github.com/nathanhruby/tgo`

- [ ] **Step 11: Commit**

```bash
git add tasks.go
git commit -m "refactor: extract helpers from prefixes/List/Write to reduce complexity (cyclop/gocognit/nestif)"
```

---

### Task 7: Fix remaining nlreturn issues

**Files:**
- Modify: `main.go`
- Modify: `tasks.go`

After the refactoring in Tasks 5 and 6, verify whether any nlreturn issues remain. The extracted action functions (Task 5) and the refactored helpers (Task 6) were written with blank lines before every `return`. If any slipped through, fix them here.

- [ ] **Step 1: Check remaining nlreturn issues**

```bash
golangci-lint run ./... 2>&1 | grep nlreturn
```

If output is empty, skip to Step 3. Otherwise continue.

- [ ] **Step 2: Add blank lines before any flagged return/break/continue**

For each flagged location, add a blank line immediately before the `return`, `break`, or `continue` statement. Example pattern:

```go
// Before fix (no blank line before return):
	someExpression()
	return someValue

// After fix:
	someExpression()

	return someValue
```

Apply this pattern to each line reported by the linter.

- [ ] **Step 3: Confirm no nlreturn errors**

```bash
golangci-lint run ./... 2>&1 | grep nlreturn
```

Expected: no output.

- [ ] **Step 4: Run tests**

```bash
go test ./...
```

Expected: `ok github.com/nathanhruby/tgo`

- [ ] **Step 5: Commit (if any changes were made)**

```bash
git add main.go tasks.go
git commit -m "fix: add blank lines before return/break/continue (nlreturn)"
```

---

### Task 8: Fix wsl whitespace issues in main.go

**Files:**
- Modify: `main.go`

Remaining wsl issue: `handleTaskError` declares four `var` statements that are flagged as cuddled declarations. Fix by grouping them into a single `var (...)` block and separating from the `switch` with a blank line.

- [ ] **Step 1: Group var declarations in handleTaskError**

```go
// old:
func handleTaskError(err error) error {
	var ae *ErrAmbiguousPrefix
	var ue *ErrUnknownPrefix
	var ie *ErrInvalidTaskFile
	var be *ErrBadFile
	switch {

// new:
func handleTaskError(err error) error {
	var (
		ae *ErrAmbiguousPrefix
		ue *ErrUnknownPrefix
		ie *ErrInvalidTaskFile
		be *ErrBadFile
	)

	switch {
```

- [ ] **Step 2: Verify wsl errors in main.go are gone**

```bash
golangci-lint run ./... 2>&1 | grep "main.go.*wsl"
```

Expected: no output.

- [ ] **Step 3: Run tests**

```bash
go test ./...
```

Expected: `ok github.com/nathanhruby/tgo`

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "fix: group var declarations in handleTaskError (wsl)"
```

---

### Task 9: Fix wsl whitespace issues in main_test.go

**Files:**
- Modify: `main_test.go`

Issue at line 14: `append` is not allowed to be cuddled with `t.Helper()`.

- [ ] **Step 1: Add blank line before allArgs assignment in runApp**

```go
// old:
func runApp(t *testing.T, dir string, args ...string) error {
	t.Helper()
	allArgs := append([]string{"tgo", "--task-dir", dir}, args...)

// new:
func runApp(t *testing.T, dir string, args ...string) error {
	t.Helper()

	allArgs := append([]string{"tgo", "--task-dir", dir}, args...)
```

- [ ] **Step 2: Verify wsl errors in main_test.go are gone**

```bash
golangci-lint run ./... 2>&1 | grep "main_test.go.*wsl"
```

Expected: no output.

- [ ] **Step 3: Run tests**

```bash
go test ./...
```

Expected: `ok github.com/nathanhruby/tgo`

- [ ] **Step 4: Commit**

```bash
git add main_test.go
git commit -m "fix: blank line before append in runApp (wsl)"
```

---

### Task 10: Fix wsl whitespace issues in tasks_test.go

**Files:**
- Modify: `tasks_test.go`

Three `tl.List(...)` calls immediately follow `var buf strings.Builder` declarations without a blank line.

- [ ] **Step 1: Add blank lines before tl.List calls**

In `TestList_PrintsTasksToStdout`:
```go
// old:
	var buf strings.Builder
	tl.List(&buf, "tasks", false, false, "")

// new:
	var buf strings.Builder

	tl.List(&buf, "tasks", false, false, "")
```

In `TestList_Grep`:
```go
// old:
	var buf strings.Builder
	tl.List(&buf, "tasks", false, false, "groceries")

// new:
	var buf strings.Builder

	tl.List(&buf, "tasks", false, false, "groceries")
```

In `TestList_Done`:
```go
// old:
	var buf strings.Builder
	tl.List(&buf, "done", false, false, "")

// new:
	var buf strings.Builder

	tl.List(&buf, "done", false, false, "")
```

- [ ] **Step 2: Verify wsl errors in tasks_test.go are gone**

```bash
golangci-lint run ./... 2>&1 | grep "tasks_test.go.*wsl"
```

Expected: no output.

- [ ] **Step 3: Run tests**

```bash
go test ./...
```

Expected: `ok github.com/nathanhruby/tgo`

- [ ] **Step 4: Commit**

```bash
git add tasks_test.go
git commit -m "fix: blank lines before tl.List calls in tests (wsl)"
```

---

### Task 11: Fix wsl whitespace issues in tasks.go

**Files:**
- Modify: `tasks.go`

After the refactoring in Task 6, many wsl issues in `List`, `Write`, and `prefixes` are gone. The remaining wsl issues are in `taskFromLine`, `NewTaskList`, `Add`, `Finish`, `Remove`, and `getTask`. Apply blank lines as described below.

- [ ] **Step 1: Verify current wsl state**

```bash
golangci-lint run ./... 2>&1 | grep "tasks.go.*wsl"
```

Note the exact set of lines reported, then apply the fixes below.

- [ ] **Step 2: Fix taskFromLine — blank line before second `if task.ID`**

```go
// old (inside the `if idx := strings.LastIndex...` branch):
		}
		if task.ID == "" {

// new:
		}

		if task.ID == "" {
```

- [ ] **Step 3: Fix NewTaskList — five blank lines**

Add blank line before `files := []fileSpec{` (after the type declaration):
```go
// old:
	type fileSpec struct {
		dest     map[string]Task
		filename string
	}
	files := []fileSpec{

// new:
	type fileSpec struct {
		dest     map[string]Task
		filename string
	}

	files := []fileSpec{
```

Add blank line before `if err == nil && fi.IsDir()` (after `fi, err := os.Stat`):
```go
// old:
		fi, err := os.Stat(path)
		if err == nil && fi.IsDir() {

// new:
		fi, err := os.Stat(path)

		if err == nil && fi.IsDir() {
```

Add blank line before `if err != nil` (after the `os.ReadFile` call):
```go
// old:
		data, err := os.ReadFile(path)
		if err != nil {

// new:
		data, err := os.ReadFile(path)

		if err != nil {
```

Add blank line before `for _, line := range` (after the `if err != nil` block):
```go
// old:
		}
		for _, line := range strings.Split(string(data), "\n") {

// new:
		}

		for _, line := range strings.Split(string(data), "\n") {
```

Add blank line before `if ok {` (after the `if err != nil` block inside the range):
```go
// old:
			if err != nil {
				return nil, &ErrBadFile{Path: path, Problem: err.Error()}
			}
			if ok {

// new:
			if err != nil {
				return nil, &ErrBadFile{Path: path, Problem: err.Error()}
			}

			if ok {
```

- [ ] **Step 4: Fix Add — two blank lines**

Add blank line before `id := hashText(text)` (after the if-newline block):
```go
// old:
	}
	id := hashText(text)

// new:
	}

	id := hashText(text)
```

Add blank line before `for taskID := range tl.Tasks` (after `ids := make(...)`):
```go
// old:
	ids := make([]string, 0, len(tl.Tasks))
	for taskID := range tl.Tasks {

// new:
	ids := make([]string, 0, len(tl.Tasks))

	for taskID := range tl.Tasks {
```

- [ ] **Step 5: Fix Finish — blank line before delete**

```go
// old:
	}
	delete(tl.Tasks, task.ID)
	tl.Done[task.ID] = task

// new:
	}

	delete(tl.Tasks, task.ID)
	tl.Done[task.ID] = task
```

- [ ] **Step 6: Fix Remove — blank line before delete**

```go
// old (in Remove):
	}
	delete(tl.Tasks, task.ID)
	return nil

// new:
	}

	delete(tl.Tasks, task.ID)

	return nil
```

- [ ] **Step 7: Fix getTask — two blank lines**

Add blank line before `for id := range tl.Tasks` (after `var matched []string`):
```go
// old:
	var matched []string
	for id := range tl.Tasks {

// new:
	var matched []string

	for id := range tl.Tasks {
```

Add blank line before `switch len(matched)` (after the for block):
```go
// old:
	}
	switch len(matched) {

// new:
	}

	switch len(matched) {
```

- [ ] **Step 8: Verify all wsl errors are gone**

```bash
golangci-lint run ./... 2>&1 | grep wsl
```

Expected: no output.

- [ ] **Step 9: Run full test suite**

```bash
go test ./...
```

Expected: `ok github.com/nathanhruby/tgo`

- [ ] **Step 10: Commit**

```bash
git add tasks.go
git commit -m "fix: add blank lines throughout tasks.go (wsl)"
```

---

### Task 12: Final verification

- [ ] **Step 1: Run golangci-lint on the full project**

```bash
golangci-lint run ./... 2>&1
```

Expected output: only the two deprecation WARNs about `gomodguard` and `wsl` (these are about linter versions, not code errors). Zero issue lines.

If any issues remain, fix them using the same patterns applied in the tasks above.

- [ ] **Step 2: Run full test suite**

```bash
go test -race ./...
```

Expected: `ok github.com/nathanhruby/tgo`

- [ ] **Step 3: Final commit**

```bash
git add -A
git commit -m "chore: all golangci-lint errors resolved"
```
