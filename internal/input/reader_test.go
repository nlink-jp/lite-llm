package input

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadUserInput_Direct(t *testing.T) {
	result, err := ReadUserInput("hello world", "")
	if err != nil {
		t.Fatalf("ReadUserInput() error: %v", err)
	}
	if result.Text != "hello world" {
		t.Errorf("Text = %q, want %q", result.Text, "hello world")
	}
	if result.Source != SourceDirect {
		t.Errorf("Source = %v, want SourceDirect", result.Source)
	}
}

func TestReadUserInput_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(path, []byte("file content"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := ReadUserInput("", path)
	if err != nil {
		t.Fatalf("ReadUserInput() error: %v", err)
	}
	if result.Text != "file content" {
		t.Errorf("Text = %q, want %q", result.Text, "file content")
	}
	if result.Source != SourceExternal {
		t.Errorf("Source = %v, want SourceExternal", result.Source)
	}
}

func TestReadUserInput_FileMissing(t *testing.T) {
	_, err := ReadUserInput("", "/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestReadUserInput_DirectTakesPriorityOverFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	_ = os.WriteFile(path, []byte("file content"), 0600)

	result, err := ReadUserInput("direct value", path)
	if err != nil {
		t.Fatalf("ReadUserInput() error: %v", err)
	}
	if result.Text != "direct value" {
		t.Errorf("Text = %q, want %q", result.Text, "direct value")
	}
	if result.Source != SourceDirect {
		t.Errorf("Source = %v, want SourceDirect", result.Source)
	}
}

func TestReadSystemPrompt_Direct(t *testing.T) {
	got, err := ReadSystemPrompt("be helpful", "")
	if err != nil {
		t.Fatalf("ReadSystemPrompt() error: %v", err)
	}
	if got != "be helpful" {
		t.Errorf("got %q, want %q", got, "be helpful")
	}
}

func TestReadSystemPrompt_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sys.txt")
	_ = os.WriteFile(path, []byte("system instructions"), 0600)

	got, err := ReadSystemPrompt("", path)
	if err != nil {
		t.Fatalf("ReadSystemPrompt() error: %v", err)
	}
	if got != "system instructions" {
		t.Errorf("got %q, want %q", got, "system instructions")
	}
}

func TestReadSystemPrompt_StdinForbidden(t *testing.T) {
	_, err := ReadSystemPrompt("", "-")
	if err == nil {
		t.Fatal("expected error when reading system prompt from stdin, got nil")
	}
}

func TestSanitizeUTF8_Valid(t *testing.T) {
	s := "hello, 世界"
	got := sanitizeUTF8(s)
	if got != s {
		t.Errorf("sanitizeUTF8(%q) = %q, want unchanged", s, got)
	}
}

func TestSanitizeUTF8_Invalid(t *testing.T) {
	// Construct a string with an invalid UTF-8 byte sequence.
	invalid := "hello\x80world"
	got := sanitizeUTF8(invalid)
	if strings.Contains(got, "\x80") {
		t.Errorf("sanitizeUTF8 should replace invalid bytes, got: %q", got)
	}
	if !strings.Contains(got, "\uFFFD") {
		t.Errorf("sanitizeUTF8 should insert replacement char, got: %q", got)
	}
}

func TestReadLines(t *testing.T) {
	input := "line1\nline2\n\nline3\n"
	r := strings.NewReader(input)
	lines, err := ReadLines(r)
	if err != nil {
		t.Fatalf("ReadLines() error: %v", err)
	}
	want := []string{"line1", "line2", "line3"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("lines[%d] = %q, want %q", i, lines[i], w)
		}
	}
}

func TestReadLines_SkipsEmpty(t *testing.T) {
	input := "\n\n\n"
	r := strings.NewReader(input)
	lines, err := ReadLines(r)
	if err != nil {
		t.Fatalf("ReadLines() error: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d: %v", len(lines), lines)
	}
}

func TestReadLines_CRLF(t *testing.T) {
	input := "line1\r\nline2\r\n"
	r := strings.NewReader(input)
	lines, err := ReadLines(r)
	if err != nil {
		t.Fatalf("ReadLines() error: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "line1" || lines[1] != "line2" {
		t.Errorf("unexpected lines: %v", lines)
	}
}
