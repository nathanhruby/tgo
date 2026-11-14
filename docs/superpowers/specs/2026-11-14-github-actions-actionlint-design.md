# GitHub Actions Linting with actionlint

**Date:** 2026-11-14
**Status:** Approved

## Goal

Check GitHub Actions workflows with `actionlint` instead of generic YAML formatting and style tools.

## Behavior

- `mise.toml` pins `actionlint` from the Aqua registry (`aqua:rhysd/actionlint`) and no longer pins `yamlfmt` or `yamllint`.
- `hk.pkl` maps a single read-only `actionlint` step using `Builtins.actionlint`. The step applies to `.github/workflows/*.yml` and `.github/workflows/*.yaml` and runs in both the `pre-commit` and `check` hooks.
- YAML outside `.github/workflows/` is no longer formatted or linted by repository tooling. GitHub Actions awareness replaces the previous broad YAML policy because the repository's only YAML is Actions workflows, GoReleaser config, and tool configuration.
- `.yamlfmt` is removed with the tool. Stale `yamllint` suppression comments are removed from `.goreleaser.yaml`.

## Constraints

- The check stays read-only: `actionlint` has no fix command, so local hooks and CI behave identically.
- `mise run check` and `mise run check:ci` continue to run every configured step.
- The unused `tests/fix-wave-tests.sh` manual fixtures are removed; no mise task, hook, or workflow invoked them.

## Verification

1. `mise install` installs `actionlint` at the pinned version.
2. `mise run check` runs the `actionlint` step against every workflow file.
3. `mise run test`, `mise run lint`, and `mise run check` pass.
