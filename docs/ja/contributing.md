# コントリビューション

コントリビューションに関心をお寄せいただきありがとうございます。このドキュメントはワークフローを説明します。
行動規範は [CODE_OF_CONDUCT.md](https://github.com/fonghehe/vue-h5-template-business-service/blob/main/CODE_OF_CONDUCT.md)
にあります。

## セットアップ

```bash
git clone https://github.com/fonghehe/vue-h5-template-business-service.git
cd vue-h5-template-business-service
```

Go 1.25+ が必要です。PostgreSQL に触れるテストは CI のサービスコンテナに対して実行され、ほとんどのユニットテストは
一時 SQLite データベースファイルを使うため外部サービスを必要としません。

## 開発コマンド

```bash
make check     # gofmt + go vet + go test
make test-race # go test -race -coverprofile=coverage.out ./...
make lint      # golangci-lint run ./...
make build     # bin/ にバイナリをコンパイル
```

## コードスタイル

Go API、モデル、トランザクション、テストの追加は[サービスの拡張](/ja/development)の実コード経路に従ってください。`.env.example` のコピーだけでは Go プロセスの環境変数は設定されません。起動は[クイックスタート](/ja/quickstart)を参照してください。

- フォーマットは `gofmt` で強制され、未フォーマットのコードは CI で失敗します。
- 静的解析は `go vet` と `golangci-lint`（設定は `.golangci.yml`）で実行されます。
- レイヤードアーキテクチャは意図的なものです — ビジネスルールは `internal/service` に置き、ハンドラーには置かないでください。

## エラーコードの追加

アプリケーションエラーコードは**公開契約**です：一度リリースした数値コードを別の意味で再利用してはいけません。
古いコードを使い回すのではなく、`internal/apierr/errors.go` に新しい定数を追加してください。

## プルリクエストチェックリスト

公開 API の変更では `/`、`/zh/`、`/ja/` の文書とフロントエンドリポジトリの OpenAPI 契約を揃えます。

1. 変更に対するテストを追加または更新する。
2. ローカルで `make check` を実行してグリーンを維持する。
3. レスポンスエンベロープ契約を壊さない — JSON フィールドのリネームは破壊的変更です。
4. 表層が変わる場合は API リファレンスと設定ドキュメントを更新する。

## リリース

リリースにタグを付けると、Docker ビルドが `git describe` 経由でバージョンを取得します。
