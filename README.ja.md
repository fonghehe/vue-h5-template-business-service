# vue-h5-template-business-service

[English](./README.md) | [简体中文](./README.zh-CN.md) | 日本語

vue-h5-template 向けの Go/Gin コマース API です。このリポジトリは認証、ユーザープロフィール、お気に入り、商品カタログ、SKU 在庫、カート、クーポン、注文、モック決済状態を担当します。H5 UI とストリーミング AI サービスは別のリポジトリにあります。

取引フローは `商品 → SKU → 在庫 → カート → クーポン → 注文 → 決済 → キャンセル/期限切れ` です。PostgreSQL がデータの基準であり、注文作成、在庫・クーポンの予約、注文時点のスナップショット保存、カートのクリアは一つのトランザクションで確定します。データベースの一意制約で注文の再試行と決済コールバックの冪等性を守り、期限切れワーカーが未決済注文の予約を解放します。任意の Redis は商品カタログのみをキャッシュし、在庫や注文の正本にはしません。

## 技術スタック

Go 1.25、Gin、GORM、PostgreSQL 17、任意の Redis、JWT、Prometheus、VitePress ドキュメント。SQLite は隔離テスト専用で、本番環境では使いません。決済は署名付きの**モックプロバイダー**であり、実際の決済事業者とは接続していません。

## クイックスタート

```bash
docker compose up -d --build
curl http://localhost:8002/health
curl http://localhost:8002/ready
curl 'http://localhost:8002/api/product/list?page=1&pageSize=10'
```

同梱の Compose 設定は API、PostgreSQL、Redis を**開発専用**の認証情報とシードデータで起動します。Go プロセスを直接実行する場合は環境変数を明示的に設定してください。`.env` は自動で読み込まれません。使用可能なデータベース URL とコマンドは[クイックスタート](https://fonghehe.github.io/vue-h5-template-business-service/ja/quickstart)を参照してください。

Compose の `business` サービスを起動せず、ソースから API を実行する場合：

```bash
POSTGRES_PORT=5433 docker compose up -d postgres
export DATABASE_URL='postgres://vue_h5:vue_h5_local@127.0.0.1:5433/vue_h5_business?sslmode=disable'
go run ./cmd/server
```

接続先を指定しない Go コマンドは既定の `localhost:5432` を使い、別アプリのデータベースに接続する可能性があります。起動のためにそのデータベースを移行・削除しないでください。

## チェック

```bash
go test ./...
go test -race ./...
go vet ./...
make lint
make build
```

PostgreSQL での「100 人が在庫 10 件を同時購入する」テストには `TEST_DATABASE_URL` が必要です。CI は隔離されたテスト用データベースを提供します。詳細なドキュメントは三言語で公開しています。

- [English documentation](https://fonghehe.github.io/vue-h5-template-business-service/)
- [中文文档](https://fonghehe.github.io/vue-h5-template-business-service/zh/)
- [日本語ドキュメント](https://fonghehe.github.io/vue-h5-template-business-service/ja/)

ローカルでドキュメントを起動するには `cd docs && pnpm install --frozen-lockfile && pnpm docs:dev` を実行します。

## ライセンス

[MIT](./LICENSE)
