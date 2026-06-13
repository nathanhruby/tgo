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

func TestTaskFromLine_StandardFormat(t *testing.T) {
	task, ok, err := taskFromLine("Buy more beer | id:abc123")
	if err != nil || !ok {
		t.Fatalf("unexpected: ok=%v err=%v", ok, err)
	}
	if task.ID != "abc123" || task.Text != "Buy more beer" {
		t.Errorf("got %+v", task)
	}
}

func TestTaskFromLine_BareText(t *testing.T) {
	task, ok, err := taskFromLine("Buy more beer")
	if err != nil || !ok {
		t.Fatalf("unexpected: ok=%v err=%v", ok, err)
	}
	want := hashText("Buy more beer")
	if task.ID != want || task.Text != "Buy more beer" {
		t.Errorf("got %+v, want ID=%s", task, want)
	}
}

func TestTaskFromLine_Comment(t *testing.T) {
	_, ok, err := taskFromLine("# this is a comment")
	if err != nil || ok {
		t.Errorf("expected comment to be skipped, got ok=%v err=%v", ok, err)
	}
}

func TestTaskFromLine_Empty(t *testing.T) {
	_, ok, err := taskFromLine("")
	if err != nil || ok {
		t.Errorf("expected empty to be skipped, got ok=%v err=%v", ok, err)
	}
}

func TestTaskFromLine_Whitespace(t *testing.T) {
	_, ok, err := taskFromLine("   ")
	if err != nil || ok {
		t.Errorf("expected whitespace-only to be skipped")
	}
}

func TestTaskToLine(t *testing.T) {
	task := Task{ID: "abc123", Text: "Buy more beer"}
	got := taskToLine(task)
	want := "Buy more beer | id:abc123\n"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestTaskFromLine_PipeInText(t *testing.T) {
	task, ok, err := taskFromLine("Buy A | B stuff | id:abc456")
	if err != nil || !ok {
		t.Fatalf("unexpected: ok=%v err=%v", ok, err)
	}
	if task.ID != "abc456" || task.Text != "Buy A | B stuff" {
		t.Errorf("got ID=%q Text=%q, want ID='abc456' Text='Buy A | B stuff'", task.ID, task.Text)
	}
}

func TestTaskRoundTrip(t *testing.T) {
	original := Task{ID: hashText("Clean the apartment"), Text: "Clean the apartment"}
	line := taskToLine(original)
	got, ok, err := taskFromLine(line[:len(line)-1]) // strip trailing newline
	if err != nil || !ok {
		t.Fatalf("round trip failed: ok=%v err=%v", ok, err)
	}
	if got != original {
		t.Errorf("want %v, got %v", original, got)
	}
}
