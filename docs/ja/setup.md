# セットアップガイド

## 前提条件

- Go 1.26 以上
- `golangci-lint`（`make lint` および `make check` に必要）
- `govulncheck`（`make check` に必要）

ツールのインストール:

```sh
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
```

## ビルド

```sh
git clone <リポジトリURL>
cd lite-llm
go mod download
make build
# バイナリは bin/lite-llm に生成されます
```

## git フックのインストール

```sh
make setup
```

`scripts/hooks/pre-commit` と `scripts/hooks/pre-push` を `.git/hooks/` にコピーします。

- **pre-commit**: `make vet lint` を実行（高速；コミット前に明らかな問題を検出）
- **pre-push**: `make check` を実行（完全ゲート: vet + lint + test + build + govulncheck）

## 設定

1. 設定ファイルのテンプレートをコピー:

   ```sh
   mkdir -p ~/.config/lite-llm
   cp config.example.toml ~/.config/lite-llm/config.toml
   ```

2. `~/.config/lite-llm/config.toml` を編集して最低限以下を設定:

   ```toml
   model = "gpt-4o-mini"
   api_key = "sk-..."
   ```

3. **設定ファイルのパーミッションを保護する**（API キーを含む場合は必須）:

   ```sh
   chmod 600 ~/.config/lite-llm/config.toml
   ```

   設定ファイルがグループまたはその他のユーザーに読み取り可能な場合
   （`0600`, `0400`, `0700` 以外のパーミッション）、起動時に警告が表示されます:

   ```
   Warning: config file ~/.config/lite-llm/config.toml has permissions 0644; expected 0600.
     The file may contain an API key. Run: chmod 600 ~/.config/lite-llm/config.toml
   ```

4. または環境変数で設定:

   ```sh
   export LITE_LLM_API_KEY="sk-..."
   export LITE_LLM_MODEL="gpt-4o-mini"
   ```

## テスト実行

```sh
make test          # ユニットテストのみ
make check         # 完全品質ゲート
```

## クロスコンパイル

```sh
make build-all
# バイナリは dist/ に生成されます
```

対応プラットフォーム: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`
