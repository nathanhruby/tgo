# tgo

A minimalist command-line task manager, ported from [sjl/t](https://github.com/sjl/t) to Go.
File-format compatible with the original `t`.

## Install

Using Go Tools

```bash
go install github.com/nathanhruby/tgo@latest
```

Using Homebrew
```bash
brew trust --formula nathanhruby/tap/tgo
brew install nathanhruby/tap/tgo
```

## Development

mise provides tool versions and tasks, while hk replaces pre-commit for local hooks.

```text
mise install
mise run build
mise run test
mise run lint
mise run check
mise run hooks:install
mise run pr-title -- "feat: add export"
```

The repository intentionally does not add a commit-message policy because `.commitlintrc.yaml` is absent.

## Usage

```bash
# Add a task
tgo add "Clean the apartment"
tgo add -m "Buy more beer"

# List open tasks (also: just run `tgo`)
tgo list

# Finish a task
tgo finish 9

# Edit a task
tgo edit 30 "Clean the entire apartment"

# Remove a task
tgo remove 9

# List finished tasks
tgo done

# Use a different list or directory
tgo --task-dir ~/tasks --list groceries add "Oat milk"
```

## Setup (alias)

Add to your shell config:

```bash
alias t='tgo --task-dir ~/tasks --list tasks'
```

## File Format

Compatible with `t`. Tasks are stored as plain text:

```
task text | id:<sha1hex>
```

- Open tasks: `<taskdir>/<listname>`
- Done tasks: `<taskdir>/.<listname>.done`
- Files are sorted by ID on write (VCS-friendly)
