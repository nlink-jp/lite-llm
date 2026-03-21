package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magifd2/lite-llm/internal/config"
)

// newTestClient creates a Client pointing at the given test server URL.
func newTestClient(serverURL string) *Client {
	cfg := &config.Config{
		Endpoint:               serverURL,
		Model:                  "test-model",
		TimeoutSeconds:         5,
		ResponseFormatStrategy: "native",
	}
	return New(cfg)
}

// mockChatHandler returns an HTTP handler that responds with a fixed chat completion.
func mockChatHandler(t *testing.T, wantContent string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		resp := chatResponse{
			Choices: []struct {
				Message message `json:"message"`
			}{
				{Message: message{Role: "assistant", Content: wantContent}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}
}

func TestChat_BasicResponse(t *testing.T) {
	srv := httptest.NewServer(mockChatHandler(t, "hello world"))
	defer srv.Close()

	c := newTestClient(srv.URL)
	got, err := c.Chat(context.Background(), ChatOptions{
		UserPrompt: "say hello",
	})
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if got != "hello world" {
		t.Errorf("Chat() = %q, want %q", got, "hello world")
	}
}

func TestChat_WithSystemPrompt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}
		if len(req.Messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("first message role = %q, want system", req.Messages[0].Role)
		}
		resp := chatResponse{Choices: []struct {
			Message message `json:"message"`
		}{{Message: message{Role: "assistant", Content: "ok"}}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Chat(context.Background(), ChatOptions{
		SystemPrompt: "you are a helper",
		UserPrompt:   "hello",
	})
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
}

func TestChat_ResponseFormatJSONObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		_ = json.Unmarshal(body, &req)
		if req.ResponseFormat == nil {
			t.Error("expected response_format, got nil")
		} else if req.ResponseFormat.Type != "json_object" {
			t.Errorf("response_format.type = %q, want json_object", req.ResponseFormat.Type)
		}
		resp := chatResponse{Choices: []struct {
			Message message `json:"message"`
		}{{Message: message{Role: "assistant", Content: `{"key":"val"}`}}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Chat(context.Background(), ChatOptions{
		UserPrompt:     "give me json",
		ResponseFormat: &ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
}

func TestChat_ResponseFormatJSONSchema(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		_ = json.Unmarshal(body, &req)
		if req.ResponseFormat == nil {
			t.Fatal("expected response_format, got nil")
		}
		if req.ResponseFormat.Type != "json_schema" {
			t.Errorf("type = %q, want json_schema", req.ResponseFormat.Type)
		}
		if req.ResponseFormat.JSONSchema == nil {
			t.Fatal("json_schema is nil")
		}
		if !req.ResponseFormat.JSONSchema.Strict {
			t.Error("strict should be true")
		}
		if req.ResponseFormat.JSONSchema.Name != "myschema" {
			t.Errorf("name = %q, want myschema", req.ResponseFormat.JSONSchema.Name)
		}
		resp := chatResponse{Choices: []struct {
			Message message `json:"message"`
		}{{Message: message{Role: "assistant", Content: `{"name":"test"}`}}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Chat(context.Background(), ChatOptions{
		UserPrompt: "generate",
		ResponseFormat: &ResponseFormat{
			Type:       "json_schema",
			SchemaName: "myschema",
			Schema:     schema,
		},
	})
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
}

func TestChat_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Chat(context.Background(), ChatOptions{UserPrompt: "hello"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestChat_AutoFallback(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		_ = json.Unmarshal(body, &req)

		if callCount == 1 {
			// First call: reject response_format
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, `{"error":{"message":"response_format not supported"}}`)
			return
		}
		// Second call (fallback): should have no response_format, JSON note in system prompt
		if req.ResponseFormat != nil {
			t.Error("fallback request should not include response_format")
		}
		found := false
		for _, m := range req.Messages {
			if m.Role == "system" && strings.Contains(m.Content, "JSON") {
				found = true
			}
		}
		if !found {
			t.Error("fallback system prompt should contain JSON instruction")
		}
		resp := chatResponse{Choices: []struct {
			Message message `json:"message"`
		}{{Message: message{Role: "assistant", Content: `{"ok":true}`}}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Endpoint:               srv.URL,
		Model:                  "test-model",
		TimeoutSeconds:         5,
		ResponseFormatStrategy: "auto",
	}
	c := New(cfg)

	// Redirect warning output in test
	var warnBuf strings.Builder
	stderr = &warnBuf

	got, err := c.Chat(context.Background(), ChatOptions{
		UserPrompt:     "give json",
		ResponseFormat: &ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if got != `{"ok":true}` {
		t.Errorf("Chat() = %q, want %q", got, `{"ok":true}`)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
	if !strings.Contains(warnBuf.String(), "Warning:") {
		t.Error("expected warning message on stderr")
	}
}

func TestChat_PromptStrategy_NeverSendsResponseFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		_ = json.Unmarshal(body, &req)
		if req.ResponseFormat != nil {
			t.Error("prompt strategy should not send response_format")
		}
		resp := chatResponse{Choices: []struct {
			Message message `json:"message"`
		}{{Message: message{Role: "assistant", Content: "{}"}}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Endpoint:               srv.URL,
		Model:                  "test-model",
		TimeoutSeconds:         5,
		ResponseFormatStrategy: "prompt",
	}
	c := New(cfg)
	_, err := c.Chat(context.Background(), ChatOptions{
		UserPrompt:     "hello",
		ResponseFormat: &ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
}

func TestChatStream_Basic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{"hello", " ", "world"}
		for _, c := range chunks {
			chunk := streamChunk{Choices: []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			}{{Delta: struct {
				Content string `json:"content"`
			}{Content: c}}}}
			data, _ := json.Marshal(chunk)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	ch := make(chan string, 10)
	var result strings.Builder

	err := c.ChatStream(context.Background(), ChatOptions{UserPrompt: "hello"}, ch)
	if err != nil {
		t.Fatalf("ChatStream() error: %v", err)
	}
	close(ch)
	for tok := range ch {
		result.WriteString(tok)
	}
	if result.String() != "hello world" {
		t.Errorf("stream result = %q, want %q", result.String(), "hello world")
	}
}

func TestEndpoint_WithV1Suffix(t *testing.T) {
	tests := []struct {
		endpoint string
		want     string
	}{
		{"http://localhost:1234/v1", "http://localhost:1234/v1/chat/completions"},
		{"http://localhost:1234/v1/", "http://localhost:1234/v1/chat/completions"},
		{"https://api.openai.com", "https://api.openai.com/v1/chat/completions"},
		{"https://api.openai.com/", "https://api.openai.com/v1/chat/completions"},
	}
	for _, tt := range tests {
		c := newTestClient(tt.endpoint)
		c.cfg.Endpoint = tt.endpoint
		got := c.endpoint()
		if got != tt.want {
			t.Errorf("endpoint(%q) = %q, want %q", tt.endpoint, got, tt.want)
		}
	}
}

func TestIsResponseFormatError(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{fmt.Errorf("some other error"), false},
		{&apiResponseError{status: 400, body: "response_format not supported"}, true},
		{&apiResponseError{status: 400, body: "unknown field response_format"}, true},
		{&apiResponseError{status: 422, body: "unsupported parameter"}, true},
		{&apiResponseError{status: 500, body: "response_format error"}, false}, // 500 is not a client error
		{&apiResponseError{status: 400, body: "invalid model"}, false},
	}
	for _, tt := range tests {
		got := isResponseFormatError(tt.err)
		if got != tt.want {
			t.Errorf("isResponseFormatError(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

func TestInjectFormatPrompt_JSONObject(t *testing.T) {
	opts := ChatOptions{
		SystemPrompt:   "you are a helper",
		UserPrompt:     "list fruits",
		ResponseFormat: &ResponseFormat{Type: "json_object"},
	}
	got := injectFormatPrompt(opts)
	if got.ResponseFormat != nil {
		t.Error("ResponseFormat should be nil after injection")
	}
	if !strings.Contains(got.SystemPrompt, "JSON") {
		t.Error("injected system prompt should contain JSON instruction")
	}
	if !strings.Contains(got.SystemPrompt, "you are a helper") {
		t.Error("original system prompt should be preserved")
	}
}

func TestInjectFormatPrompt_JSONSchema(t *testing.T) {
	schema := json.RawMessage(`{"type":"object"}`)
	opts := ChatOptions{
		SystemPrompt:   "",
		UserPrompt:     "generate",
		ResponseFormat: &ResponseFormat{Type: "json_schema", Schema: schema},
	}
	got := injectFormatPrompt(opts)
	if !strings.Contains(got.SystemPrompt, `{"type":"object"}`) {
		t.Error("injected system prompt should contain schema content")
	}
}
