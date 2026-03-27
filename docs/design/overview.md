# lite-llm Architecture Overview

## Purpose

lite-llm is a minimal CLI for interacting with OpenAI API-compatible LLM endpoints.
It is a focused rebuild of [llm-cli](https://github.com/magifd2/llm-cli), dropping multi-provider
abstraction in favour of simplicity and two new data-analysis-oriented features:
automatic prompt-injection protection and batch processing.

## Directory Structure

```
lite-llm/
├── cmd/root.go              # cobra root command, flag definitions, orchestration
├── internal/
│   ├── config/              # TOML config loading, environment variable overrides
│   ├── client/              # OpenAI-compatible HTTP client (Chat, ChatStream)
│   ├── input/               # stdin / file reading, UTF-8 sanitization, line splitting
│   ├── isolation/           # prompt-injection protection (nonce tags, system-prompt wrapping)
│   └── output/              # text / JSON / JSONL response formatter
├── docs/
│   ├── design/              # this document and architecture notes
│   ├── guide/               # user-facing usage guides
│   ├── ja/                  # Japanese translations
│   ├── dependencies.md
│   ├── setup.md
│   └── structure.md
├── scripts/hooks/           # git hook scripts installed by `make setup`
├── main.go
├── go.mod
├── Makefile
└── config.example.toml
```

## Configuration

Values are resolved in priority order (highest first):

1. CLI flags (`--model`, `--endpoint`)
2. Environment variables (`LITE_LLM_API_KEY`, `LITE_LLM_BASE_URL`, `LITE_LLM_MODEL`)
3. Config file (`~/.config/lite-llm/config.toml`)
4. Compiled-in defaults

## Data Isolation (Prompt-Injection Protection)

Inputs from stdin or files are treated as **data**, not as instructions.
The `isolation` package implements this by:

1. Generating a cryptographically random nonce (3 bytes → 6 hex chars) per invocation.
2. Building a tag name: `user_data_<nonce>` (e.g. `user_data_a3f8b2`).
3. Wrapping the user input:
   ```
   <user_data_a3f8b2>
   {user input}
   </user_data_a3f8b2>
   ```
4. Prepending a `CRITICAL` isolation constraint to the system prompt (before any
   user-supplied system prompt text) that names the tag and instructs the model to
   treat its contents as data only.

The random nonce means an attacker who embeds `</user_data_XXXX>` in data cannot
predict the tag name and therefore cannot escape the data container.

### `{{DATA_TAG}}` placeholder

Users may reference the generated tag name in their **system prompt** by writing
`{{DATA_TAG}}`. At runtime this is replaced with the actual tag name (e.g.
`user_data_a3f8b2`).

**Important:** `{{DATA_TAG}}` expansion is applied **only to the system prompt**.
It is never expanded in user input, file contents, or any other field. Expanding it
in user input would allow malicious data to learn the nonce and escape the isolation.

### Disabling isolation

Pass `--no-safe-input` to skip wrapping. Use this only when the input is known to
be trusted (e.g. scripted prompts you control).

## Batch Mode

When `--batch` is set, the input is split into lines (empty lines are skipped) and
each line is sent as a separate request. Processing is sequential. Errors on
individual lines are written to stderr; processing continues with the next line.

Each batch line is treated as external data and isolation is applied automatically
(regardless of whether the input comes from a file or stdin).

## Structured Output

| Flag | API behaviour | Fallback (prompt strategy) |
|------|--------------|---------------------------|
| `--format json` | `response_format: {type: json_object}` | Injects "Respond with valid JSON only." into system prompt |
| `--json-schema <file>` | `response_format: {type: json_schema, ...}` | Injects schema content into system prompt |

The `response_format_strategy` config key controls fallback behaviour:

- `auto` (default): send `response_format`; if the API rejects it with a 4xx
  containing "response_format", "not supported", "unsupported", or "unknown field",
  retry with prompt injection and log a warning.
- `native`: always send `response_format`; fail if unsupported.
- `prompt`: never send `response_format`; always use prompt injection.

When a model emits content around the JSON payload (control tokens, `<think>` /
`<reasoning>` tags, Mistral `[THINK]` tags, or markdown code fences), the output
formatter automatically strips the preamble and extracts the JSON value. If no
valid JSON is found, the response is emitted as a JSON-encoded string.

## Endpoint URL resolution

The client appends the chat completions path to the configured endpoint:

- If the endpoint already ends with `/v1` (e.g. `http://localhost:1234/v1`),
  only `/chat/completions` is appended → `http://localhost:1234/v1/chat/completions`.
- Otherwise `/v1/chat/completions` is appended → `https://api.openai.com/v1/chat/completions`.

This allows both bare base URLs (`https://api.openai.com`) and versioned URLs
(`http://localhost:1234/v1`, common in LM Studio and similar local servers) to work
without manual path adjustment.

## Debug Mode (`--debug`)

Pass `--debug` to log the full API request and response bodies to stderr before and
after each HTTP call. Both `chatOnce` (blocking) and `ChatStream` (streaming) log
their payloads. The output is pretty-printed JSON when the payload is valid JSON,
raw bytes otherwise.

This is useful for verifying that data isolation is working correctly: the logged
`messages` array will show the `CRITICAL` constraint prepended to the system prompt
and the user input wrapped in `<user_data_XXXXXX>` tags.

`--debug` and `--quiet` can coexist: `--quiet` suppresses warning messages while
`--debug` logs request/response details.

## Quiet Mode (`--quiet` / `-q`)

Pass `--quiet` (or `-q`) to suppress all warnings written to stderr:

- `response_format` fallback warning (when auto-retry with prompt injection occurs)
- Config file permission warning (when the file is not 0600)

Warnings are routed through `cmd.ErrOrStderr()` so they can be captured or discarded
cleanly both in normal use and in tests.

## Quality Gates

- **pre-commit hook**: `make vet lint`
- **pre-push hook**: `make check` (vet + lint + test + build + `govulncheck`)

Install with `make setup`.
