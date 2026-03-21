# lite-llm アーキテクチャ概要

## 目的

lite-llm は OpenAI API 互換エンドポイント専用の軽量 CLI ツールです。
既存の [llm-cli](https://github.com/magifd2/llm-cli) をリビルドし、マルチプロバイダ抽象化を
廃してシンプルさを追求した上で、データ分析用途向けの以下の2機能を追加しています:

- 自動プロンプトインジェクション対策（データアイソレーション）
- バッチ処理モード

## ディレクトリ構造

```
lite-llm/
├── cmd/root.go              # cobra ルートコマンド・フラグ定義・オーケストレーション
├── internal/
│   ├── config/              # TOML設定読み込み・環境変数オーバーライド
│   ├── client/              # OpenAI互換 HTTPクライアント (Chat, ChatStream)
│   ├── input/               # stdin/ファイル読み込み・UTF-8サニタイズ・行分割
│   ├── isolation/           # プロンプトインジェクション対策（nonceタグ・ラッピング）
│   └── output/              # テキスト/JSON/JSONL レスポンスフォーマッタ
├── docs/
│   ├── design/              # 本ドキュメントとアーキテクチャメモ
│   ├── guide/               # ユーザー向け利用ガイド
│   ├── ja/                  # 日本語訳
│   ├── dependencies.md
│   ├── setup.md
│   └── structure.md
├── scripts/hooks/           # `make setup` でインストールされる git フック
├── main.go
├── go.mod
├── Makefile
└── config.example.toml
```

## 設定管理

以下の優先順位（高い順）で値を解決します:

1. CLI フラグ（`--model`, `--endpoint`）
2. 環境変数（`LITE_LLM_API_KEY`, `LITE_LLM_ENDPOINT`, `LITE_LLM_MODEL`）
3. 設定ファイル（`~/.config/lite-llm/config.toml`）
4. コンパイル時デフォルト値

## データアイソレーション（プロンプトインジェクション対策）

stdin またはファイルから入力されたデータは**指示ではなくデータ**として扱われます。
`isolation` パッケージが以下の処理を行います:

1. 実行ごとに暗号学的ランダム nonce を生成（3バイト → 6桁の hex 文字）。
2. タグ名を構築: `user_data_<nonce>`（例: `user_data_a3f8b2`）。
3. ユーザー入力をラッピング:
   ```
   <user_data_a3f8b2>
   {ユーザー入力}
   </user_data_a3f8b2>
   ```
4. システムプロンプトにタグ名を明示した注記を付加し、モデルにデータとして扱うよう指示。

nonce がランダムであるため、攻撃者がデータ内に `</user_data_XXXX>` を埋め込んでも
正しいタグ名を予測できず、データコンテナから脱出できません。

### `{{DATA_TAG}}` プレースホルダー

**システムプロンプトにのみ作用します。**

`{{DATA_TAG}}` の展開は `--system-prompt` / `--system-prompt-file` で指定した
**システムプロンプトの文字列に対してのみ** 行われます。ユーザー入力・ファイル内容・
stdin など他のフィールドには一切作用しません。ユーザー入力で展開すると悪意あるデータが
nonce を知ることができ、アイソレーションを回避されるリスクがあるため、意図的に制限して
います。

システムプロンプトにデータコンテナを参照したい場合に使用します:

```sh
lite-llm \
  --system-prompt "<{{DATA_TAG}}> 内のデータから日付を抽出し JSON 配列で返してください。" \
  --file report.txt
```

### アイソレーションの無効化

信頼できる入力に対しては `--no-safe-input` で無効化できます。

## バッチモード

`--batch` 指定時、入力を行分割（空行スキップ）して各行を独立したリクエストとして
シーケンシャルに処理します。1行の処理でエラーが発生した場合は stderr に出力し
次の行へ進みます。バッチ入力は常に外部データとして扱われ、アイソレーションが
自動適用されます。

## 構造化出力

| フラグ | API の動作 | フォールバック（prompt 戦略） |
|--------|-----------|---------------------------|
| `--format json` | `response_format: {type: json_object}` | "Respond with valid JSON only." をシステムプロンプトに注入 |
| `--json-schema <file>` | `response_format: {type: json_schema, ...}` | スキーマ内容をシステムプロンプトに注入 |

`response_format_strategy` 設定値:

- `auto`（デフォルト）: `response_format` を送信し、非対応エラー検出時は
  プロンプト注入にフォールバックして警告を stderr に出力。
- `native`: 常に `response_format` を送信。非対応なら失敗。
- `prompt`: `response_format` を送信しない。常にプロンプト注入のみ使用。

モデルが JSON ペイロードの周囲に余分な内容（コントロールトークン、`<think>` /
`<reasoning>` タグ、Mistral の `[THINK]` タグ、Markdown コードフェンス等）を
出力した場合、フォーマッターが自動的にそれを除去して JSON 値を抽出します。
有効な JSON が見つからない場合は、レスポンス全体を JSON 文字列として出力します。

## エンドポイント URL の解決

クライアントは設定されたエンドポイントにチャット補完パスを付加します:

- エンドポイントが `/v1` で終わる場合（例: `http://localhost:1234/v1`）、
  `/chat/completions` のみを付加 → `http://localhost:1234/v1/chat/completions`
- それ以外の場合は `/v1/chat/completions` を付加 → `https://api.openai.com/v1/chat/completions`

これにより、ベース URL（`https://api.openai.com`）とバージョン付き URL
（`http://localhost:1234/v1`、LM Studio などのローカルサーバーで一般的）の
どちらも手動でパスを調整することなく動作します。

## クワイエットモード（`--quiet` / `-q`）

`--quiet`（または `-q`）を指定すると、stderr に書き込まれるワーニングをすべて
抑制します:

- `response_format` フォールバックワーニング（プロンプト注入での自動リトライ時）
- 設定ファイルのパーミッションワーニング（0600 以外の場合）

ワーニングは `cmd.ErrOrStderr()` を経由して出力されるため、通常使用・テスト双方で
クリーンにキャプチャまたは破棄できます。

## 品質ゲート

- **pre-commit フック**: `make vet lint`
- **pre-push フック**: `make check`（vet + lint + test + build + `govulncheck`）

`make setup` でインストールします。
