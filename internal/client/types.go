package client

import "encoding/json"

// chatRequest is the JSON body sent to the OpenAI Chat Completions API.
type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []message       `json:"messages"`
	Stream         bool            `json:"stream,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *jsonSchema `json:"json_schema,omitempty"`
}

type jsonSchema struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

// chatResponse is the JSON body of a non-streaming response.
type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

// streamChunk is a single SSE data chunk from the streaming API.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// ResponseFormat describes how the model should format its output.
type ResponseFormat struct {
	// Type is "json_object" or "json_schema".
	Type string
	// SchemaName is the schema identifier used when Type is "json_schema".
	SchemaName string
	// Schema is the raw JSON Schema document used when Type is "json_schema".
	Schema json.RawMessage
}
