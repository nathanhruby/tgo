package main

import (
	"crypto/sha1"
	"fmt"
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
