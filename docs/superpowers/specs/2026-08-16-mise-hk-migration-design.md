# mise + hk Migration Design

**Date:** 2026-08-16
**Project:** `tgo`
**Status:** Approved in chat; implementation pending

## Goals

- Make `mise` the canonical command and tool-version layer.
- Pin Go, `golangci-lint`, and `hk` through `mise.toml`.
- Make `hk` the owner of local Git hooks.
- Preserve the current repository checks and their intent.
- Preserve PR-title validation exactly because release notes depend on it.
- Run equivalent checks in CI through mise-managed tasks.
- Keep the existing `golang-ci` and GoReleaser workflows functionally unchanged.
- Keep local fixer behavior distinct from read-only CI validation.

## Non-goals

- Change the Go application or its runtime behavior.
- Introduce a new commit-message policy.
- Invent or add the currently referenced but absent `.commitlintrc.yaml`.
- Tighten or broaden accepted PR-title types, scopes, subject formatting, events, or permissions.
- Migrate the existing `golang-ci` or GoReleaser workflow implementations beyond what is required to leave them functionally unchanged.
- Perform unrelated dependency or repository cleanup.

## Current Behavior

### Developer commands

The `Makefile` currently provides:

| Command | Current behavior |
| --- | --- |
| `make build` | `go build -o dist/tgo` |
| `make test` | `go test ./...` |
| `make lint` | `golangci-lint run ./...` |
| `make devprep` | `pre-commit install` |

The migration replaces these with mise tasks and removes the `Makefile`.

### Local pre-commit checks

`.pre-commit-config.yaml` currently pins:

- `pre-commit-hooks` at `v6.0.0`
- `commitlint-pre-commit-hook` at `v9.26.0`
- `golangci-lint` at `v2.12.2`

The configured checks cover large files, merge conflicts, VCS permalinks, new submodules, YAML syntax, executable shebangs, case conflicts, EOF normalization, trailing whitespace, mixed line endings, private keys, and Go linting. `check-merge-conflict` is listed twice; the migration will run it once because the duplicate has no behavior difference. The `no-commit-to-branch` hook is disabled and remains disabled.

The commitlint hook references `.commitlintrc.yaml`, but that file is absent from the repository. The migration will not infer or introduce a stricter commit-message policy. PR-title validation remains mandatory and is specified separately below.

### PR-title validation

The current workflow uses `pull_request_target` for:

- `opened`
- `edited`
- `synchronize`

It grants only `pull-requests: read` permissions.

Accepted types are exactly `fix`, `feat`, `docs`, `ci`, and `chore`. Scopes are optional. The subject must match `^[a-z].+$`. WIP titles are not exempted, and single-commit messages are not separately validated. These semantics must remain unchanged.

### Other workflows

The `golang-ci` and GoReleaser workflows remain functionally unchanged. GitHub Actions dependency updates continue to be managed by Renovate.

## Architecture

### `mise.toml`

Add a root `mise.toml` that:

- Pins Go to the validated installed patch `1.26.6`, compatibly with the `go.mod` declaration.
- Pins `golangci-lint` to the current intended release, `2.12.2`, unless implementation validation identifies a compatibility requirement.
- Pins `yamlfmt` to `0.21.0` and `yamllint` to `1.38.0`.
- Pins `hk` to an explicit release.
- Defines these tasks:

| Task | Responsibility |
| --- | --- |
| `build` | Run `go build -o dist/tgo` |
| `test` | Run `go test ./...` |
| `lint` | Run `golangci-lint run ./...` |
| `check` | Run the complete read-only repository validation suite, including Go lint |
| `check:ci` | Run the read-only repository validation suite while skipping Go lint, which has a dedicated workflow |
| `pr-title` | Adapt a supplied PR title to `hk util check-conventional-commit`, then enforce the remaining current rules |
| `hooks:install` | Install the hk-managed local hooks with mise integration |

The Go patch release is pinned to the validated installed `1.26.6`, and the hk release is pinned to `1.53.0`.

### `hk.pkl`

Add an explicit hk configuration. Every local check must be named or mapped deliberately; the configuration must not rely on an implicit aggregate that could silently omit a check.

The `pre-commit` hook will run the repository checks and Go lint. It may apply safe formatting fixes. The `check` hook will expose the same checks in read-only form for CI. The CI pre-commit replacement will skip the Go-lint step because the existing repository already has a dedicated `golang-ci` workflow; `mise run check` remains the complete local/standalone check command.

The mapping must distinguish:

- Checks implemented by hk utilities.
- Checks implemented by dedicated `.mise/tasks/` scripts because exact parity or input adaptation is not available through hk alone.
- Fixing commands used locally.
- Read-only commands used in CI.

## Check Mapping

The following checks must remain covered:

| Existing check | hk/mise design |
| --- | --- |
| `check-added-large-files` | Explicit hk mapping or dedicated task; reject oversized added files |
| `check-merge-conflict` | Explicit hk mapping; detect conflict markers |
| `check-case-conflict` | Explicit hk mapping; detect paths that collide by case |
| `check-executables-have-shebangs` | Explicit hk mapping; reject executable text files without shebangs |
| `detect-private-key` | Explicit hk mapping; reject recognized private-key material |
| `end-of-file-fixer` | Local fixer mapping plus read-only CI validation |
| `trailing-whitespace` | Local fixer mapping, preserving Markdown line breaks and excluding `CHANGELOG.md`; read-only CI validation |
| `mixed-line-ending` | Local fixer to LF plus read-only CI validation |
| YAML formatting | hk’s `Builtins.yamlfmt`, with local fixes and read-only CI checks |
| YAML style/lint | hk’s `Builtins.yamllint`, read-only in local hooks and CI |
| `check-vcs-permalinks` | `.mise/tasks/check-vcs-permalinks`, explicitly mapped from hk and callable directly |
| `forbid-new-submodules` | `.mise/tasks/forbid-new-submodules`, explicitly mapped from hk and callable directly |
| `golangci-lint` | Pinned mise tool, invoked by the `lint` task and hk pre-commit/check hooks |
| PR-title conventional structure | `hk util check-conventional-commit --allowed-types fix,feat,docs,ci,chore` against a temporary title file |
| PR-title lowercase subject | Adapter task preserves the existing `^[a-z].+$` rule after hk validates the title structure |

Exact parity for unsupported checks must be implemented through dedicated tasks. A check must not be silently dropped merely because hk does not provide a matching built-in.

The local hook may fix YAML formatting, EOF, trailing whitespace, and line endings. YAML linting and all other checks are read-only. CI must run every check in validation-only mode and must fail without modifying the checkout.

## Mise Tasks and Scripts

Add executable scripts under `.mise/tasks/` for:

- VCS permalink validation.
- New-submodule detection.
- Markdown-safe trailing whitespace handling.
- PR-title validation.

Use hk builtins for YAML formatting and style validation; do not add a custom YAML parser script.

The PR-title task must accept a title argument or an explicitly documented environment variable so it can be called both locally and from GitHub Actions. It writes that title to a temporary file and delegates conventional-commit structure and allowed-type validation to `hk util check-conventional-commit --allowed-types fix,feat,docs,ci,chore`. The adapter then preserves the remaining current behavior:

- Optional scope.
- Subject regex: `^[a-z].+$`.
- No WIP bypass.
- No `fixup!`, `squash!`, or `amend!` bypass.
- No single-commit message validation.

The task returns a non-zero status and a concise diagnostic for invalid input. Missing or malformed input also fails rather than being treated as valid. It must remove the temporary file on both success and failure.

## Workflows

### Replacement pre-commit workflow

Replace `.github/workflows/pre-commit.yml` with a mise-based workflow that:

1. Checks out the repository.
2. Installs the pinned mise tools.
3. Runs the read-only hk validation task over the complete checkout, skipping the Go-lint step to preserve the existing split with `golang-ci.yml`.
4. Does not run Python or pre-commit.
5. Does not mutate files.

### Replacement PR-title workflow

Replace `.github/workflows/pr-title.yml` with a mise-based workflow that:

1. Continues to use `pull_request_target`.
2. Continues to respond to `opened`, `edited`, and `synchronize`.
3. Retains only `pull-requests: read` permissions.
4. Installs the pinned mise tools.
5. Passes `github.event.pull_request.title` to `mise run pr-title`.
6. Preserves the current accepted types, optional scope, subject regex, WIP handling, and single-commit behavior exactly.

The task must execute repository-controlled validation logic from the trusted workflow revision rather than execute arbitrary pull-request code. The PR-title adapter must use hk’s utility for conventional-commit parsing rather than duplicate that parser.

### Unchanged workflows

`.github/workflows/golang-ci.yml` and `.github/workflows/goreleaser.yml` remain functionally unchanged.

## Failure and Error Behavior

- All validation tasks return non-zero on failure.
- Read-only CI checks never rewrite files.
- Local fixer checks rewrite only files covered by their documented fixer behavior.
- A failed task identifies the failing check and affected paths where practical.
- `pr-title` explains the accepted title format without introducing additional rules.
- Tool installation failures surface from mise without being masked by wrapper scripts.

## Files Changed

### Added

- `mise.toml`
- `hk.pkl`
- `.mise/tasks/check-vcs-permalinks`
- `.mise/tasks/forbid-new-submodules`
- `.mise/tasks/trailing-whitespace`
- `.mise/tasks/pr-title`

Additional `.mise/tasks/` files may be added only when needed to preserve exact check parity.

### Updated

- `.github/workflows/pre-commit.yml`
- `.github/workflows/pr-title.yml`
- `.renovaterc.json`
- `README.md`

### Removed

- `Makefile`
- `.pre-commit-config.yaml`

### Intentionally unchanged

- `.github/workflows/golang-ci.yml`
- `.github/workflows/goreleaser.yml`
- `.commitlintrc.yaml` remains absent.

### Renovate configuration

Remove stale pre-commit-specific configuration, including the pre-commit manager and `:enablePreCommit` behavior. Retain GitHub Actions update management and unrelated Renovate settings.

## README Updates

Document:

- Installing or activating mise.
- Running `mise install`.
- The canonical commands: `mise run build`, `mise run test`, `mise run lint`, and `mise run check`.
- Installing local hooks with `mise run hooks:install`.
- Running PR-title validation locally with `mise run pr-title -- "<title>"`.
- That hk replaces pre-commit for local hooks.
- That no commit-message policy is introduced because `.commitlintrc.yaml` is absent.

Remove references to Make targets and `pre-commit install`.

## Validation Plan

Before implementation is considered ready:

1. Confirm the diff contains only intended files.
2. Run `mise install` and verify all declared tools resolve to pinned versions.
3. Run `mise run build`.
4. Run `mise run test`.
5. Run `mise run lint`.
6. Run `mise run check` and verify it is read-only.
7. Exercise local hk hooks against fixtures for each mapped check, including YAML formatter fixer behavior and read-only YAML lint behavior.
8. Exercise `pr-title` with valid and invalid examples covering every allowed type, optional scopes, uppercase subjects, empty subjects, and unsupported types.
9. Validate both replacement workflow files as YAML.
10. Confirm the PR-title workflow retains its original event types and permissions.
11. Confirm the `golang-ci` and GoReleaser workflows are functionally unchanged.
12. Confirm Renovate still manages GitHub Actions while no longer managing pre-commit configuration.

## Risks and Open Decisions

- The exact hk release must be selected and pinned during implementation.
- The exact Go patch-level pin must remain compatible with `go.mod`.
- hk’s native capabilities and fixer flags must be verified before choosing built-in mappings; unsupported checks require dedicated mise tasks.
- The PR-title adapter must account for hk’s temporary-file interface and documented autosquash-prefix bypass behavior while preserving the existing workflow’s stricter no-bypass behavior.
- PR-title parity must be tested carefully because the existing validation drives release-note generation.
- `pull_request_target` remains security-sensitive. The replacement workflow must avoid executing untrusted pull-request code or checking out an attacker-controlled revision for validation logic.
- Removing the commitlint hook removes the only current reference to the absent `.commitlintrc.yaml`; this is intentional and does not authorize adding a new commit-message policy.
- Keeping `golang-ci` unchanged means it remains an explicit exception to the otherwise mise-centered command layer until a separate migration is approved.
