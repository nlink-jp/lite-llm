# lite-llm

> **アーカイブ済み:** このプロジェクトは [llm-cli](https://github.com/nlink-jp/llm-cli) に引き継がれました。llm-cli は同等の全機能に加え、マルチイメージ VLM 入力、[nlk](https://github.com/nlink-jp/nlk) ライブラリの完全統合、JSON スキーマ構造化出力を提供します。新規利用には llm-cli をお使いください。

OpenAI 互換 LLM API 向けの軽量 CLI です。スクリプティングやデータパイプラインでの利用を想定して設計されています。

## 主な機能

- **データアイソレーション** — stdin・ファイル入力をランダムタグ付き XML で自動ラップし、プロンプトインジェクションを防止（デフォルト有効）
- **バッチモード** — 入力ファイルを行単位で処理し、1行につき1リクエストを送信
- **構造化出力** — `--format json` / `--json-schema` に対応。`response_format` 非対応のローカル LLM 向け自動フォールバック付き
- **ストリーミング** — `--stream` によるトークン逐次出力
- **クワイエットモード** — `--quiet` / `-q` でワーニングを抑制し、パイプラインをクリーンに保つ

## インストール

```sh
git clone https://github.com/nlink-jp/lite-llm.git
cd lite-llm
make build
# バイナリ: dist/lite-llm
```

またはビルド済みバイナリを[リリースページ](https://github.com/nlink-jp/lite-llm/releases)からダウンロードしてください。

## クイックスタート

```sh
# API キーを設定
export LITE_LLM_API_KEY=sk-...

# 質問する
lite-llm "日本の首都はどこですか？"

# データをパイプで渡す（自動的にアイソレーション適用）
echo "2024-01-15: 売上 1,240,000円" | lite-llm "日付と金額を JSON で抽出してください" --format json

# バッチ処理
cat questions.txt | lite-llm --batch --format jsonl \
  --system-prompt "1文で回答してください。"

# ストリーミング
lite-llm --stream "Go プログラミングについて俳句を詠んでください"
```

## 設定

設定ファイルのサンプルをコピーして値を編集します：

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

**優先順位（高い順）:** CLI フラグ → 環境変数 → 設定ファイル → コンパイル時デフォルト

| 環境変数               | 説明              |
|-----------------------|-------------------|
| `LITE_LLM_API_KEY`   | API キー          |
| `LITE_LLM_BASE_URL`  | API ベース URL    |
| `LITE_LLM_MODEL`     | デフォルトモデル名 |

## 使い方

```
lite-llm [flags] [prompt]

入力:
  -p, --prompt string              ユーザープロンプトテキスト
  -f, --file string                入力ファイルパス（- で stdin）
  -s, --system-prompt string       システムプロンプトテキスト
  -S, --system-prompt-file string  システムプロンプトファイルパス

モデル / エンドポイント:
  -m, --model string               モデル名（設定ファイルを上書き）
      --endpoint string            API ベース URL（設定ファイルを上書き）

実行モード:
      --stream                     ストリーミング出力を有効化
      --batch                      バッチモード: 1行につき1リクエスト

出力フォーマット:
      --format string              出力形式: text（デフォルト）, json, jsonl
      --json-schema string         JSON Schema ファイル（--format json を暗黙的に有効化）

セキュリティ:
      --no-safe-input              データアイソレーションを無効化
  -q, --quiet                      stderr へのワーニングを抑制
      --debug                      API リクエスト・レスポンスの内容を stderr に出力

設定:
  -c, --config string              設定ファイルパス
```

## データアイソレーション

ファイルや stdin からの入力は、プロンプトインジェクションを防ぐためにランダムタグ付き XML 要素でラップされます：

```
<user_data_a3f8b2>
{入力データ}
</user_data_a3f8b2>
```

**システムプロンプト**内で `{{DATA_TAG}}` を使うと、タグ名を参照できます：

```sh
echo "Alice, 34, エンジニア" | lite-llm \
  --system-prompt "<{{DATA_TAG}}> からフィールドを抽出し、name・age・role をキーとする JSON を返してください。" \
  --format json
```

> `{{DATA_TAG}}` の展開は**システムプロンプトのみ**に作用します。ユーザー入力には展開されません。

入力が信頼できる場合は `--no-safe-input` で無効化できます。

## 構造化出力

```sh
# JSON オブジェクト
lite-llm --format json "Go のベストプラクティスを3つ挙げてください"

# JSON Schema
lite-llm --json-schema person.json "架空の人物を生成してください"

# バッチ + JSONL
lite-llm --batch --format jsonl \
  --system-prompt "入力テキストの感情を positive / negative / neutral の1語で分類してください。" \
  --file reviews.txt
```

`response_format` に非対応のローカル LLM（LM Studio, Ollama 等）を使う場合は設定ファイルに追記します：

```toml
response_format_strategy = "auto"   # デフォルト: ネイティブ送信、非対応ならプロンプト注入にフォールバック
# または
response_format_strategy = "prompt" # 常にプロンプト注入のみ使用（response_format を送信しない）
```

## ローカル LLM（LM Studio / Ollama）

```sh
# LM Studio
lite-llm --endpoint http://localhost:1234/v1 --model my-model "こんにちは"

# Ollama
lite-llm --endpoint http://localhost:11434 --model llama3 "こんにちは"
```

`http://localhost:1234/v1` と `http://localhost:1234` のどちらの形式も正しく動作します。

## クワイエットモード

スクリプトやパイプラインでワーニングを抑制します：

```sh
lite-llm -q --format json "JSON を返してください" | jq .
```

## ドキュメント

- [セットアップガイド](docs/ja/setup.md)
- [プロンプトガイド](docs/ja/guide/prompting.md)
- [構造化出力ガイド](docs/ja/guide/structured-output.md)
- [アーキテクチャ概要](docs/ja/design/overview.md)
- [プロンプトインジェクションガード 効果測定レポート](docs/ja/design/prompt-injection-test.md)

## ソースからビルド

Go 1.26 以上が必要です。

```sh
make build          # 現在のプラットフォーム → dist/lite-llm
make build-all      # 5プラットフォーム全て  → dist/
make check          # vet + lint + test + build + govulncheck
make setup          # git フックをインストール
```

## ライセンス

[LICENSE](LICENSE) ファイルを参照するか、作者にお問い合わせください。
