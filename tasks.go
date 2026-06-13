package main

import (
	"crypto/sha1"
	"fmt"
	"strings"
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
	h := sha1.New()
	h.Write([]byte(text))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Task represents a single task entry.
type Task struct {
	ID   string
	Text string
}

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
			parts := strings.SplitN(piece, ":", 2)
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

// prefixes returns a mapping of id -> shortest unique prefix for each id in O(n·k) time,
// where k is the maximum ID length (constant 40 for SHA1 hex strings).
// Ported from sjl/t's _prefixes function. All IDs should be uniform length (SHA1 hex).
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

		if otherID, found := ps[prefix]; found {
			// collision: walk forward until they diverge
			broke := false
			for j := i; j <= idLen; j++ {
				if otherID[:j] == id[:j] {
					ps[id[:j]] = ""
				} else {
					ps[otherID[:j]] = otherID
					ps[id[:j]] = id
					broke = true
					break
				}
			}
			if !broke {
				// Identical IDs — skip (SHA1 of distinct texts cannot collide in practice)
				if otherID != id {
					if idLen+1 <= len(otherID) {
						ps[otherID[:idLen+1]] = otherID
					}
					ps[id] = id
				}
			}
		} else {
			ps[prefix] = id
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
