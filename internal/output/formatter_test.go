package output

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseMode(t *testing.T) {
	tests := []struct {
		input   string
		want    Mode
		wantErr bool
	}{
		{"", ModeText, false},
		{"text", ModeText, false},
		{"json", ModeJSON, false},
		{"jsonl", ModeJSONL, false},
		{"xml", ModeText, true},
		{"JSON", ModeText, true},
	}
	for _, tt := range tests {
		got, err := ParseMode(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseMode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseMode(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestWrite_Text(t *testing.T) {
	var buf strings.Builder
	f := New(&buf, ModeText)
	if err := f.Write("hello world"); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	got := buf.String()
	if got != "hello world\n" {
		t.Errorf("got %q, want %q", got, "hello world\n")
	}
}

func TestWrite_JSON_ValidJSON(t *testing.T) {
	var buf strings.Builder
	f := New(&buf, ModeJSON)
	if err := f.Write(`{"key":"value","num":42}`); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	output := buf.String()
	// Should be valid JSON.
	var v interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &v); err != nil {
		t.Errorf("output is not valid JSON: %v\noutput: %q", err, output)
	}
	// Should be pretty-printed (contain newlines).
	if !strings.Contains(output, "\n") {
		t.Errorf("expected pretty-printed JSON, got: %q", output)
	}
}

func TestWrite_JSON_PlainText(t *testing.T) {
	var buf strings.Builder
	f := New(&buf, ModeJSON)
	if err := f.Write("plain text response"); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	output := strings.TrimSpace(buf.String())
	// Should be a JSON string.
	var s string
	if err := json.Unmarshal([]byte(output), &s); err != nil {
		t.Errorf("expected JSON string, got parse error: %v\noutput: %q", err, output)
	}
	if s != "plain text response" {
		t.Errorf("decoded = %q, want %q", s, "plain text response")
	}
}

func TestWrite_JSONLReturnsError(t *testing.T) {
	var buf strings.Builder
	f := New(&buf, ModeJSONL)
	err := f.Write("something")
	if err == nil {
		t.Error("Write() with ModeJSONL should return an error")
	}
}

func TestWriteJSONL_Success(t *testing.T) {
	var buf strings.Builder
	f := New(&buf, ModeJSONL)
	if err := f.WriteJSONL("input line", "response text", ""); err != nil {
		t.Fatalf("WriteJSONL() error: %v", err)
	}
	line := strings.TrimSpace(buf.String())

	type record struct {
		Input  string  `json:"input"`
		Output *string `json:"output"`
		Error  *string `json:"error"`
	}
	var r record
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, line)
	}
	if r.Input != "input line" {
		t.Errorf("Input = %q, want %q", r.Input, "input line")
	}
	if r.Output == nil || *r.Output != "response text" {
		t.Errorf("Output = %v, want %q", r.Output, "response text")
	}
	if r.Error != nil {
		t.Errorf("Error = %v, want nil", r.Error)
	}
}

func TestWriteJSONL_WithError(t *testing.T) {
	var buf strings.Builder
	f := New(&buf, ModeJSONL)
	if err := f.WriteJSONL("input line", "", "something went wrong"); err != nil {
		t.Fatalf("WriteJSONL() error: %v", err)
	}
	line := strings.TrimSpace(buf.String())

	type record struct {
		Input  string  `json:"input"`
		Output *string `json:"output"`
		Error  *string `json:"error"`
	}
	var r record
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, line)
	}
	if r.Error == nil || *r.Error != "something went wrong" {
		t.Errorf("Error = %v, want %q", r.Error, "something went wrong")
	}
	if r.Output != nil {
		t.Errorf("Output should be nil for error records, got %v", r.Output)
	}
}

func TestWriteJSONL_MultipleLines(t *testing.T) {
	var buf strings.Builder
	f := New(&buf, ModeJSONL)
	f.WriteJSONL("a", "resp a", "")
	f.WriteJSONL("b", "", "err b")
	f.WriteJSONL("c", "resp c", "")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	for _, line := range lines {
		var v interface{}
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Errorf("line is not valid JSON: %q", line)
		}
	}
}

func TestWriteText_Streaming(t *testing.T) {
	var buf strings.Builder
	f := New(&buf, ModeText)
	f.WriteText("hello")
	f.WriteText(" ")
	f.WriteText("world")
	f.Newline()
	if buf.String() != "hello world\n" {
		t.Errorf("got %q, want %q", buf.String(), "hello world\n")
	}
}
