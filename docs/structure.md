# Project Directory Structure

```
lite-llm/
├── cmd/
│   └── root.go              # cobra root command and orchestration logic
├── internal/
│   ├── config/
│   │   ├── config.go        # Config struct, Load(), DefaultConfigPath(), ResolvePath()
│   │   └── config_test.go
│   ├── client/
│   │   ├── client.go        # Client, Chat(), ChatStream(), fallback logic
│   │   ├── types.go         # JSON request/response types, ResponseFormat
│   │   └── client_test.go
│   ├── input/
│   │   ├── reader.go        # ReadUserInput(), ReadSystemPrompt(), ReadLines(), sanitizeUTF8()
│   │   └── reader_test.go
│   ├── isolation/
│   │   ├── wrapper.go       # WrapInput(), generateNonce(), isolationNote()
│   │   └── wrapper_test.go
│   └── output/
│       ├── formatter.go     # Formatter, ModeText/ModeJSON/ModeJSONL, Write(), WriteJSONL()
│       └── formatter_test.go
├── docs/
│   ├── design/
│   │   └── overview.md      # architecture and design decisions
│   ├── guide/
│   │   ├── prompting.md     # prompt writing guide
│   │   └── structured-output.md  # JSON/schema output guide
│   ├── ja/                  # Japanese translations (mirrors docs/ structure)
│   ├── dependencies.md      # third-party dependency rationale
│   ├── setup.md             # build and configuration guide
│   └── structure.md         # this file
├── scripts/
│   └── hooks/
│       ├── pre-commit       # git pre-commit hook
│       └── pre-push         # git pre-push hook
├── external/                # read-only reference: original llm-cli source (https://github.com/magifd2/llm-cli)
├── main.go                  # entry point
├── go.mod
├── go.sum
├── Makefile
├── config.example.toml      # configuration template
├── RULES.md                 # project rules
└── .gitignore
```

## Design rationale

- `internal/` only — no public library API (`pkg/` is unused).
- A single cobra command at the root level; no subcommands.
- Each `internal/` package has one clear responsibility and no circular dependencies:
  `config` → `client` → `cmd` (config and client do not import each other's siblings).
- `isolation` and `output` are pure packages with no I/O side effects, making them
  straightforward to unit-test.
