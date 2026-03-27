// Package cmd implements the lite-llm CLI.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/nlink-jp/lite-llm/internal/client"
	"github.com/nlink-jp/lite-llm/internal/config"
	"github.com/nlink-jp/lite-llm/internal/input"
	"github.com/nlink-jp/lite-llm/internal/isolation"
	"github.com/nlink-jp/lite-llm/internal/output"
)

// version is set at build time via ldflags.
var version = "dev"

// Execute is the entry point called from main.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "lite-llm [flags] [prompt]",
		Version: version,
		Short:   "Lightweight OpenAI-compatible LLM CLI",
		Long: `lite-llm is a minimal CLI for interacting with OpenAI-compatible LLM APIs.

Key features:
  - Data isolation: stdin and file inputs are treated as data, not instructions
    (automatic prompt-injection protection, enabled by default).
  - Batch mode: process input line-by-line, one request per line.
  - Structured output: request JSON or schema-constrained responses.`,
		SilenceUsage: true,
		RunE:         run,
	}

	f := cmd.Flags()

	// Input
	f.StringP("prompt", "p", "", "User prompt text")
	f.StringP("file", "f", "", "Input file path (use - for stdin)")
	f.StringP("system-prompt", "s", "", "System prompt text")
	f.StringP("system-prompt-file", "S", "", "System prompt file path")

	// Model / endpoint
	f.StringP("model", "m", "", "Model name (overrides config)")
	f.String("endpoint", "", "API base URL (overrides config)")

	// Execution mode
	f.Bool("stream", false, "Enable streaming output (incompatible with --batch)")
	f.Bool("batch", false, "Batch mode: process input line-by-line, one request per line")

	// Output format
	f.String("format", "", "Output format: text (default), json, jsonl")
	f.String("json-schema", "", "JSON Schema file for structured output (implies --format json)")

	// Security
	f.Bool("no-safe-input", false, "Disable automatic data isolation for stdin/file inputs")

	// Output control
	f.BoolP("quiet", "q", false, "Suppress warning and informational messages on stderr")
	f.Bool("debug", false, "Log API request and response bodies to stderr")

	// Config
	f.StringP("config", "c", "", "Config file path")

	// Constraints
	cmd.MarkFlagsMutuallyExclusive("stream", "batch")
	cmd.MarkFlagsMutuallyExclusive("format", "json-schema")

	return cmd
}

func run(cmd *cobra.Command, args []string) error {
	f := cmd.Flags()

	// --- Stderr routing ---
	// Route warning output to cmd.ErrOrStderr() so that tests can capture it
	// via cmd.SetErr() and --quiet can suppress it cleanly.
	quiet, _ := f.GetBool("quiet")
	if quiet {
		config.Stderr = io.Discard
		client.SetStderr(io.Discard)
	} else {
		config.Stderr = cmd.ErrOrStderr()
		client.SetStderr(cmd.ErrOrStderr())
	}

	debug, _ := f.GetBool("debug")
	if debug {
		client.SetDebug(cmd.ErrOrStderr())
	} else {
		client.SetDebug(nil)
	}

	// --- Config ---
	cfgFile, _ := f.GetString("config")
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if m, _ := f.GetString("model"); m != "" {
		cfg.Model.Name = m
	}
	if e, _ := f.GetString("endpoint"); e != "" {
		cfg.API.BaseURL = e
	}
	if cfg.Model.Name == "" {
		return fmt.Errorf("no model specified: set model in config file, LITE_LLM_MODEL env var, or --model flag")
	}

	// --- System prompt ---
	sysPromptText, _ := f.GetString("system-prompt")
	sysPromptFile, _ := f.GetString("system-prompt-file")
	systemPrompt, err := input.ReadSystemPrompt(sysPromptText, sysPromptFile)
	if err != nil {
		return fmt.Errorf("system prompt: %w", err)
	}

	// --- User input ---
	// In batch mode stdin/file is consumed line-by-line inside runBatch,
	// so we must not read it here to avoid exhausting the stream early.
	promptText, _ := f.GetString("prompt")
	if len(args) > 0 && promptText == "" {
		promptText = strings.Join(args, " ")
	}
	fileFlag, _ := f.GetString("file")
	batchMode, _ := f.GetBool("batch")
	var userResult input.ReadResult
	if !batchMode {
		userResult, err = input.ReadUserInput(promptText, fileFlag)
		if err != nil {
			return fmt.Errorf("user input: %w", err)
		}
	}

	// --- Output format ---
	formatStr, _ := f.GetString("format")
	jsonSchemaFile, _ := f.GetString("json-schema")

	var responseFormat *client.ResponseFormat
	if jsonSchemaFile != "" {
		rf, err := loadJSONSchemaFile(jsonSchemaFile)
		if err != nil {
			return fmt.Errorf("json-schema: %w", err)
		}
		responseFormat = rf
		// json-schema implies json output format.
		formatStr = "json"
	}
	if formatStr == "json" && responseFormat == nil {
		responseFormat = &client.ResponseFormat{Type: "json_object"}
	}

	outMode, err := output.ParseMode(formatStr)
	if err != nil {
		return err
	}

	// --format jsonl requires --batch
	stream, _ := f.GetBool("stream")
	if outMode == output.ModeJSONL && !batchMode {
		return fmt.Errorf("--format jsonl requires --batch")
	}
	// --json-schema is incompatible with --stream (structured output requires non-streaming)
	if jsonSchemaFile != "" && stream {
		return fmt.Errorf("--json-schema cannot be combined with --stream")
	}

	// --- Safety ---
	noSafeInput, _ := f.GetBool("no-safe-input")
	safeInput := !noSafeInput

	// --- Signal handling ---
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// --- Client ---
	c := client.New(cfg)
	formatter := output.New(cmd.OutOrStdout(), outMode)

	// --- Dispatch ---
	if batchMode {
		return runBatch(ctx, cmd, c, cfg, systemPrompt, userResult, safeInput, outMode, responseFormat, formatter)
	}
	return runSingle(ctx, c, systemPrompt, userResult, safeInput, stream, responseFormat, formatter)
}

// runSingle handles a single-request execution (streaming or blocking).
func runSingle(
	ctx context.Context,
	c *client.Client,
	systemPrompt string,
	userResult input.ReadResult,
	safeInput bool,
	stream bool,
	responseFormat *client.ResponseFormat,
	formatter *output.Formatter,
) error {
	if userResult.Text == "" {
		return fmt.Errorf("no input provided: pass a prompt as an argument, via --prompt, --file, or stdin")
	}

	wrappedUser, wrappedSystem := isolation.WrapInput(
		userResult.Text,
		systemPrompt,
		userResult.Source == input.SourceExternal,
		safeInput,
	)

	opts := client.ChatOptions{
		SystemPrompt:   wrappedSystem,
		UserPrompt:     wrappedUser,
		ResponseFormat: responseFormat,
	}

	if stream {
		responseChan := make(chan string, 64)
		errChan := make(chan error, 1)
		go func() {
			errChan <- c.ChatStream(ctx, opts, responseChan)
			close(responseChan)
		}()
		for token := range responseChan {
			if err := formatter.WriteText(token); err != nil {
				return err
			}
		}
		if err := <-errChan; err != nil {
			return err
		}
		return formatter.Newline()
	}

	resp, err := c.Chat(ctx, opts)
	if err != nil {
		return err
	}
	return formatter.Write(resp)
}

// runBatch handles batch (line-by-line) execution.
func runBatch(
	ctx context.Context,
	cmd *cobra.Command,
	c *client.Client,
	_ *config.Config,
	systemPrompt string,
	userResult input.ReadResult,
	safeInput bool,
	outMode output.Mode,
	responseFormat *client.ResponseFormat,
	formatter *output.Formatter,
) error {
	// Determine the batch input source.
	fileFlag, _ := cmd.Flags().GetString("file")
	var lines []string
	var err error

	if fileFlag != "" && fileFlag != "-" {
		// Read from the named file.
		f, err := os.Open(fileFlag)
		if err != nil {
			return fmt.Errorf("error opening batch file: %w", err)
		}
		defer func() { _ = f.Close() }()
		lines, err = input.ReadLines(f)
		if err != nil {
			return err
		}
	} else {
		// Read from stdin.
		lines, err = input.ReadLines(os.Stdin)
		if err != nil {
			return err
		}
	}

	if len(lines) == 0 {
		return fmt.Errorf("batch mode: no input lines found")
	}

	hasError := false
	for _, line := range lines {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		wrappedUser, wrappedSystem := isolation.WrapInput(
			line,
			systemPrompt,
			true, // batch input is always external
			safeInput,
		)

		opts := client.ChatOptions{
			SystemPrompt:   wrappedSystem,
			UserPrompt:     wrappedUser,
			ResponseFormat: responseFormat,
		}

		resp, err := c.Chat(ctx, opts)
		if err != nil {
			hasError = true
			if outMode == output.ModeJSONL {
				_ = formatter.WriteJSONL(line, "", err.Error())
			} else {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error processing line %q: %v\n", truncate(line, 60), err)
			}
			continue
		}

		if outMode == output.ModeJSONL {
			if writeErr := formatter.WriteJSONL(line, resp, ""); writeErr != nil {
				return writeErr
			}
		} else {
			if writeErr := formatter.Write(resp); writeErr != nil {
				return writeErr
			}
		}
	}

	if hasError {
		return fmt.Errorf("one or more batch lines failed (see stderr for details)")
	}
	return nil
}

// loadJSONSchemaFile reads a JSON Schema from path and returns a ResponseFormat.
func loadJSONSchemaFile(path string) (*client.ResponseFormat, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading schema file %q: %w", path, err)
	}
	// Validate that the file contains valid JSON.
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("schema file %q is not valid JSON: %w", path, err)
	}
	// Derive schema name from filename without extension.
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return &client.ResponseFormat{
		Type:       "json_schema",
		SchemaName: name,
		Schema:     raw,
	}, nil
}

// truncate shortens s to at most n runes, appending "…" if truncated.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
