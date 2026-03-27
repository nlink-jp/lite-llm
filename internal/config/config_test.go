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
	if cfg.API.BaseURL != defaultBaseURL {
		t.Errorf("API.BaseURL = %q, want %q", cfg.API.BaseURL, defaultBaseURL)
	}
	if cfg.Model.Name != defaultModel {
		t.Errorf("Model.Name = %q, want %q", cfg.Model.Name, defaultModel)
	}
	if cfg.API.TimeoutSeconds != defaultTimeoutSeconds {
		t.Errorf("API.TimeoutSeconds = %d, want %d", cfg.API.TimeoutSeconds, defaultTimeoutSeconds)
	}
	if cfg.API.ResponseFormatStrategy != defaultResponseFormatStrategy {
		t.Errorf("API.ResponseFormatStrategy = %q, want %q", cfg.API.ResponseFormatStrategy, defaultResponseFormatStrategy)
	}
	if cfg.API.APIKey != "" {
		t.Errorf("API.APIKey = %q, want empty string", cfg.API.APIKey)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[api]
base_url = "http://localhost:1234"
api_key  = "test-key"
timeout_seconds = 30
response_format_strategy = "native"

[model]
name = "llama3"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.API.BaseURL != "http://localhost:1234" {
		t.Errorf("API.BaseURL = %q, want %q", cfg.API.BaseURL, "http://localhost:1234")
	}
	if cfg.Model.Name != "llama3" {
		t.Errorf("Model.Name = %q, want %q", cfg.Model.Name, "llama3")
	}
	if cfg.API.APIKey != "test-key" {
		t.Errorf("API.APIKey = %q, want %q", cfg.API.APIKey, "test-key")
	}
	if cfg.API.TimeoutSeconds != 30 {
		t.Errorf("API.TimeoutSeconds = %d, want 30", cfg.API.TimeoutSeconds)
	}
	if cfg.API.ResponseFormatStrategy != "native" {
		t.Errorf("API.ResponseFormatStrategy = %q, want %q", cfg.API.ResponseFormatStrategy, "native")
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("LITE_LLM_API_KEY", "env-key")
	t.Setenv("LITE_LLM_BASE_URL", "http://env-endpoint")
	t.Setenv("LITE_LLM_MODEL", "env-model")

	cfg, err := Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.API.APIKey != "env-key" {
		t.Errorf("API.APIKey = %q, want %q", cfg.API.APIKey, "env-key")
	}
	if cfg.API.BaseURL != "http://env-endpoint" {
		t.Errorf("API.BaseURL = %q, want %q", cfg.API.BaseURL, "http://env-endpoint")
	}
	if cfg.Model.Name != "env-model" {
		t.Errorf("Model.Name = %q, want %q", cfg.Model.Name, "env-model")
	}
}

func TestEnvOverridesFileValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[api]
base_url = "http://file-endpoint"
api_key  = "file-key"

[model]
name = "file-model"
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
	if cfg.Model.Name != "env-model" {
		t.Errorf("Model.Name = %q, want %q", cfg.Model.Name, "env-model")
	}
	// file value should remain for non-overridden keys
	if cfg.API.BaseURL != "http://file-endpoint" {
		t.Errorf("API.BaseURL = %q, want %q", cfg.API.BaseURL, "http://file-endpoint")
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
	if err := os.WriteFile(path, []byte("[model]\nname = \"gpt-4o-mini\""), 0644); err != nil {
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
	if err := os.WriteFile(path, []byte("[model]\nname = \"gpt-4o-mini\""), 0600); err != nil {
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
