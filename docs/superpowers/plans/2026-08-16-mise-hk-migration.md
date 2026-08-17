# mise + hk Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Make and pre-commit with pinned mise tasks and hk hooks while preserving repository checks and exact PR-title validation used by release-note generation.

**Architecture:** `mise.toml` is the single command and tool-version entry point. `hk.pkl` owns local Git-hook behavior and calls hk utilities, mise tasks, and repository check scripts. GitHub Actions installs the committed mise environment through `jdx/mise-action` and invokes the same read-only tasks; the PR-title adapter delegates conventional-commit parsing to `hk util check-conventional-commit` and retains only the rules hk does not express.

**Tech Stack:** Go 1.26.6, mise, hk 1.53.0, `golangci-lint` 2.12.2, `yamlfmt` 0.21.0, `yamllint` 1.38.0, Pkl, POSIX shell, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-08-16-mise-hk-migration-design.md`

## Global Constraints

- Replace `make build`, `make test`, `make lint`, and `make devprep` with `mise run build`, `mise run test`, `mise run lint`, and `mise run hooks:install`.
- Preserve PR-title types exactly: `fix`, `feat`, `docs`, `ci`, and `chore`.
- Preserve optional PR-title scopes and the subject regex `^[a-z].+$`.
- Reject `fixup!`, `squash!`, and `amend!` PR-title bypasses even though hk’s utility skips them for commit-message use.
- Preserve `pull_request_target`, events `opened`, `edited`, and `synchronize`, and `pull-requests: read` permissions.
- Keep local fixer behavior for EOF, trailing whitespace, and LF line endings; CI must be read-only.
- Do not add `.commitlintrc.yaml` or introduce a new commit-message policy.
- Keep `.github/workflows/golang-ci.yml` and `.github/workflows/goreleaser.yml` functionally unchanged.
- Use `jdx/mise-action@7e36c90d9ab29c415a2384db3006f3ec8a8cc654` (`v4.2.4`) in new workflows.
- Do not commit changes unless the user explicitly requests a commit.

---

## File Map

### New files

- `mise.toml` — pinned tools and canonical project tasks.
- `hk.pkl` — explicit hk hook and check definitions.
- `.mise/tasks/check-vcs-permalinks` — replacement for `check-vcs-permalinks`.
- `.mise/tasks/forbid-new-submodules` — replacement for `forbid-new-submodules`.
- `.mise/tasks/trailing-whitespace` — fixer/checker preserving Markdown hard-line-break spaces.
- `.mise/tasks/mixed-line-ending` — exact LF normalizer replacing pre-commit's `--fix=lf` behavior, which hk's builtin does not provide.
- `.mise/tasks/pr-title` — title adapter using `hk util check-conventional-commit` plus the existing lowercase/no-bypass rules.

### Modified files

- `.github/workflows/pre-commit.yml` — replace Python/pre-commit execution with mise and hk.
- `.github/workflows/pr-title.yml` — replace the semantic-PR action with mise-managed title validation.
- `.renovaterc.json` — remove stale pre-commit manager settings while retaining GitHub Actions management.
- `.gitignore` — unignore the committed `.mise/tasks/` migration scripts despite the legacy `tasks` ignore rule.
- `README.md` — document mise, hk, tasks, and hook installation.

### Removed files

- `Makefile`
- `.pre-commit-config.yaml`

### Intentionally unchanged

- `.github/workflows/golang-ci.yml`
- `.github/workflows/goreleaser.yml`
- `.golangci.yml`
- Go source and tests
- `.commitlintrc.yaml` remains absent.

---

### Task 1: Add the mise tool and task foundation

**Files:**
- Create: `mise.toml`
- Inspect: `go.mod`, `Makefile`

**Interfaces:**
- Produces `mise run build`, `mise run test`, `mise run lint`, `mise run check`, `mise run check:ci`, `mise run pr-title`, and `mise run hooks:install`.
- Makes `go`, `golangci-lint`, `yamlfmt`, `yamllint`, and `hk` available at pinned versions to later tasks.

- [ ] **Step 1: Write the mise configuration**

Create `mise.toml` with:

```toml
min_version = "2024.1.1"

[tools]
go = "1.26.6"
golangci-lint = "2.12.2"
yamlfmt = "0.21.0"
yamllint = "1.38.0"
hk = "1.53.0"

[tasks.build]
run = "go build -o dist/tgo"

[tasks.test]
run = "go test ./..."

[tasks.lint]
run = "golangci-lint run ./..."

[tasks.check]
run = "hk run check --all --check"

[tasks."check:ci"]
run = "hk run check --all --check --skip-step golangci-lint"

[tasks.pr-title]
run = ".mise/tasks/pr-title"

[tasks."hooks:install"]
run = "hk install --mise"
```

The implementation must keep `check` as the complete check and must use `check:ci` for the existing CI split that runs Go lint in `golang-ci.yml`.

- [ ] **Step 2: Install the declared tools**

Run:

```sh
mise install
mise exec -- go version
mise exec -- golangci-lint version
mise exec -- yamlfmt --version
mise exec -- yamllint --version
mise exec -- hk --version
```

Expected: Go reports `go1.26.6`, `golangci-lint` reports `2.12.2`, `yamlfmt` reports `0.21.0`, `yamllint` reports `1.38.0`, and hk reports `1.53.0`.

- [ ] **Step 3: Validate the basic project tasks**

Run:

```sh
mise run build
mise run test
mise run lint
```

Expected: the existing Go build, tests, and lint complete successfully. `dist/tgo` may be created by the build and remains ignored by `.gitignore`.

---

### Task 2: Implement standalone repository check scripts

**Files:**
- Create: `.mise/tasks/check-vcs-permalinks`
- Create: `.mise/tasks/forbid-new-submodules`
- Create: `.mise/tasks/trailing-whitespace`

**Interfaces:**
- Each script accepts file paths as positional arguments, returns status `0` when valid, and returns non-zero with affected paths on failure; `trailing-whitespace` additionally accepts `--fix` before its paths.
- Scripts must work when called by hk with staged paths and when called over the complete checkout in CI.
- YAML formatting and YAML style checks are provided by hk builtins, not scripts in this task.

- [ ] **Step 1: Write fixture cases for the repository-specific checks**

Use a temporary directory outside the repository working tree and create these fixtures:

```sh
fixture_dir="$(mktemp -d)"
branch_ref=main
printf '%s\n' '[mutable link](https://github.com/nathanhruby/tgo/blob'"/$branch_ref"'/README.md#L12)' > "$fixture_dir/mutable.md"
printf '%s\n' '[fixed link](https://github.com/nathanhruby/tgo/blob/0123456789abcdef0123456789abcdef01234567/README.md)' > "$fixture_dir/permalink.md"
```

For the submodule check, create a temporary Git repository with a staged gitlink entry using mode `160000`, then confirm the script rejects the staged addition. Do not add a submodule to the project repository.

- [ ] **Step 2: Implement VCS permalink validation**

Implement `.mise/tasks/check-vcs-permalinks` to reproduce the existing hook’s intent:

- Inspect text files supplied by hk.
- Reproduce the pinned hook’s GitHub URL pattern: inspect `https://<domain>/<owner>/<repo>/blob/<ref>/<path>#L<number>` links.
- Treat a ref as permanent when it is 4–64 hexadecimal characters; reject other refs, including branch names such as `main` and `master`.
- Report each offending file and matching line, and keep the default domain `github.com`.
- Do not reject links without the `blob/...#L<number>` shape, matching the former hook.
- Return zero when no VCS links are present or all detected links are immutable.

Include fixture assertions that the branch URL fails, a 40-character SHA URL passes, and a short 4-character hexadecimal SHA URL passes.

- [ ] **Step 3: Implement new-submodule detection**

Implement `.mise/tasks/forbid-new-submodules` to reject newly added gitlink entries while allowing the repository to run when no submodules are present:

- For staged local-hook execution, inspect the staged index with `git diff --cached --raw` and reject added entries whose new mode is `160000`.
- For full-checkout CI execution, inspect tracked index entries with `git ls-files --stage` and reject any gitlink entry.
- Print the path of each rejected entry.
- Return zero when no gitlink is found.

- [ ] **Step 4: Implement Markdown-safe trailing whitespace handling**

Implement `.mise/tasks/trailing-whitespace` with these interfaces:

```sh
.mise/tasks/trailing-whitespace [--fix] <path>...
```

The check mode returns non-zero for trailing whitespace except exactly two spaces at the end of a Markdown line, matching the former `--markdown-linebreak-ext=md` behavior. The fix mode removes other trailing whitespace, preserves those Markdown hard-line-break spaces, preserves file contents outside the affected whitespace, and never modifies `CHANGELOG.md`; hk will exclude that file before invoking the script. The script must report affected paths and never rewrite files in check mode.

- [ ] **Step 5: Run the standalone script tests**

Run:

```sh
! .mise/tasks/check-vcs-permalinks "$fixture_dir/mutable.md"
.mise/tasks/check-vcs-permalinks "$fixture_dir/permalink.md"
.mise/tasks/forbid-new-submodules README.md
printf 'text  \n' > "$fixture_dir/line.md"
printf 'text \t\n' > "$fixture_dir/whitespace.txt"
.mise/tasks/trailing-whitespace --fix "$fixture_dir/line.md" "$fixture_dir/whitespace.txt"
grep -F 'text  ' "$fixture_dir/line.md"
grep -Fx 'text' "$fixture_dir/whitespace.txt"
rm -rf "$fixture_dir"
```

Expected: immutable links pass; mutable VCS links fail; the current repository passes the submodule check. YAML fixture behavior is tested through the hk builtin checks in Task 3.

---

### Task 3: Add hk configuration and connect all checks

**Files:**
- Create: `hk.pkl`
- Modify: `mise.toml`

**Interfaces:**
- Produces `hk validate`, `hk run pre-commit`, `hk run check`, `mise run check:ci`, and `mise run hooks:install`.
- `pre-commit` uses fixer commands by default; `check` uses read-only commands.
- `yamlfmt` may fix YAML locally; `yamllint` is read-only in both hooks and CI.
- The Go lint step is named `golangci-lint` so CI can skip it with `--skip-step golangci-lint`.

- [ ] **Step 1: Define the hk configuration imports and reusable steps**

Use these pinned hk `1.53.0` imports:

```pkl
amends "package://github.com/jdx/hk/releases/download/v1.53.0/hk@1.53.0#/Config.pkl"
import "package://github.com/jdx/hk/releases/download/v1.53.0/hk@1.53.0#/Builtins.pkl"
```

Define `local linters = new Mapping<String, Step> { ... }` with one named entry for each check. The entries must use these commands and settings:

- `check-added-large-files`: `check = "hk util check-added-large-files {{files}}"`, `glob = "**/*"`.
- `check-merge-conflict`: `check = "hk util check-merge-conflict {{files}}"`, `glob = "**/*"`.
- `check-case-conflict`: `check = "hk util check-case-conflict {{files}}"`, `glob = "**/*"`.
- `check-executables-have-shebangs`: `check = "hk util check-executables-have-shebangs {{files}}"`, `glob = "**/*"`.
- `detect-private-key`: `check = "hk util detect-private-key {{files}}"`, `glob = "**/*"`.
- `end-of-file`: `check = "hk util end-of-file-fixer {{files}}"`, `fix = "hk util end-of-file-fixer --fix {{files}}"`, `glob = "**/*"`, and `check_first = true`.
- `trailing-whitespace`: `check = "mise run trailing-whitespace {{files}}"`, `fix = "mise run trailing-whitespace -- --fix {{files}}"`, `glob = "**/*"`, `exclude = List("CHANGELOG.md")`, and `check_first = true`; the dedicated script preserves exactly two trailing spaces on Markdown lines.
- `mixed-line-ending`: `check = "mise run mixed-line-ending {{files}}"`, `fix = "mise run mixed-line-ending -- --fix {{files}}"`, `glob = "**/*"`, and `check_first = true`; this preserves the former `--fix=lf` behavior because hk's builtin only normalizes mixed endings.
- `yamlfmt`: use `Builtins.yamlfmt`, with its formatter as the local `fix` command and its check command in CI.
- `yamllint`: use `Builtins.yamllint` as a read-only lint step and serialize it after `yamlfmt` so local formatting completes before style validation.
- `check-vcs-permalinks`: `check = "mise run check-vcs-permalinks {{files}}"`, `glob = "**/*"`; implement the exact `blob/<ref>/<path>#L<number>` and 4–64 hexadecimal-ref behavior described in Task 2.
- `forbid-new-submodules`: `check = "mise run forbid-new-submodules {{files}}"`, `glob = "**/*"`.
- `golangci-lint`: `check = "mise run lint"`, `glob = List("**/*.go", "go.mod", "go.sum")`, and `exclusive = true`.

Use the builtin forms directly in `hk.pkl`:

```pkl
["yamlfmt"] = Builtins.yamlfmt
["yamllint"] = Builtins.yamllint
```

Configure `yamlfmt` to apply local fixes and run read-only under `hk run check --check`; keep `yamllint` read-only in both local hooks and CI. The broadened policy intentionally allows these builtins to reject style issues that the former syntax-only hook accepted.

Name the Go step exactly `golangci-lint`, and keep each fixer’s `check` and `fix` commands separate so CI can force read-only execution.

- [ ] **Step 2: Define the pre-commit and check hooks**

Configure:

```pkl
hooks {
  ["pre-commit"] {
    fix = true
    stash = "git"
    steps = linters
  }
  ["check"] {
    fix = false
    steps = linters
  }
}
```

The resulting `hk.pkl` must parse under hk `1.53.0`; `pre-commit` fixes and stages safe formatting changes, while `check` never writes.

- [ ] **Step 3: Validate the hk configuration**

Run:

```sh
mise exec -- hk validate
mise exec -- hk run check --all --check --skip-step golangci-lint
```

Expected: configuration validation succeeds, and all repository checks except Go lint run without modifying tracked or untracked files.

- [ ] **Step 4: Test the YAML builtins in a temporary Git worktree**

Create a temporary worktree, add a YAML file containing formatter-visible spacing such as `key:    value`, and stage it. Run:

```sh
hk run pre-commit
```

Expected: `Builtins.yamlfmt` formats the staged YAML file. Then create a YAML file with a scalar line longer than 80 characters, run `hk run check --all --check`, and confirm the configured `Builtins.yamllint` line-length rule fails without changing the file. The test must also confirm that `hk run check --all --check` does not rewrite either fixture.

- [ ] **Step 5: Test local fixer behavior in a temporary Git worktree**

Create a temporary worktree from the current repository, introduce trailing whitespace and CRLF content in a temporary text file, stage it, then run:

```sh
hk run pre-commit
```

Expected: hk normalizes the same whitespace/line-ending classes formerly fixed by pre-commit, stages those fixes, and leaves unrelated unstaged changes intact. Remove the temporary worktree after the test.

---

### Task 4: Implement exact PR-title validation through hk

**Files:**
- Create: `.mise/tasks/pr-title`
- Modify: `mise.toml`

**Interfaces:**
- `mise run pr-title -- "<title>"` validates one title.
- The script also accepts `PR_TITLE` for GitHub Actions.
- Conventional-commit structure and allowed types are delegated to `hk util check-conventional-commit`.

- [ ] **Step 1: Write the PR-title acceptance matrix**

Use this expected matrix:

| Title | Expected |
| --- | --- |
| `fix: repair parser` | pass |
| `feat(cli): add export` | pass |
| `docs: update README` | pass |
| `docs: Update README` | fail: uppercase subject |
| `ci: update workflow` | pass |
| `chore: refresh tools` | pass |
| `refactor: simplify parser` | fail: unsupported type |
| `feat:` | fail: empty subject |
| `feat: Add export` | fail: uppercase subject |
| `fixup! fix: temporary` | fail: no hk autosquash bypass for PR titles |
| `squash! feat: temporary` | fail: no hk autosquash bypass for PR titles |
| `amend! docs: temporary` | fail: no hk autosquash bypass for PR titles |

- [ ] **Step 2: Implement the title adapter**

Implement `.mise/tasks/pr-title` so it:

1. Accepts exactly one positional title or reads `PR_TITLE`.
2. Fails when the title is absent.
3. Rejects `fixup! `, `squash! `, and `amend! ` before invoking hk.
4. Writes the title plus one newline to a securely created temporary file.
5. Runs:

```sh
hk util check-conventional-commit \
  --allowed-types fix,feat,docs,ci,chore \
  "$title_file"
```

6. Extracts the conventional-commit subject and rejects it unless it matches `^[a-z].+$`.
7. Uses a trap or equivalent cleanup path so the temporary file is removed on success, hk failure, parser failure, and interrupted execution.
8. Prints a concise accepted-format diagnostic on failure.

Do not duplicate hk’s conventional-commit parser in shell code.

- [ ] **Step 3: Run the acceptance matrix**

Run each title through `mise run pr-title -- "<title>"` and compare the exit status to the table above. Also run:

```sh
PR_TITLE='feat(scope): add export' mise run pr-title
```

Expected: both argument and environment-variable forms pass for a valid title, and every invalid matrix case fails.

---

### Task 5: Replace the two GitHub workflows

**Files:**
- Modify: `.github/workflows/pre-commit.yml`
- Modify: `.github/workflows/pr-title.yml`

**Interfaces:**
- Both workflows use the pinned `jdx/mise-action` commit.
- The PR-title workflow remains safe for untrusted pull-request branches because it checks out the pull request base commit and runs only trusted workflow-revision logic under `pull_request_target`.

- [ ] **Step 1: Replace the pre-commit workflow setup**

Keep the existing pull-request trigger and `permissions: read-all`. Keep the existing checkout action and digest. Replace Python setup and `pre-commit/action` with:

```yaml
- uses: jdx/mise-action@7e36c90d9ab29c415a2384db3006f3ec8a8cc654 # v4.2.4
  with:
    version: 2026.3.10
    install: true

- name: Run repository checks
  run: mise run check:ci
```

The check command must be read-only and must not use Python or pre-commit.

- [ ] **Step 2: Replace the PR-title action**

Keep:

```yaml
on:
  pull_request_target:
    types:
      - opened
      - edited
      - synchronize

permissions:
  pull-requests: read
```

Check out the pull request base commit using the existing pinned checkout action so the workflow reads `mise.toml` and `.mise/tasks/pr-title` from trusted repository content, never from the pull-request head. Add the pinned mise action and pass the event title through an environment variable:

```yaml
- name: Checkout trusted base
  uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
  with:
    ref: ${{ github.event.pull_request.base.sha }}

- uses: jdx/mise-action@7e36c90d9ab29c415a2384db3006f3ec8a8cc654 # v4.2.4
  with:
    version: 2026.3.10
    install: true

- name: Validate PR title
  env:
    PR_TITLE: ${{ github.event.pull_request.title }}
  run: mise run pr-title
```

Do not check out the pull-request head or execute files supplied by it. The only checkout ref is `${{ github.event.pull_request.base.sha }}`.

- [ ] **Step 3: Validate workflow structure**

Run a YAML parser against both workflow files and inspect the resulting text for:

```sh
grep -E 'pull_request_target|opened|edited|synchronize|pull-requests: read|jdx/mise-action@7e36c90d9ab29c415a2384db3006f3ec8a8cc654' .github/workflows/pr-title.yml
! grep -E 'pre-commit|setup-python|amannn/action-semantic-pull-request' .github/workflows/pre-commit.yml .github/workflows/pr-title.yml
 grep -F 'github.event.pull_request.base.sha' .github/workflows/pr-title.yml
```

Expected: the original PR-title trigger/permission contract remains, and neither replacement workflow invokes the removed tooling.

---

### Task 6: Remove obsolete tooling, update Renovate, and document the new workflow

**Files:**
- Delete: `Makefile`
- Delete: `.pre-commit-config.yaml`
- Modify: `.renovaterc.json`
- Modify: `.gitignore`
- Modify: `README.md`

**Interfaces:**
- Contributors use mise commands and hk installation instructions documented in `README.md`.
- Renovate continues managing GitHub Actions without managing removed pre-commit configuration.

- [ ] **Step 1: Remove stale Renovate pre-commit settings**

Delete `:enablePreCommit` from the Renovate `extends` list and remove `pre-commit` from `matchManagers`. Preserve the remaining best-practices, dependency, GitHub Actions, labels, reviewer, schedule, timezone, and automerge settings.

- [ ] **Step 2: Update contributor documentation**

Add a concise development section to `README.md` that documents:

```text
mise install
mise run build
mise run test
mise run lint
mise run check
mise run hooks:install
mise run pr-title -- "feat: add export"
```

Explain that mise provides tool versions/tasks and hk replaces pre-commit for local hooks. State that the repository intentionally does not add a commit-message policy because `.commitlintrc.yaml` is absent.

Add narrow negated patterns to `.gitignore` so the existing `tasks` rule does not hide committed `.mise/tasks/` scripts.

- [ ] **Step 3: Remove the old command/configuration files**

Delete `Makefile` and `.pre-commit-config.yaml` after all references have been migrated. Do not delete or modify the dedicated `golang-ci` and GoReleaser workflows.

- [ ] **Step 4: Search for stale references**

Run:

```sh
grep -RInE '(^|[^[:alnum:]_-])(make|Makefile|pre-commit|commitlint)([^[:alnum:]_-]|$)' \
  --exclude-dir=.git --exclude='2026-08-16-mise-hk-migration-design.md' \
  --exclude='2026-08-16-mise-hk-migration.md' .
```

Expected: no active contributor or workflow reference remains except intentional historical/spec documentation and the explicitly documented absent commitlint configuration.

---

### Task 7: Run the complete migration verification

**Files:**
- Inspect: all changed files from Tasks 1–6

- [ ] **Step 1: Validate configuration and tool installation**

Run:

```sh
mise install
mise run --help
hk validate
```

Expected: all tools resolve from the pinned configuration and hk validates `hk.pkl`.

- [ ] **Step 2: Run application checks**

Run:

```sh
mise run build
mise run test
mise run lint
```

Expected: all commands pass.

- [ ] **Step 3: Run repository checks in read-only mode**

Record file metadata before and after:

```sh
before="$(git --no-optional-locks status --short)"
mise run check
after="$(git --no-optional-locks status --short)"
test "$before" = "$after"
```

Expected: `mise run check` passes and does not modify the worktree or index.

- [ ] **Step 4: Run the PR-title matrix again through the final mise configuration**

Run all cases from Task 4 and confirm exit statuses remain unchanged after workflow/task integration.

- [ ] **Step 5: Validate workflow YAML and preserved workflows**

Run the YAML parser against `.github/workflows/pre-commit.yml`, `.github/workflows/pr-title.yml`, `.github/workflows/golang-ci.yml`, and `.github/workflows/goreleaser.yml`. Compare the latter two files against their pre-migration contents and confirm no functional edits were made.

- [ ] **Step 6: Inspect the final change set**

Run:

```sh
git diff --check
git status --short
git diff --stat
git diff -- .github/workflows/golang-ci.yml .github/workflows/goreleaser.yml
```

Expected: no whitespace errors, only intended files changed, and the two explicitly unchanged workflows have no diff.

## Completion Criteria

- `mise run build`, `mise run test`, `mise run lint`, and `mise run check` pass.
- `hk validate` passes.
- Local hk hooks perform the former safe fixes and checks without pre-commit.
- CI pre-commit validation is mise/hk-based, read-only, and skips only the separately managed Go-lint step.
- PR-title validation uses hk’s conventional-commit utility and preserves every existing release-note rule.
- The PR-title workflow retains its original trigger events and `pull-requests: read` permission.
- Make and pre-commit configuration are removed.
- README and Renovate contain no stale active references.
- Go source, tests, golang-ci, and GoReleaser behavior remain unchanged.
