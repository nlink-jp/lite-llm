# Dependency Rationale

## Direct dependencies

### `github.com/BurntSushi/toml`

- **Purpose**: Parsing TOML configuration files.
- **Why not in-house**: TOML has a non-trivial spec (multi-line strings, inline tables,
  datetime types). A custom parser would duplicate significant work and be harder to
  keep spec-compliant.
- **Why not JSON config**: TOML supports inline comments, which are important for a
  user-facing configuration file. JSON does not.
- **License**: MIT. No compliance concerns.

### `github.com/spf13/cobra`

- **Purpose**: CLI command and flag parsing.
- **Why not in-house**: Provides flag inheritance, `--help` generation, mutual-exclusion
  constraints (`MarkFlagsMutuallyExclusive`), and shell completion for free.
  Re-implementing this robustly would be a significant effort.
- **License**: Apache 2.0. No compliance concerns.

## Removed dependencies (compared to [llm-cli](https://github.com/magifd2/llm-cli))

| Package | Reason for removal |
|---------|-------------------|
| `cloud.google.com/go/auth` | Vertex AI support dropped |
| `github.com/aws/aws-sdk-go-v2` | Bedrock support dropped |
| `google.golang.org/genai` | Vertex AI support dropped |
| `github.com/briandowns/spinner` | Dependency minimisation; TTY detection via `os.File.Stat()` |
| `github.com/mattn/go-isatty` | Replaced by stdlib `os.File.Stat()` |

## Standard library only (no third-party)

The following features use stdlib only:

- HTTP client: `net/http`
- JSON encoding/decoding: `encoding/json`
- SSE stream parsing: `bufio.Scanner`
- Cryptographic nonce: `crypto/rand`
- UTF-8 validation: `unicode/utf8`, `strings.ToValidUTF8`
- JSON extraction from LLM preamble: `regexp` (think-tag removal, code-fence extraction)
