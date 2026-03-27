# AGENTS.md — lite-llm

Lightweight CLI for OpenAI-compatible LLM APIs.
Part of [lite-series](https://github.com/nlink-jp/lite-series).

## Rules

- Project rules (security, testing, docs, release, etc.): → [RULES.md](RULES.md)
- Series-wide conventions (config format, CLI, Makefile, etc.): → [../CONVENTIONS.md](https://github.com/nlink-jp/lite-series/blob/main/CONVENTIONS.md)

## Build & test

```sh
make build    # bin/lite-llm
make check    # vet → lint → test → build → govulncheck (full gate)
go test ./... # tests only
```

## Key structure

```
cmd/root.go              ← CLI entry point (Cobra); flag → config wiring lives here
internal/config/         ← Config struct, Load(), env overrides
internal/client/         ← HTTP client for OpenAI-compatible API
internal/isolation/      ← Prompt-injection protection (nonce-tagged XML wrapping)
internal/input/          ← stdin / file reading
internal/output/         ← text / JSON / JSONL formatting
```

## Gotchas

- **Config format**: sectioned TOML (`[api]`, `[model]`). Flat keys are rejected. See `config.example.toml`.
- **Module path**: `github.com/nlink-jp/lite-llm` — use this in all imports.
- **`/v1` handling**: `cfg.API.BaseURL` accepts with or without `/v1` suffix; `client.endpoint()` normalises it.
- **No hosted CI**: quality gate runs locally via Git hooks. Run `make setup` once after cloning.
- **Env var for base URL**: `LITE_LLM_BASE_URL` (not `ENDPOINT`).
