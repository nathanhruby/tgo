package main

import (
	"errors"
	"testing"
)

func TestErrAmbiguousPrefix(t *testing.T) {
	err := &ErrAmbiguousPrefix{Prefix: "abc"}
	var target *ErrAmbiguousPrefix
	if !errors.As(err, &target) {
		t.Fatal("errors.As failed for ErrAmbiguousPrefix")
	}
	if target.Prefix != "abc" {
		t.Errorf("want prefix 'abc', got %q", target.Prefix)
	}
}

func TestErrUnknownPrefix(t *testing.T) {
	err := &ErrUnknownPrefix{Prefix: "xyz"}
	var target *ErrUnknownPrefix
	if !errors.As(err, &target) {
		t.Fatal("errors.As failed for ErrUnknownPrefix")
	}
	if target.Prefix != "xyz" {
		t.Errorf("want prefix 'xyz', got %q", target.Prefix)
	}
}

func TestErrInvalidTaskFile(t *testing.T) {
	err := &ErrInvalidTaskFile{Path: "/some/path"}
	var target *ErrInvalidTaskFile
	if !errors.As(err, &target) {
		t.Fatal("errors.As failed for ErrInvalidTaskFile")
	}
	if target.Path != "/some/path" {
		t.Errorf("want path '/some/path', got %q", target.Path)
	}
}

func TestErrBadFile(t *testing.T) {
	err := &ErrBadFile{Path: "/some/path", Problem: "permission denied"}
	var target *ErrBadFile
	if !errors.As(err, &target) {
		t.Fatal("errors.As failed for ErrBadFile")
	}
	if target.Path != "/some/path" || target.Problem != "permission denied" {
		t.Errorf("want path='/some/path' problem='permission denied', got path=%q problem=%q", target.Path, target.Problem)
	}
}

func TestHashText(t *testing.T) {
	// SHA1 of "hello" is aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d
	got := hashText("hello")
	want := "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"
	if got != want {
		t.Errorf("hashText(\"hello\") = %q, want %q", got, want)
	}
}
