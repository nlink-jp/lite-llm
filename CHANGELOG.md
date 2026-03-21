# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-03-21

### Added

- **Data isolation**: stdin and file inputs are automatically wrapped in
  nonce-tagged XML (`<user_data_<hex6>>`) to prevent prompt injection.
  The `{{DATA_TAG}}` placeholder in system prompts expands to the tag name,
  allowing explicit reference to the data container.
- **Batch mode** (`--batch`): process an input file line by line, sending one
  LLM request per line. Errors on individual lines are reported and processing
  continues.
- **Structured output**:
  - `--format json` — request a JSON object response via `response_format`.
  - `--json-schema <file>` — constrain output to a JSON Schema with `strict: true`.
  - `response_format_strategy = auto|native|prompt` — fallback configuration for
    local LLMs (LM Studio, Ollama) that do not support `response_format`.
- **JSONL output** (`--format jsonl`) for batch mode: each processed line
  produces one JSON Lines record with `input`, `output`, and `error` fields.
- **Streaming output** (`--stream`): token-by-token output via SSE.
- **Config file** (`~/.config/lite-llm/config.toml`): supports `endpoint`,
  `model`, `api_key`, `timeout_seconds`, and `response_format_strategy`.
  Warns if the file permissions are not 0600.
- **Environment variable overrides**: `LITE_LLM_API_KEY`, `LITE_LLM_ENDPOINT`,
  `LITE_LLM_MODEL`.
- **Cross-compilation**: `make build-all` produces binaries for
  `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`.

[0.1.0]: https://github.com/magifd2/lite-llm/releases/tag/v0.1.0
