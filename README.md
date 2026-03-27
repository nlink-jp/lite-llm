# lite-llm

A lightweight CLI for OpenAI-compatible LLM APIs, designed for scripting and data-pipeline use cases.

## Features

- **Data isolation** — stdin and file inputs are automatically wrapped in nonce-tagged XML to prevent prompt injection (enabled by default)
- **Batch mode** — process an input file line by line, one LLM request per line
- **Structured output** — `--format json` and `--json-schema` with automatic fallback for local LLMs that don't support `response_format`
- **Streaming** — token-by-token output via `--stream`
- **Quiet mode** — `--quiet` / `-q` suppresses warnings for clean pipeline output

## Installation

```sh
git clone https://github.com/nlink-jp/lite-llm.git
cd lite-llm
make build
# binary: bin/lite-llm
```

Or download a pre-built binary from the [releases page](https://github.com/nlink-jp/lite-llm/releases).

## Quick Start

```sh
# Set your API key
export LITE_LLM_API_KEY=sk-...

# Ask a question
lite-llm "What is the capital of Japan?"

# Pipe data in (automatically isolated from instructions)
echo "2024-01-15: Revenue $12,400" | lite-llm "Extract the date and amount as JSON" --format json

# Batch processing
cat questions.txt | lite-llm --batch --format jsonl \
  --system-prompt "Answer in one sentence."

# Streaming
lite-llm --stream "Write a haiku about Go programming"
```

## Configuration

Copy the example config and set your values:

```sh
mkdir -p ~/.config/lite-llm
cp config.example.toml ~/.config/lite-llm/config.toml
chmod 600 ~/.config/lite-llm/config.toml
```

```toml
# ~/.config/lite-llm/config.toml
[api]
base_url = "https://api.openai.com"
api_key  = "sk-..."

[model]
name = "gpt-4o-mini"
```

**Priority order (highest first):** CLI flags → environment variables → config file → compiled-in defaults

| Environment variable  | Description        |
|-----------------------|--------------------|
| `LITE_LLM_API_KEY`   | API key            |
| `LITE_LLM_BASE_URL`  | API base URL       |
| `LITE_LLM_MODEL`     | Default model name |

## Usage

```
lite-llm [flags] [prompt]

Input flags:
  -p, --prompt string              User prompt text
  -f, --file string                Input file path (use - for stdin)
  -s, --system-prompt string       System prompt text
  -S, --system-prompt-file string  System prompt file path

Model / endpoint:
  -m, --model string               Model name (overrides config)
      --endpoint string            API base URL (overrides config)

Execution mode:
      --stream                     Enable streaming output
      --batch                      Batch mode: one request per input line

Output format:
      --format string              Output format: text (default), json, jsonl
      --json-schema string         JSON Schema file (implies --format json)

Security:
      --no-safe-input              Disable automatic data isolation
  -q, --quiet                      Suppress warnings on stderr
      --debug                      Log API request and response bodies to stderr

Config:
  -c, --config string              Config file path
```

## Data Isolation

When input comes from a file or stdin, lite-llm wraps it in a randomly-tagged XML
element to prevent prompt injection:

```
<user_data_a3f8b2>
{your data here}
</user_data_a3f8b2>
```

Use `{{DATA_TAG}}` in your **system prompt** to reference the tag by name:

```sh
echo "Alice, 34, engineer" | lite-llm \
  --system-prompt "Extract fields from <{{DATA_TAG}}>. Return JSON with keys: name, age, role." \
  --format json
```

> `{{DATA_TAG}}` is expanded **only in the system prompt**, never in user input.

Disable with `--no-safe-input` when the input is trusted.

## Structured Output

```sh
# JSON object
lite-llm --format json "List three Go best practices"

# JSON Schema
lite-llm --json-schema person.json "Generate a fictional person"

# Batch + JSONL
lite-llm --batch --format jsonl \
  --system-prompt "Classify sentiment: positive, negative, or neutral." \
  --file reviews.txt
```

For local LLMs (LM Studio, Ollama) that don't support `response_format`, set:

```toml
response_format_strategy = "auto"   # default: try native, fall back to prompt injection
# or
response_format_strategy = "prompt" # always use prompt injection, never send response_format
```

## Local LLM (LM Studio / Ollama)

```sh
# LM Studio
lite-llm --endpoint http://localhost:1234/v1 --model my-model "Hello"

# Ollama
lite-llm --endpoint http://localhost:11434 --model llama3 "Hello"
```

lite-llm handles both `http://localhost:1234/v1` and `http://localhost:1234` endpoint
formats correctly.

## Quiet Mode

Suppress warnings (useful in scripts and pipelines):

```sh
lite-llm -q --format json "give me json" | jq .
```

## Documentation

[日本語版 README](README.ja.md)

- [Setup Guide](docs/setup.md) / [日本語](docs/ja/setup.md)
- [Prompting Guide](docs/guide/prompting.md) / [日本語](docs/ja/guide/prompting.md)
- [Structured Output Guide](docs/guide/structured-output.md) / [日本語](docs/ja/guide/structured-output.md)
- [Architecture Overview](docs/design/overview.md) / [日本語](docs/ja/design/overview.md)
- [Prompt Injection Guard: Effectiveness Test](docs/design/prompt-injection-test.md) / [日本語](docs/ja/design/prompt-injection-test.md)

## Building from Source

Requires Go 1.26+.

```sh
make build          # current platform → bin/lite-llm
make build-all      # all 5 platforms  → dist/
make check          # vet + lint + test + build + govulncheck
make setup          # install git hooks
```

## License

See [LICENSE](LICENSE) if present, or contact the author.
