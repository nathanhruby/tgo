# AGENTS.md

Instructions for agents working in this repository.

## Required workflow

1. **Read the design before changing behavior.** Find the relevant spec and plan under `docs/superpowers/specs/` and `docs/superpowers/plans/`. If the change has no spec, write a short one first.
   Done when: you can name the file and the behavior it specifies.
2. **Write the failing test first.** Add or update a `_test.go` test that exercises the missing behavior through the real code path. Run `mise run test` and confirm it fails for the intended reason, not an unrelated compile error.
   Done when: the new test is red and its failure message names the missing behavior.
3. **Implement the smallest change that passes.** Prefer the standard library and existing helpers. Do not add an abstraction until a second caller needs it.
   Done when: the new test passes and the full suite still passes.
4. **Run every check.** Run `mise run test`, `mise run lint`, and `mise run check`.
   Done when: all three pass. If a tool is unavailable, say so explicitly instead of claiming it passed.
5. **Commit with a conventional commit.** Commit each coherent change after the checks pass.
   Done when: `mise run pr-title -- "<your subject>"` accepts the message.

## Non-negotiable rules

### Tests come first

Every behavior change starts with a test that fails before the implementation. Bug fixes include a regression test that reproduces the bug. Test the observable behavior, not just fixture text: drive the code through its real entry points and assert what a user would see.

### Keep tests honest

Keep every existing test running. Fix a failing test by correcting the production code or the test's premise. Never delete a test, call `t.Skip`, loosen an assertion, add a build tag, or hide a failure to make the suite green. If a test is genuinely wrong, update it in the same commit and explain why in the body. Never add test-only branches to production code.

### Write simple, idiomatic Go

- Standard library first; use the existing helpers in this package before adding new ones.
- Small functions with one job and explicit error returns. Wrap errors with context.
- Use value receivers for Bubble Tea models.
- Follow `gofmt` output and `.golangci.yml`; do not add `//nolint` to work around a real problem.
- Keep exported identifiers documented and unexported helpers close to their use.
- No new dependency without a concrete need already exercised by the code.

### Use conventional commits

Format: `<type>(<scope>): <subject>`

- Types: `fix`, `feat`, `docs`, `ci`, `chore`. Scope is optional.
- Subject: lowercase, imperative, no trailing period.
- Examples: `feat: add watch command`, `fix: retain output on refresh error`, `docs: add agent guidelines`, `chore(deps): update golangci-lint`.
- PR titles use the same format; `.mise/tasks/pr-title` validates them.

## Repository map

- `main.go` — CLI wiring: commands, flags, actions, and error formatting.
- `tasks.go` — task model, file format, ID prefixes, reading and writing lists.
- `watch.go` — Bubble Tea watch model, timer sequencing, and rendering.
- `*_test.go` — package-level tests next to the code they cover.
- `docs/superpowers/specs/` and `docs/superpowers/plans/` — approved designs and implementation plans.
- `tests/fix-wave-tests.sh` — shell-level end-to-end checks.
- `.mise/tasks/pr-title` — conventional commit and PR title validator.

## Commands

| Task | Command |
|------|---------|
| Install pinned tools | `mise install` |
| Build | `mise run build` |
| Test | `mise run test` |
| Lint | `mise run lint` |
| All repository checks | `mise run check` |
| Install git hooks | `mise run hooks:install` |
| Validate a commit/PR title | `mise run pr-title -- "feat: add export"` |

Tool versions come from `mise.toml`. Do not hardcode alternate versions.

## Testing conventions

- Use the standard `testing` package; there is no assertion library.
- Prefer table-driven tests when cases share setup and assertions.
- Use `t.TempDir()` for task files; never write into the repository.
- Inject clocks, schedulers, and other time sources. Tests must not sleep or wait in real time.
- Assert exact output where formatting matters, and computed expected output over hard-coded IDs or prefixes.
- Run `go test ./... -race` when touching concurrent code.
