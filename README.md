# tgo

A minimalist command-line task manager, ported from [sjl/t](https://github.com/sjl/t) to Go.
File-format compatible with the original `t`.

## Install

```bash
go install github.com/nathanhruby/tgo@latest
```

## Usage

```bash
# Add a task
tgo add "Clean the apartment"
tgo add "Buy more beer"

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
