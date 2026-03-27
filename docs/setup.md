# Setup Guide

## Prerequisites

- Go 1.26 or later
- `golangci-lint` (for `make lint` and `make check`)
- `govulncheck` (for `make check`)

Install linting tools:

```sh
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
```

## Build

```sh
git clone <repo-url>
cd lite-llm
go mod download
make build
# Binary is at bin/lite-llm
```

## Install git hooks

```sh
make setup
```

This copies `scripts/hooks/pre-commit` and `scripts/hooks/pre-push` into `.git/hooks/`.

- **pre-commit**: runs `make vet lint` (fast; catches obvious issues before every commit)
- **pre-push**: runs `make check` (full gate: vet + lint + test + build + govulncheck)

## Configuration

1. Copy the example config:

   ```sh
   mkdir -p ~/.config/lite-llm
   cp config.example.toml ~/.config/lite-llm/config.toml
   ```

2. Edit `~/.config/lite-llm/config.toml` and set at minimum:

   ```toml
   [api]
   api_key = "sk-..."

   [model]
   name = "gpt-4o-mini"
   ```

3. **Secure the config file permissions** (required if the file contains an API key):

   ```sh
   chmod 600 ~/.config/lite-llm/config.toml
   ```

   lite-llm warns at startup if the config file is readable by group or others
   (i.e. permissions other than `0600`, `0400`, or `0700`):

   ```
   Warning: config file ~/.config/lite-llm/config.toml has permissions 0644; expected 0600.
     The file may contain an API key. Run: chmod 600 ~/.config/lite-llm/config.toml
   ```

4. Alternatively, set environment variables:

   ```sh
   export LITE_LLM_API_KEY="sk-..."
   export LITE_LLM_MODEL="gpt-4o-mini"
   ```

## Running tests

```sh
make test          # unit tests only
make check         # full quality gate
```

## Cross-compilation

```sh
make build-all
# Binaries are placed in dist/
```

Supported platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`.
