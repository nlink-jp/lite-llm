package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- loadJSONSchemaFile ---

func TestLoadJSONSchemaFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "person.json")
	schema := `{"type":"object","properties":{"name":{"type":"string"}}}`
	if err := os.WriteFile(path, []byte(schema), 0600); err != nil {
		t.Fatal(err)
	}

	rf, err := loadJSONSchemaFile(path)
	if err != nil {
		t.Fatalf("loadJSONSchemaFile() error: %v", err)
	}
	if rf.Type != "json_schema" {
		t.Errorf("Type = %q, want json_schema", rf.Type)
	}
	if rf.SchemaName != "person" {
		t.Errorf("SchemaName = %q, want person", rf.SchemaName)
	}
	var raw interface{}
	if err := json.Unmarshal(rf.Schema, &raw); err != nil {
		t.Errorf("Schema is not valid JSON: %v", err)
	}
}

func TestLoadJSONSchemaFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := loadJSONSchemaFile(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON schema file")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("error should mention invalid JSON, got: %v", err)
	}
}

func TestLoadJSONSchemaFile_Missing(t *testing.T) {
	_, err := loadJSONSchemaFile("/nonexistent/schema.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadJSONSchemaFile_SchemaNameFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		wantName string
	}{
		{"user.json", "user"},
		{"my_schema.json", "my_schema"},
		{"schema.v2.json", "schema.v2"},
	}
	for _, tt := range tests {
		dir := t.TempDir()
		path := filepath.Join(dir, tt.filename)
		_ = os.WriteFile(path, []byte(`{}`), 0600)

		rf, err := loadJSONSchemaFile(path)
		if err != nil {
			t.Errorf("loadJSONSchemaFile(%q) error: %v", tt.filename, err)
			continue
		}
		if rf.SchemaName != tt.wantName {
			t.Errorf("SchemaName = %q, want %q", rf.SchemaName, tt.wantName)
		}
	}
}

// --- truncate ---

func TestTruncate(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello…"},
		{"hello", 5, "hello"},
		{"hello", 0, "…"},
		{"日本語テスト文字列", 4, "日本語テ…"},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.s, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}

// --- command integration tests ---

// runCmd executes the root command with the given args against the provided test
// server and returns stdout, stderr, and any error.
func runCmd(t *testing.T, serverURL string, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var outBuf, errBuf bytes.Buffer

	// Create a fresh command for each test to avoid flag state leaking.
	cmd := newRootCmd()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Prepend flags that point at the test server.
	// Use a non-existent config path so tests are not affected by the user's real config file.
	fullArgs := append([]string{
		"--config", "/nonexistent/test-config.toml",
		"--endpoint", serverURL,
		"--model", "test-model",
	}, args...)
	cmd.SetArgs(fullArgs)

	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// mockServer returns a test server that replies with the given response text.
func mockServer(t *testing.T, responseText string) *httptest.Server {
	t.Helper()
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type resp struct {
		Choices []struct {
			Message msg `json:"message"`
		} `json:"choices"`
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp{
			Choices: []struct {
				Message msg `json:"message"`
			}{{Message: msg{Role: "assistant", Content: responseText}}},
		})
	}))
}

func TestCommand_BasicPrompt(t *testing.T) {
	srv := mockServer(t, "hello from model")
	defer srv.Close()

	stdout, _, err := runCmd(t, srv.URL, "--prompt", "say hello")
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(stdout, "hello from model") {
		t.Errorf("stdout = %q, want to contain %q", stdout, "hello from model")
	}
}

func TestCommand_PositionalArg(t *testing.T) {
	srv := mockServer(t, "positional response")
	defer srv.Close()

	stdout, _, err := runCmd(t, srv.URL, "my prompt text")
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(stdout, "positional response") {
		t.Errorf("stdout = %q, want to contain %q", stdout, "positional response")
	}
}

func TestCommand_FileInput(t *testing.T) {
	srv := mockServer(t, "file response")
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	_ = os.WriteFile(path, []byte("data from file"), 0600)

	stdout, _, err := runCmd(t, srv.URL, "--file", path, "--prompt", "summarize")
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(stdout, "file response") {
		t.Errorf("stdout = %q, want to contain %q", stdout, "file response")
	}
}

func TestCommand_NoInputError(t *testing.T) {
	srv := mockServer(t, "")
	defer srv.Close()

	_, _, err := runCmd(t, srv.URL)
	if err == nil {
		t.Fatal("expected error when no input provided")
	}
}

func TestCommand_MissingModelError(t *testing.T) {
	// Write a config that explicitly clears the model to trigger the validation error.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	_ = os.WriteFile(cfgPath, []byte("[model]\nname = \"\""), 0600)

	var outBuf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{
		"--config", cfgPath,
		"--endpoint", "http://localhost",
		"--prompt", "hello",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no model specified")
	}
	if !strings.Contains(err.Error(), "model") {
		t.Errorf("error should mention model, got: %v", err)
	}
}

func TestCommand_JSONFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify response_format was sent.
		var req struct {
			ResponseFormat *struct {
				Type string `json:"type"`
			} `json:"response_format"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		type msg struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		type resp struct {
			Choices []struct {
				Message msg `json:"message"`
			} `json:"choices"`
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp{
			Choices: []struct {
				Message msg `json:"message"`
			}{{Message: msg{Role: "assistant", Content: `{"key":"value"}`}}},
		})
	}))
	defer srv.Close()

	stdout, _, err := runCmd(t, srv.URL, "--format", "json", "--prompt", "give me JSON")
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	// Output should be pretty-printed JSON.
	var v interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &v); err != nil {
		t.Errorf("stdout is not valid JSON: %v\nstdout: %q", err, stdout)
	}
}

func TestCommand_JSONSchemaFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ResponseFormat *struct {
				Type       string `json:"type"`
				JSONSchema *struct {
					Name string `json:"name"`
				} `json:"json_schema"`
			} `json:"response_format"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_schema" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, `{"error":{"message":"expected json_schema"}}`)
			return
		}

		type msg struct{ Role, Content string }
		type resp struct {
			Choices []struct{ Message msg `json:"message"` } `json:"choices"`
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp{Choices: []struct {
			Message msg `json:"message"`
		}{{Message: msg{Role: "assistant", Content: `{"name":"Alice"}`}}}})
	}))
	defer srv.Close()

	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "person.json")
	_ = os.WriteFile(schemaPath, []byte(`{"type":"object","properties":{"name":{"type":"string"}}}`), 0600)

	stdout, _, err := runCmd(t, srv.URL, "--json-schema", schemaPath, "--prompt", "generate")
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(stdout, "Alice") {
		t.Errorf("stdout = %q, want to contain Alice", stdout)
	}
}

func TestCommand_BatchJSONL(t *testing.T) {
	callCount := 0
	responses := []string{"positive", "negative", "neutral"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := responses[callCount%len(responses)]
		callCount++
		type msg struct{ Role, Content string }
		type respBody struct {
			Choices []struct{ Message msg `json:"message"` } `json:"choices"`
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(respBody{Choices: []struct {
			Message msg `json:"message"`
		}{{Message: msg{Role: "assistant", Content: resp}}}})
	}))
	defer srv.Close()

	// Write batch input to a temp file.
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.txt")
	_ = os.WriteFile(inputPath, []byte("great\nbad\nokay\n"), 0600)

	stdout, _, err := runCmd(t, srv.URL,
		"--batch", "--format", "jsonl",
		"--file", inputPath,
		"--system-prompt", "classify sentiment",
	)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 JSONL lines, got %d:\n%s", len(lines), stdout)
	}
	for i, line := range lines {
		var rec struct {
			Input  string  `json:"input"`
			Output *string `json:"output"`
			Error  *string `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
		if rec.Error != nil {
			t.Errorf("line %d has unexpected error: %s", i, *rec.Error)
		}
	}
	if callCount != 3 {
		t.Errorf("expected 3 API calls, got %d", callCount)
	}
}

func TestCommand_StreamBatchMutuallyExclusive(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"--endpoint", "http://localhost",
		"--model", "m",
		"--stream", "--batch",
		"--prompt", "hello",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --stream + --batch")
	}
}

func TestCommand_FormatJSONLWithoutBatchErrors(t *testing.T) {
	srv := mockServer(t, "response")
	defer srv.Close()

	_, _, err := runCmd(t, srv.URL, "--format", "jsonl", "--prompt", "hello")
	if err == nil {
		t.Fatal("expected error for --format jsonl without --batch")
	}
	if !strings.Contains(err.Error(), "batch") {
		t.Errorf("error should mention batch, got: %v", err)
	}
}

func TestCommand_QuietSuppressesWarnings(t *testing.T) {
	// Server that rejects response_format (triggers the fallback warning).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ResponseFormat *struct{ Type string `json:"type"` } `json:"response_format"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req.ResponseFormat != nil {
			// First call: reject to trigger fallback warning.
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, `{"error":{"message":"response_format not supported"}}`)
			return
		}
		type msg struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		type resp struct {
			Choices []struct {
				Message msg `json:"message"`
			} `json:"choices"`
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp{
			Choices: []struct {
				Message msg `json:"message"`
			}{{Message: msg{Role: "assistant", Content: `{"ok":true}`}}},
		})
	}))
	defer srv.Close()

	// Without --quiet: warning should appear on stderr.
	_, stderrOut, err := runCmd(t, srv.URL, "--format", "json", "--prompt", "hello")
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(stderrOut, "Warning:") {
		t.Errorf("expected warning on stderr without --quiet, got: %q", stderrOut)
	}

	// With --quiet: stderr should be empty.
	_, stderrQuiet, err := runCmd(t, srv.URL, "--format", "json", "--quiet", "--prompt", "hello")
	if err != nil {
		t.Fatalf("Execute() error with --quiet: %v", err)
	}
	if stderrQuiet != "" {
		t.Errorf("expected no stderr output with --quiet, got: %q", stderrQuiet)
	}
}
