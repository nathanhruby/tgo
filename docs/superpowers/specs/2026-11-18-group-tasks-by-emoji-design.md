# Group Tasks by Leading Emoji

**Date:** 2026-11-18
**Status:** Approved
**Amends in part:** `2026-11-15-watch-lipgloss-design.md` (its list/watch parity anti-goal
applies only while grouping is off; grouping adds headings and indentation to the watch body).

## Intent

Task text is free-form, so lists that mix errands, work, and hobbies read as one undifferentiated
column. A leading emoji already works as a home-made tag. This change makes that convention
visible: with `--group`, tasks that start with emoji are collected under their emoji, and the
emoji groups are ordered lexicographically. `tgo list` prints a heading per group; `tgo watch`
renders the same grouping as a Lipgloss nested list, so the dashboard gains hierarchy for free.

Grouping is display-only. The file format, stored text, IDs, and every mutating subcommand stay
exactly as they are.

## Goals

- Add `--group` (`-G`) to `list`, `done`, and `watch`.
- Treat the maximal run of emoji grapheme clusters at the start of the text as the group key.
  A space, an ASCII character, or any non-emoji cluster ends the run.
- Order groups lexicographically by the emoji key, with ungrouped tasks (`""`) last. Keep ID
  order inside a group.
- `list`: print the emoji on its own line, its rows indented two spaces, one blank line between
  blocks, and no heading for ungrouped tasks.
- `watch`: render headings flush left with each group's tasks as an indented, bulleted sublist;
  ungrouped tasks stay flat at column 0 as they are today.
- With `--group` off, output is byte-identical to today.

## Non-Goals

- Persisting groups, reordering task files, or changing the file format.
- Sorting rows by text inside a group; ID order stays.
- Configurable headings, indentation, or separators; group selection or filtering in watch.
- Group headings in `add`, `finish`, `remove`, or `edit` output.
- A complete Unicode emoji implementation; a curated range table plus variation-selector support
  is enough.

## Anti-Goals

- A heading that is visible without any of its rows.
- A truncation boundary that ends on a heading.
- Changing what `-g` (grep) means.
- Mutating stored task text, or changing IDs when grouping is toggled.
- Watch/list drift beyond the documented differences: bullets, indentation, and `list`'s blank
  lines between blocks.

## Constraints

- `-g` is already `--grep` (`main.go`), so grouping takes `-G`. The long flag is `--group`.
- `github.com/rivo/uniseg` is already an indirect dependency; this change promotes it to direct.
- `taskRow` is the shared row source. Tests must keep asserting real rendered output, not only
  the helper's return values.
- Tests must not sleep, read the wall clock, or depend on terminal width beyond what is injected.

## Approaches Considered

Two forks were laid out and chosen.

**Where groups live: flat rows, renderers detect boundaries (chosen).** `taskRow` gains a `Group`
field, and `taskRows` takes a `group bool`. Both `List` and watch walk the flat slice and start a
new block when `Group` changes. This is the smaller diff and keeps one row builder. The cost is
that the heading rule and the truncation rule are implemented in two renderers; both are covered
by exact-output tests.

Rejected: returning `[]taskGroup` from a new `groupedTaskRows` (cleaner block iteration, but a
second row builder next to `taskRows` for a feature that is only a display convention), and
grouping separately inside each renderer from the raw task map (duplicated emoji scanning and
ordering, and exactly the drift the watch parity goal forbids).

**Emoji detection: grapheme clusters with `uniseg` (chosen).** Iterate clusters with
`uniseg.StepString`, accumulating while the cluster starts with a non-ASCII rune and contains an
emoji-range code point or U+FE0F. Flags, skin tones, ZWJ families, and multi-emoji runs stay
whole; a keycap (`1️⃣`) starts with ASCII and is correctly not a group.

Rejected: a hand-rolled rune loop (no dependency change, but ZWJ, variation selectors, and skin
tones come apart so keys disagree with what the terminal draws), and a dedicated emoji-detection
library (a new module for one predicate, while the cluster loop is still needed).

## Design

### Group detection (`tasks.go`)

```go
// splitEmojiGroup splits text into its leading emoji run and the remaining display text.
// The remainder has one following space removed. A task that is only emoji keeps its full
// text as the remainder so the row is never blank.
func splitEmojiGroup(text string) (key, rest string)
```

- Walk `text` with `uniseg.StepString(text, state)`, starting with `state = -1`.
- A cluster continues the key when its first rune is non-ASCII and the cluster contains a rune in
  the emoji table or U+FE0F. Otherwise the run stops.
- The emoji table covers the pictograph/emoticon/transport/flag blocks (U+1F000–U+1FAFF,
  U+2600–U+27BF, U+2B00–U+2BFF) plus the default-emoji clock and media symbols in U+23xx.
  U+FE0F lets text-presentation symbols such as `❤️` and `©️` join. The table lives in one
  unexported function so it can be refined without touching callers.
- After the run, strip at most one leading ASCII space from the remainder.
- Invalid UTF-8 ends the run; such text is ungrouped rather than an error.
- `key == ""` means ungrouped.

### Rows and ordering (`tasks.go`)

```go
type taskRow struct {
    Label string
    Group string // leading emoji run; "" for ungrouped
    Text  string // display text with the emoji run removed
}

func taskRows(tasks map[string]Task, verbose, quiet, group bool, grep string) ([]taskRow, int)
```

- Labels and `labelWidth` are still computed across every task ID before filtering, so padding
  never shifts with grouping or grep.
- With `group == false` behavior and output are unchanged and `Group` stays empty.
- With `group == true`:
  - Compute each task's key once, then sort IDs by `(group key, id)`. Keys compare as strings;
    `""` sorts after every non-empty key. Inside a group, IDs ascend.
  - Grep matches the **stored** text, not the stripped display text, so `--grep 🍕` still finds
    its tasks.
  - Rows carry `Group` and the stripped `Text`. Quiet rows keep `Group` and an empty `Label`;
    verbose rows keep full IDs.
- A group whose rows are all filtered out simply produces no rows; renderers see no block for it.

### `List` output (`tasks.go`)

`List` gains a `group bool` before `grep`. With grouping on it walks the flat rows, and whenever
`row.Group` differs from the previous row:

- print one blank line, except before the first block;
- print the emoji heading on its own line, unless the key is `""`;
- remember the current key.

Every row whose key is non-empty is printed with a two-space indent (`  %-*s - %s`, or
`  %s` in quiet mode). Ungrouped rows print exactly as today, at column 0 with no heading.
A list with no emoji tasks therefore renders byte-identically to `--group` off.

Example:

```
🍕
  1a - Order pizza
  2b - Eat pizza

🚀
  3c - Launch

4d - No emoji
```

### Watch rendering (`watch.go`)

- `watchConfig` gains `group bool`; `watchConfigFromCommand` reads `cmd.Bool("group")`, and
  `load` passes it to `taskRows`.
- `renderBody` returns `[]watchLine`, where `watchLine{text string; heading bool}`. `screenLines`
  projects `.text` and uses `.heading` for truncation.
- Boundary detection mirrors `List`: consecutive rows with the same `Group` form a block.
- An emoji block renders as
  `list.New(key, list.New(items...)).Enumerator(blank).EnumeratorStyle(lipgloss.NewStyle())`:
  the heading is flush left and the task items inherit the default bullet and two-space indent.
  Its first rendered line is marked `heading: true`; the rest are task lines.
- The ungrouped block renders as today, `list.New(items...)` with `list.Bullet` at column 0.
- With grouping on but no emoji tasks, the body is exactly today's bullet list.
- Item text still uses `watchLabelStyle.Width(labelWidth)` plus `" - "` plus the row text, so
  labels align the same way inside every group. Tabs keep expanding to four spaces (the existing
  documented exception).
- The status bar and open count are unchanged.

### Height budget and truncation (`watch.go`)

The existing budget arithmetic stays, with headings counted as body lines:

- `N` in `… +N more` still counts hidden **task** rows only; headings are never counted.
- If the last line that fits is a heading, drop it, so the marker follows the previous task and a
  heading never appears without a row. Its group's tasks are already part of `N`.
- Error precedence is unchanged: error first, then body lines (including headings), then the
  marker, then the bar; at height 1 the error displaces the bar.
- Width truncation still runs per rendered line, so a long heading truncates like any other line.

### CLI and docs

- `listFlags()` adds `--group` / `-G`, usage `group tasks by leading emoji`. `list`, `done`, and
  `watch` all pick it up because they share those flags.
- `listAction` and `doneAction` pass `cmd.Bool("group")` to `List`.
- README usage gains a grouped `list` and `watch` example. Help text comes from the flag.
- The watch-lipgloss spec gets a one-line pointer to this document at its top, and this document
  records the amended parity rule.

## Consequences

- `taskRows` and `List` signatures change; every call site and test helper is updated. `taskRow`
  gains a field, but existing struct literals keep compiling.
- `go.mod` promotes `github.com/rivo/uniseg` from indirect to direct. No new modules.
- Grouped `list` output gains headings, indentation, and blank lines; ungrouped output is
  untouched. Grouped watch output stops being "bullets over list lines"; its tests are rewritten
  to the new contract rather than loosened.
- The watch truncation tests gain heading cases: no dangling heading, marker counts only tasks.
- No file-format or command-behavior change for `add`, `edit`, `finish`, or `remove`.

## Testing

Deliberately red before implementation:

- `splitEmojiGroup` table: no emoji; one emoji; two adjacent; space-separated; ASCII directly
  after the emoji; CJK directly after; ZWJ family; flag; skin tone; keycap stays ungrouped;
  emoji-only task; trailing space; `©️` with U+FE0F; non-ASCII letter such as `é`.
- `taskRows`: group order ascending with ungrouped last; ID order within a group; prefixes and
  padding unchanged across groups; verbose and quiet; grep on stored text (including an emoji
  query) with empty groups dropped; stripped text; emoji-only row text.
- `List`: exact grouped output (headings, two-space indent, blank line between blocks, no trailing
  blank line); grouped output with only ungrouped tasks equals ungrouped output byte for byte;
  quiet; verbose; grouped `done`; grouping plus grep.
- Watch, driving `Init`/`Update`/`View` with injected ticks: nested render shape; ungrouped rows
  flat; grouping off unchanged; grouping on with no emoji tasks unchanged; height budget with
  headings (trailing heading dropped, `N` counts only task rows, error precedence); width
  truncation of a heading; status bar and open count unaffected; `group` forwarded through load.
- CLI: `buildApp()` accepts `--group` and `-G` on `list`, `done`, and `watch`; `-g` still means
  grep.

Validation: `mise run test`, `mise run lint`, `mise run check`, per `AGENTS.md`. Work happens on
`nrh/groups` off `main` and lands through a pull request.

## Left to the Builder

- The exact emoji range table and its helper name; the rule above is the contract.
- Whether `splitEmojiGroup` returns the stripped display text or the raw remainder plus a flag.
- Whether `taskRows` precomputes keys in a map or sorts a small ID/key pair slice.
- The blank enumerator/style plumbing for the per-group nested list, and whether the ungrouped
  block and emoji blocks are rendered separately or joined into one body slice.
- Where `watchLine` lives and whether `renderBody` grows a second return value instead.
