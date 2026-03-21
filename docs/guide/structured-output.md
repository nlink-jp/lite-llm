# Structured Output Guide

## Overview

lite-llm supports three structured output modes:

| Mode | Flag | API mechanism | Reliability |
|------|------|--------------|-------------|
| JSON object | `--format json` | `response_format: {type: json_object}` | High (native) |
| JSON Schema | `--json-schema <file>` | `response_format: {type: json_schema, ...}` | Highest (native) |
| Prompt injection | `response_format_strategy = prompt` | System-prompt instruction only | Lower |

## `--format json` — JSON object output

Requests a valid JSON object from the model. The API enforces the format natively.

```sh
lite-llm --format json "List 5 programming languages with their year of creation"
```

**Important constraint (OpenAI API requirement):** When using `json_object` mode,
your prompt *must* contain the word "JSON" somewhere (in the system prompt or user
prompt). If it does not, the OpenAI API may return an error. lite-llm does not
add "JSON" automatically.

Good:
```sh
lite-llm --format json "Return a JSON object with the top 3 planets by size"
```

## `--json-schema <file>` — Schema-constrained output

Requests output that strictly conforms to a JSON Schema. This is the most reliable
way to get structured data with a predictable shape.

```sh
lite-llm --json-schema person.json "Generate a fictional person"
```

### Writing the schema file

The schema file must be a valid JSON document containing a JSON Schema object.

Example `person.json`:

```json
{
  "type": "object",
  "properties": {
    "name": { "type": "string" },
    "age": { "type": "integer", "minimum": 0 },
    "email": { "type": "string", "format": "email" }
  },
  "required": ["name", "age"],
  "additionalProperties": false
}
```

### OpenAI strict mode constraints

When `--json-schema` is used, lite-llm always sends `strict: true`.
The OpenAI API imposes the following constraints in strict mode:

- All object properties must be listed in `required`.
- `additionalProperties` must be `false`.
- Recursive schemas are supported but must use `$defs`.
- `anyOf`, `oneOf`, `allOf` are supported with restrictions.
- Some JSON Schema keywords are not supported (e.g. `if/then/else`, `not`).

See the [OpenAI structured outputs documentation](https://platform.openai.com/docs/guides/structured-outputs)
for the full list.

### Schema name

The schema name sent to the API is derived from the filename (without extension).
For example, `person.json` → name `person`. Choose descriptive filenames.

### Combining with system prompt

Even when using `--json-schema`, describing the expected output in plain English in
the system prompt can improve quality:

```sh
lite-llm \
  --json-schema person.json \
  --system-prompt "Generate a realistic fictional person suitable for a fantasy novel." \
  "Create a character"
```

### `--json-schema` with batch mode

```sh
lite-llm \
  --batch \
  --json-schema sentiment.json \
  --system-prompt "Classify the sentiment of the input text." \
  --file reviews.txt
```

Note: `--format jsonl` is automatically implied when `--batch` is used. Each batch
result is a JSONL record where `output` contains the model's JSON response as a string.

## Fallback strategy (`response_format_strategy`)

Local LLM APIs (LM Studio, Ollama, etc.) may not support the `response_format` field
and return a 4xx error when it is present. Configure the fallback behaviour in
`~/.config/lite-llm/config.toml`:

### `auto` (default)

Send `response_format`; if the API rejects it, retry with prompt injection and
print a warning to stderr.

```toml
response_format_strategy = "auto"
```

Fallback detection looks for these patterns in the error body:
`response_format`, `not supported`, `unsupported`, `unknown field`.

### `native`

Always send `response_format`. If the API does not support it, the command fails.
Use this for OpenAI or other APIs known to support structured output.

```toml
response_format_strategy = "native"
```

### `prompt`

Never send `response_format`. Always use system-prompt injection only.
Recommended for local LLMs (LM Studio, Ollama) that do not support `response_format`.

```toml
response_format_strategy = "prompt"
```

**Limitations of prompt strategy:**

- JSON output is not guaranteed; the model may include prose before or after the JSON.
- Schema compliance is best-effort; complex schemas may not be followed precisely.
- `--json-schema` still works but schema enforcement relies entirely on the model's
  instruction-following capability.

## `--format jsonl` — JSON Lines (batch only)

`--format jsonl` **requires `--batch`**. Specifying `--format jsonl` without `--batch`
is an error. Each processed line produces one JSON Lines record:

```jsonl
{"input":"line text","output":"model response","error":null}
```

On error:

```jsonl
{"input":"line text","output":null,"error":"API error (status 429): rate limit exceeded"}
```

This format is designed for downstream processing with tools like `jq`:

```sh
cat results.jsonl | jq -r '.output' | head -5
```

## `--json-schema` and `--stream` are incompatible

Structured output (`--json-schema`) requires the full response to validate the schema.
Streaming returns partial tokens and cannot be validated. These flags cannot be combined.
