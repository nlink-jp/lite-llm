# 構造化出力ガイド

## 概要

lite-llm は3つの構造化出力モードをサポートしています:

| モード | フラグ | API の仕組み | 信頼性 |
|--------|--------|-------------|--------|
| JSON オブジェクト | `--format json` | `response_format: {type: json_object}` | 高（ネイティブ） |
| JSON スキーマ | `--json-schema <file>` | `response_format: {type: json_schema, ...}` | 最高（ネイティブ） |
| プロンプト注入 | `response_format_strategy = prompt` | システムプロンプトによる指示のみ | 低め |

## `--format json` — JSON オブジェクト出力

モデルに有効な JSON オブジェクトを返すよう要求します。API がネイティブにフォーマットを
強制します。

```sh
lite-llm --format json "プログラミング言語5つとその誕生年を列挙してください"
```

**重要な制約（OpenAI API の要件）:** `json_object` モードを使用する場合、プロンプト
（システムプロンプトまたはユーザープロンプト）のどこかに "JSON" という語を含める
必要があります。含まれていない場合、OpenAI API はエラーを返すことがあります。
lite-llm は "JSON" を自動的に追加しません。

良い例:
```sh
lite-llm --format json "上位3惑星を JSON オブジェクトで返してください"
```

## `--json-schema <file>` — スキーマ制約出力

JSON スキーマに厳密に準拠した出力を要求します。予測可能な形状の構造化データを
得る最も信頼性の高い方法です。

```sh
lite-llm --json-schema person.json "架空の人物を生成してください"
```

### スキーマファイルの書き方

スキーマファイルは JSON Schema オブジェクトを含む有効な JSON ドキュメントで
なければなりません。

例 `person.json`:

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

### OpenAI strict モードの制約

`--json-schema` 使用時、lite-llm は常に `strict: true` を送信します。
OpenAI API の strict モードには以下の制約があります:

- オブジェクトの全プロパティを `required` に列挙する必要がある。
- `additionalProperties` は `false` にする必要がある。
- 再帰的スキーマは `$defs` を使用する必要がある。
- `if/then/else`, `not` などのキーワードは非対応。

詳細は [OpenAI structured outputs ドキュメント](https://platform.openai.com/docs/guides/structured-outputs)
を参照してください。

### スキーマ名

API に送信されるスキーマ名はファイル名（拡張子なし）から導出されます。
例: `person.json` → 名前は `person`。説明的なファイル名を選んでください。

### システムプロンプトとの組み合わせ

`--json-schema` を使う場合でも、期待する出力をシステムプロンプトで
自然言語で説明すると品質が向上することがあります:

```sh
lite-llm \
  --json-schema person.json \
  --system-prompt "ファンタジー小説に登場するリアルな架空の人物を生成してください。" \
  "キャラクターを作成"
```

### バッチモードとの組み合わせ

`--json-schema` はバッチモードと組み合わせて使用できます。その場合は
`--format jsonl` を明示的に指定してください（自動的には有効になりません）。

```sh
lite-llm \
  --batch \
  --format jsonl \
  --json-schema sentiment.json \
  --system-prompt "入力テキストのセンチメントを分類してください。" \
  --file reviews.txt
```

各バッチ行の結果は JSONL レコードとして出力され、`output` フィールドにモデルの
JSON レスポンス文字列が格納されます。

## フォールバック戦略（`response_format_strategy`）

LM Studio や Ollama などのローカル LLM API は `response_format` フィールドを
サポートしておらず、含まれている場合に 4xx エラーを返すことがあります。
`~/.config/lite-llm/config.toml` でフォールバック動作を設定します:

### `auto`（デフォルト）

`response_format` を送信し、API が拒否した場合はプロンプト注入にフォールバックして
stderr に警告を出力します。

```toml
response_format_strategy = "auto"
```

以下のパターンをエラーボディで検出した場合にフォールバックします:
`response_format`, `not supported`, `unsupported`, `unknown field`

### `native`

常に `response_format` を送信します。非対応の場合はエラーで終了します。
OpenAI など、構造化出力をサポートすることが確実な API に使用します。

```toml
response_format_strategy = "native"
```

### `prompt`

`response_format` を送信しません。常にシステムプロンプト注入のみ使用します。
`response_format` を非対応のローカル LLM（LM Studio, Ollama 等）に推奨します。

```toml
response_format_strategy = "prompt"
```

**`prompt` 戦略の限界:**

- JSON 出力が保証されない（モデルが JSON の前後に散文を含める可能性がある）。
- スキーマ準拠はベストエフォート（複雑なスキーマでは完全に従わない場合がある）。
- `--json-schema` は機能するが、スキーマ強制はモデルの指示追従能力に依存する。

**JSON の自動抽出:**

モデルが JSON ペイロードの前後に余分な内容を出力した場合、lite-llm は自動的に
それを除去して JSON を抽出します。対応パターン:

| パターン | 代表的なモデル |
|---------|--------------|
| `<think>...</think>` / `<reasoning>...</reasoning>` | DeepSeek R1, Qwen3, QwQ |
| `[THINK]...[/THINK]` | Mistral（Magistral, Ministral-3, Devstral） |
| Markdown コードフェンス（` ```json ``` `、` ``` ``` `） | 各種 |
| コントロールトークンや `{` / `[` 前のプリアンブルテキスト | 各種 |

有効な JSON が見つからない場合は、レスポンス全体を JSON 文字列として出力します。

## `--format jsonl` — JSON Lines（バッチ専用）

`--format jsonl` は **`--batch` と同時に指定する必要があります**。`--batch` なしで
`--format jsonl` を指定した場合はエラーになります。各処理行が1つの JSON Lines
レコードを生成します:

```jsonl
{"input":"行テキスト","output":"モデルのレスポンス","error":null}
```

エラー時:

```jsonl
{"input":"行テキスト","output":null,"error":"API error (status 429): rate limit exceeded"}
```

`jq` などのツールで下流処理が容易です:

```sh
cat results.jsonl | jq -r '.output' | head -5
```

## `--json-schema` と `--stream` の非互換性

構造化出力（`--json-schema`）はレスポンス全体を受け取ってスキーマを検証する必要が
あります。ストリーミングは部分トークンを返すためスキーマ検証ができません。
これらのフラグを同時に指定した場合はエラーになります。
