package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	// Use a non-existent path to fall through to defaults only.
	cfg, err := Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.Endpoint != defaultEndpoint {
		t.Errorf("Endpoint = %q, want %q", cfg.Endpoint, defaultEndpoint)
	}
	if cfg.Model != defaultModel {
		t.Errorf("Model = %q, want %q", cfg.Model, defaultModel)
	}
	if cfg.TimeoutSeconds != defaultTimeoutSeconds {
		t.Errorf("TimeoutSeconds = %d, want %d", cfg.TimeoutSeconds, defaultTimeoutSeconds)
	}
	if cfg.ResponseFormatStrategy != defaultResponseFormatStrategy {
		t.Errorf("ResponseFormatStrategy = %q, want %q", cfg.ResponseFormatStrategy, defaultResponseFormatStrategy)
	}
	if cfg.APIKey != "" {
		t.Errorf("APIKey = %q, want empty string", cfg.APIKey)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
endpoint = "http://localhost:1234"
model = "llama3"
api_key = "test-key"
timeout_seconds = 30
response_format_strategy = "native"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.Endpoint != "http://localhost:1234" {
		t.Errorf("Endpoint = %q, want %q", cfg.Endpoint, "http://localhost:1234")
	}
	if cfg.Model != "llama3" {
		t.Errorf("Model = %q, want %q", cfg.Model, "llama3")
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "test-key")
	}
	if cfg.TimeoutSeconds != 30 {
		t.Errorf("TimeoutSeconds = %d, want 30", cfg.TimeoutSeconds)
	}
	if cfg.ResponseFormatStrategy != "native" {
		t.Errorf("ResponseFormatStrategy = %q, want %q", cfg.ResponseFormatStrategy, "native")
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("LITE_LLM_API_KEY", "env-key")
	t.Setenv("LITE_LLM_ENDPOINT", "http://env-endpoint")
	t.Setenv("LITE_LLM_MODEL", "env-model")

	cfg, err := Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "env-key")
	}
	if cfg.Endpoint != "http://env-endpoint" {
		t.Errorf("Endpoint = %q, want %q", cfg.Endpoint, "http://env-endpoint")
	}
	if cfg.Model != "env-model" {
		t.Errorf("Model = %q, want %q", cfg.Model, "env-model")
	}
}

func TestEnvOverridesFileValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
endpoint = "http://file-endpoint"
model = "file-model"
api_key = "file-key"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("LITE_LLM_MODEL", "env-model")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	// env var should win over file value
	if cfg.Model != "env-model" {
		t.Errorf("Model = %q, want %q", cfg.Model, "env-model")
	}
	// file value should remain for non-overridden keys
	if cfg.Endpoint != "http://file-endpoint" {
		t.Errorf("Endpoint = %q, want %q", cfg.Endpoint, "http://file-endpoint")
	}
}

func TestInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("not valid toml :::"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("Load() expected error for invalid TOML, got nil")
	}
}

func TestResolvePath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
		want  string
	}{
		{"~/foo/bar", filepath.Join(home, "foo/bar")},
		{"/abs/path", "/abs/path"},
	}
	for _, tt := range tests {
		got, err := ResolvePath(tt.input)
		if err != nil {
			t.Errorf("ResolvePath(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ResolvePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLoad_WarnsOnInsecurePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`model = "gpt-4o-mini"`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var warnBuf strings.Builder
	orig := Stderr
	Stderr = &warnBuf
	defer func() { Stderr = orig }()

	if _, err := Load(path); err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if !strings.Contains(warnBuf.String(), "Warning:") {
		t.Errorf("expected permission warning for 0644 file, got: %q", warnBuf.String())
	}
	if !strings.Contains(warnBuf.String(), "chmod 600") {
		t.Errorf("warning should suggest chmod 600, got: %q", warnBuf.String())
	}
}

func TestLoad_NoWarnOnSecurePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`model = "gpt-4o-mini"`), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var warnBuf strings.Builder
	orig := Stderr
	Stderr = &warnBuf
	defer func() { Stderr = orig }()

	if _, err := Load(path); err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if warnBuf.Len() != 0 {
		t.Errorf("expected no warning for 0600 file, got: %q", warnBuf.String())
	}
}

func TestCheckPermissions_Modes(t *testing.T) {
	tests := []struct {
		perm     os.FileMode
		wantWarn bool
	}{
		{0600, false},
		{0644, true},  // world-readable
		{0640, true},  // group-readable
		{0660, true},  // group-writable
		{0400, false}, // read-only by owner
		{0700, false}, // owner rwx only
	}
	for _, tt := range tests {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		_ = os.WriteFile(path, []byte(""), tt.perm)
		info, _ := os.Stat(path)

		var buf strings.Builder
		orig := Stderr
		Stderr = &buf
		checkPermissions(path, info)
		Stderr = orig

		gotWarn := buf.Len() > 0
		if gotWarn != tt.wantWarn {
			t.Errorf("perm %04o: wantWarn=%v gotWarn=%v (output: %q)", tt.perm, tt.wantWarn, gotWarn, buf.String())
		}
	}
}
