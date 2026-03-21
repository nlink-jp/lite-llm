// Package input handles reading user prompts from files, stdin, and direct values,
// including UTF-8 sanitization.
package input

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// Source indicates where an input string originated.
type Source int

const (
	// SourceDirect means the value came from a CLI flag or positional argument (trusted).
	SourceDirect Source = iota
	// SourceExternal means the value came from a file or stdin (untrusted data).
	SourceExternal
)

// ReadResult holds the loaded prompt text and its origin.
type ReadResult struct {
	Text   string
	Source Source
}

// ReadUserInput loads the user prompt using the following priority:
//  1. directValue (from --prompt flag or positional arg) — SourceDirect
//  2. filePath (from --file flag) — SourceExternal; use "-" for stdin
//  3. stdin pipe (auto-detected) — SourceExternal
//
// Returns an empty ReadResult if no input is available.
func ReadUserInput(directValue, filePath string) (ReadResult, error) {
	if directValue != "" {
		return ReadResult{Text: sanitizeUTF8(directValue), Source: SourceDirect}, nil
	}
	if filePath != "" {
		text, err := readFile(filePath)
		if err != nil {
			return ReadResult{}, err
		}
		return ReadResult{Text: text, Source: SourceExternal}, nil
	}
	// Auto-detect stdin pipe.
	if isPiped(os.Stdin) {
		text, err := readReader(os.Stdin, "stdin")
		if err != nil {
			return ReadResult{}, err
		}
		return ReadResult{Text: text, Source: SourceExternal}, nil
	}
	return ReadResult{}, nil
}

// ReadSystemPrompt loads the system prompt from a direct value or a file.
// Reading from stdin is not allowed for system prompts.
func ReadSystemPrompt(directValue, filePath string) (string, error) {
	if directValue != "" {
		return sanitizeUTF8(directValue), nil
	}
	if filePath != "" {
		if filePath == "-" {
			return "", fmt.Errorf("reading system prompt from stdin is not allowed")
		}
		return readFile(filePath)
	}
	return "", nil
}

// readFile reads content from a named file or stdin ("-").
func readFile(filePath string) (string, error) {
	if filePath == "-" {
		return readReader(os.Stdin, "stdin")
	}
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening file %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()
	return readReader(f, filePath)
}

// readReader drains r and returns its content as a sanitized UTF-8 string.
func readReader(r io.Reader, name string) (string, error) {
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return "", fmt.Errorf("error reading %s: %w", name, err)
	}
	return sanitizeUTF8(buf.String()), nil
}

// isPiped returns true when r is connected to a pipe or redirected file.
func isPiped(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// sanitizeUTF8 replaces invalid UTF-8 sequences with the replacement character.
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "\uFFFD")
}

// ReadLines splits text into non-empty trimmed lines.
// Used as input for batch processing.
func ReadLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line != "" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading lines: %w", err)
	}
	return lines, nil
}
