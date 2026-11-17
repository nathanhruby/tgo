# Renovate jdx Group and Biweekly Wednesday Schedule

**Date:** 2026-11-17
**Status:** Approved

## Goal

Group every Renovate update from the `jdx` GitHub organization into one pull request, create those pull requests every other Wednesday, and let them automerge on the same day.

## Behavior

- A `packageRules` entry matches `jdx/**` (GitHub Actions such as `jdx/mise-action`), `github.com/jdx/**` (Go modules), and `hk` (the `jdx/hk` mise tool, whose Renovate package name is `hk`). It sets `groupName` to `jdx` and sets `separateMajorMinor` to `false` so major updates join the same group.
- Both `schedule` and `automergeSchedule` use `* * 8-14,22-28 * 3` with the existing `America/Los_Angeles` timezone. Renovate requires both the day of month and day of week to match, so this fires on the second and fourth Wednesday of each month.
- Existing automerge policy still applies: minor, patch, pin, and digest updates automerge, while major updates request review.
- `platformAutomerge` stays `true`. Renovate only creates these pull requests on the scheduled Wednesday, so the platform automerge is enqueued that day.

## Constraints

- Renovate cron cannot express an exact rolling 14-day cadence. The second and fourth Wednesday pattern is the stable equivalent; month boundaries can produce a three-week gap.
- The deprecated `@breejs/later` syntax (`every 2 weeks on Wednesday`) is not used because it is scheduled for removal and its week-of-year recurrence fires on consecutive Wednesdays at the year boundary.

## Verification

1. `renovate-config-validator .renovaterc.json` accepts the configuration.
2. The cron yields only second and fourth Wednesdays in `America/Los_Angeles`.
3. `mise run test`, `mise run lint`, and `mise run check` pass.
