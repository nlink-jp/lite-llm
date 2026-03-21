// Package client provides an OpenAI API-compatible HTTP client with support for
// streaming, structured output (response_format), and automatic fallback for
// APIs that do not implement the response_format field.
package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/magifd2/lite-llm/internal/config"
)

const chatCompletionsPath = "/v1/chat/completions"

// Client sends requests to an OpenAI-compatible Chat Completions endpoint.
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

// New creates a new Client using the provided configuration.
func New(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
		},
	}
}

// ChatOptions contains all parameters for a chat request.
type ChatOptions struct {
	SystemPrompt   string
	UserPrompt     string
	ResponseFormat *ResponseFormat // nil means plain text output
}

// Chat sends a single request and returns the full response text.
// When cfg.ResponseFormatStrategy is "auto" and the API rejects response_format,
// it retries with a prompt-injection fallback and logs a warning to stderr.
func (c *Client) Chat(ctx context.Context, opts ChatOptions) (string, error) {
	switch c.cfg.ResponseFormatStrategy {
	case "native":
		return c.chatOnce(ctx, opts, true)
	case "prompt":
		return c.chatOnce(ctx, injectFormatPrompt(opts), false)
	default: // "auto"
		resp, err := c.chatOnce(ctx, opts, true)
		if err != nil && isResponseFormatError(err) && opts.ResponseFormat != nil {
			_, _ = fmt.Fprintf(stderr, "Warning: response_format not supported by this API, falling back to prompt injection\n")
			return c.chatOnce(ctx, injectFormatPrompt(opts), false)
		}
		return resp, err
	}
}

// ChatStream sends a streaming request and writes response tokens to responseChan.
// The caller must not close responseChan; this function does not close it either.
func (c *Client) ChatStream(ctx context.Context, opts ChatOptions, responseChan chan<- string) error {
	msgs := buildMessages(opts.SystemPrompt, opts.UserPrompt)
	reqBody := chatRequest{
		Model:    c.cfg.Model,
		Messages: msgs,
		Stream:   true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("error marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return readStream(ctx, resp.Body, responseChan)
}

// chatOnce performs a single non-streaming request.
// sendFormat controls whether the response_format field is included.
func (c *Client) chatOnce(ctx context.Context, opts ChatOptions, sendFormat bool) (string, error) {
	msgs := buildMessages(opts.SystemPrompt, opts.UserPrompt)
	reqBody := chatRequest{
		Model:    c.cfg.Model,
		Messages: msgs,
	}
	if sendFormat && opts.ResponseFormat != nil {
		reqBody.ResponseFormat = buildResponseFormat(opts.ResponseFormat)
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", &apiResponseError{status: resp.StatusCode, body: string(body)}
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from API")
	}
	return chatResp.Choices[0].Message.Content, nil
}

func (c *Client) endpoint() string {
	base := strings.TrimRight(c.cfg.Endpoint, "/")
	// If the base URL already ends with /v1, append only /chat/completions.
	if strings.HasSuffix(base, "/v1") {
		return base + "/chat/completions"
	}
	return base + chatCompletionsPath
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
}

func buildMessages(systemPrompt, userPrompt string) []message {
	msgs := make([]message, 0, 2)
	if systemPrompt != "" {
		msgs = append(msgs, message{Role: "system", Content: systemPrompt})
	}
	msgs = append(msgs, message{Role: "user", Content: userPrompt})
	return msgs
}

func buildResponseFormat(rf *ResponseFormat) *responseFormat {
	if rf == nil {
		return nil
	}
	switch rf.Type {
	case "json_schema":
		return &responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchema{
				Name:   rf.SchemaName,
				Strict: true,
				Schema: rf.Schema,
			},
		}
	default: // "json_object"
		return &responseFormat{Type: "json_object"}
	}
}

// readStream processes an SSE stream and sends content deltas to responseChan.
func readStream(ctx context.Context, r io.Reader, responseChan chan<- string) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // skip malformed chunks
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		content := chunk.Choices[0].Delta.Content
		if content == "" {
			continue
		}
		select {
		case responseChan <- content:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return scanner.Err()
}

// injectFormatPrompt returns a copy of opts with JSON instructions appended
// to the system prompt. Used when the API does not support response_format.
func injectFormatPrompt(opts ChatOptions) ChatOptions {
	if opts.ResponseFormat == nil {
		return opts
	}
	var note string
	if opts.ResponseFormat.Type == "json_schema" && len(opts.ResponseFormat.Schema) > 0 {
		note = fmt.Sprintf(
			"Respond with a valid JSON object only, conforming to the following JSON Schema:\n%s\nDo not include any explanation or prose outside the JSON object.",
			string(opts.ResponseFormat.Schema),
		)
	} else {
		note = "Respond with a valid JSON object only. Do not include any explanation or prose."
	}

	sep := ""
	if opts.SystemPrompt != "" {
		sep = "\n\n"
	}
	return ChatOptions{
		SystemPrompt:   opts.SystemPrompt + sep + note,
		UserPrompt:     opts.UserPrompt,
		ResponseFormat: nil, // do not send response_format
	}
}

// apiResponseError carries the HTTP status and body of a failed API response.
type apiResponseError struct {
	status int
	body   string
}

func (e *apiResponseError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.status, e.body)
}

// isResponseFormatError returns true when the error likely indicates that the
// remote API does not support the response_format field.
func isResponseFormatError(err error) bool {
	if err == nil {
		return false
	}
	apiErr, ok := err.(*apiResponseError)
	if !ok {
		return false
	}
	if apiErr.status != http.StatusBadRequest && apiErr.status != http.StatusUnprocessableEntity {
		return false
	}
	lower := strings.ToLower(apiErr.body)
	return strings.Contains(lower, "response_format") ||
		strings.Contains(lower, "not supported") ||
		strings.Contains(lower, "unsupported") ||
		strings.Contains(lower, "unknown field")
}

// stderr is the writer used for warning messages; can be overridden in tests.
var stderr io.Writer = os.Stderr

// SetStderr redirects client warning output to w.
// Pass io.Discard to suppress all warnings (e.g. when --quiet is active).
func SetStderr(w io.Writer) { stderr = w }
