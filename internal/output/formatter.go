// Package output handles formatting and writing LLM responses to an io.Writer.
package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// Mode controls the output format.
type Mode int

const (
	// ModeText writes the response as plain text followed by a newline.
	ModeText Mode = iota
	// ModeJSON pretty-prints the response as a JSON value.
	// If the response is already a valid JSON object or array it is re-indented;
	// otherwise it is emitted as a JSON string.
	ModeJSON
	// ModeJSONL emits one compact JSON object per line in the format:
	//   {"input":"...","output":"...","error":null}
	// Intended for batch mode.
	ModeJSONL
)

// ParseMode converts a format string to a Mode.
// Valid values are "", "json", and "jsonl". An empty string yields ModeText.
func ParseMode(s string) (Mode, error) {
	switch s {
	case "", "text":
		return ModeText, nil
	case "json":
		return ModeJSON, nil
	case "jsonl":
		return ModeJSONL, nil
	default:
		return ModeText, fmt.Errorf("unknown output format %q: must be one of text, json, jsonl", s)
	}
}

// Formatter writes responses to w according to the configured Mode.
type Formatter struct {
	mode Mode
	w    io.Writer
}

// New creates a new Formatter.
func New(w io.Writer, mode Mode) *Formatter {
	return &Formatter{mode: mode, w: w}
}

// WriteText appends the response to the stream (used during streaming).
// For ModeText it writes the token directly. For other modes it buffers nothing
// and should not be used; use Write instead.
func (f *Formatter) WriteText(token string) error {
	_, err := fmt.Fprint(f.w, token)
	return err
}

// Newline writes a trailing newline. Used after a streaming text response.
func (f *Formatter) Newline() error {
	_, err := fmt.Fprintln(f.w)
	return err
}

// Write formats and writes a complete response.
//
// For ModeText the response is written as-is with a trailing newline.
// For ModeJSON the response is formatted as pretty-printed JSON.
// For ModeJSONL this method must not be called directly; use WriteJSONL instead.
func (f *Formatter) Write(response string) error {
	switch f.mode {
	case ModeText:
		_, err := fmt.Fprintln(f.w, response)
		return err
	case ModeJSON:
		return f.writeJSON(response)
	default:
		return fmt.Errorf("Write() is not valid for ModeJSONL; use WriteJSONL instead")
	}
}

// WriteJSONL writes a single batch result as a JSON Lines record.
// If errMsg is non-empty the record carries an error; otherwise output holds the result.
func (f *Formatter) WriteJSONL(input, output, errMsg string) error {
	type record struct {
		Input  string  `json:"input"`
		Output *string `json:"output"`
		Error  *string `json:"error"`
	}
	r := record{Input: input}
	if errMsg != "" {
		r.Error = &errMsg
	} else {
		r.Output = &output
	}
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("error encoding JSONL record: %w", err)
	}
	_, err = fmt.Fprintln(f.w, string(b))
	return err
}

// writeJSON tries to pretty-print the response as a JSON object/array.
// If the response is not valid JSON it falls back to encoding it as a JSON string.
func (f *Formatter) writeJSON(response string) error {
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(response), &raw); err == nil {
		// Valid JSON: re-indent and write.
		pretty, err := json.MarshalIndent(raw, "", "  ")
		if err != nil {
			return fmt.Errorf("error formatting JSON: %w", err)
		}
		_, err = fmt.Fprintln(f.w, string(pretty))
		return err
	}
	// Not valid JSON: encode as a JSON string.
	b, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error encoding response as JSON string: %w", err)
	}
	_, err = fmt.Fprintln(f.w, string(b))
	return err
}
