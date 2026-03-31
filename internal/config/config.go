// Package config handles loading and merging of application configuration
// from a TOML file and environment variables.
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

// Stderr is the writer used for permission warnings; can be overridden in tests.
var Stderr io.Writer = os.Stderr

const (
	defaultBaseURL                = "https://api.openai.com"
	defaultModel                  = "gpt-4o-mini"
	defaultTimeoutSeconds         = 120
	defaultResponseFormatStrategy = "auto"
)

// Config holds all application configuration.
// Values are resolved in order: CLI flags > environment variables > config file > defaults.
type Config struct {
	API   APIConfig   `toml:"api"`
	Model ModelConfig `toml:"model"`
}

// APIConfig holds connection and request settings for the LLM API.
type APIConfig struct {
	BaseURL                string `toml:"base_url"`
	APIKey                 string `toml:"api_key"`
	TimeoutSeconds         int    `toml:"timeout_seconds"`
	ResponseFormatStrategy string `toml:"response_format_strategy"`
}

// ModelConfig specifies which model to use.
type ModelConfig struct {
	Name string `toml:"name"`
}

// Load reads configuration from configPath (or the default path when empty),
// then applies environment variable overrides.
func Load(configPath string) (*Config, error) {
	cfg := defaults()

	path := configPath
	if path == "" {
		var err error
		path, err = DefaultConfigPath()
		if err != nil {
			return nil, err
		}
	}

	if info, err := os.Stat(path); err == nil {
		checkPermissions(path, info)
		if _, err := toml.DecodeFile(path, cfg); err != nil {
			return nil, fmt.Errorf("error reading config file %s: %w", path, err)
		}
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

func defaults() *Config {
	return &Config{
		API: APIConfig{
			BaseURL:                defaultBaseURL,
			TimeoutSeconds:         defaultTimeoutSeconds,
			ResponseFormatStrategy: defaultResponseFormatStrategy,
		},
		Model: ModelConfig{
			Name: defaultModel,
		},
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("LITE_LLM_API_KEY"); v != "" {
		cfg.API.APIKey = v
	}
	if v := os.Getenv("LITE_LLM_BASE_URL"); v != "" {
		cfg.API.BaseURL = v
	}
	if v := os.Getenv("LITE_LLM_MODEL"); v != "" {
		cfg.Model.Name = v
	}
}

// checkPermissions warns if the config file is accessible by group or others.
// Config files may contain API keys and should be readable only by the owner (0600).
func checkPermissions(path string, info os.FileInfo) {
	// Windows NTFS does not support Unix permission bits; os.FileInfo.Mode()
	// always reports 0666, which would trigger a false-positive warning.
	if runtime.GOOS == "windows" {
		return
	}
	perm := info.Mode().Perm()
	if perm&0077 != 0 {
		_, _ = fmt.Fprintf(Stderr,
			"Warning: config file %s has permissions %04o; expected 0600.\n"+
				"  The file may contain an API key. Run: chmod 600 %s\n",
			path, perm, path,
		)
	}
}

// DefaultConfigPath returns the default configuration file path.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "lite-llm", "config.toml"), nil
}

// ResolvePath expands a leading ~ to the user's home directory and returns an absolute path.
func ResolvePath(p string) (string, error) {
	if len(p) > 0 && p[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		p = filepath.Join(home, p[1:])
	}
	return filepath.Abs(p)
}
