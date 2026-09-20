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

同梱の Compose 設定は API、PostgreSQL、Redis を**開発専用**の認証情報とシードデータで起動します。ソースから開発する場合は `.env.example` をプロジェクト直下の `.env` にコピーしてください。サーバーが自動で読み込み、既存のプロセス環境変数が優先されます。使用可能なデータベース URL とコマンドは[クイックスタート](https://fonghehe.github.io/vue-h5-template-business-service/ja/quickstart)を参照してください。

Compose の `business` サービスを起動せず、ソースから API を実行する場合：

```bash
cp .env.example .env # 初回のみ。専用 DB の DATABASE_URL を設定
POSTGRES_PORT=5433 docker compose up -d --wait postgres # 必要な場合のみ
go run ./cmd/server
```

サンプル `.env` はポート 5433 の専用開発 DB を使います。ローカル PostgreSQL が利用できる場合は `.env` の `DATABASE_URL` を専用 DB に設定し、Compose の起動を省略できます。`.env` も明示的な `DATABASE_URL` もない場合は既定の `localhost:5432` を使い、別アプリの DB に接続する可能性があります。起動のためにそのデータベースを移行・削除しないでください。

## 対話型 API ドキュメント

開発環境の Swagger UI は [http://localhost:8002/swagger/index.html](http://localhost:8002/swagger/index.html) から開けます。現在の OpenAPI 契約は `/openapi.yaml` です。ページは `github.com/swaggo/gin-swagger` で登録し、リソースは `github.com/swaggo/files` が提供します。別のドキュメントサーバーや CDN は不要です。既存の OpenAPI 3 契約を読み込むため、`swag init` で Swagger 2 文書を生成する必要はありません。`APP_ENV=production` ではこれらのルートを登録しません。

`POST /api/auth/login` で開発アカウント `user` / `123456` を使ってログインし、`data.accessToken` を **Authorize** に入力してください。管理 API は `admin` / `123456` を使います。決済はモックで、実際の課金は発生しません。

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

## アーキテクチャと本番運用

`Handler → Service → Repository → PostgreSQL` を維持し、明示的 HTTP DTO で契約を固定します。`make migrate` は順序移行、`make migrate-status` は適用状態を表示し、HTTP や seed を開始しません。staging/production は `AUTO_MIGRATE=false` が既定で、レプリカ開始前に移行します。独立 `compose.production.yml` は外部の専用 DB を利用します。[本番動作とアップグレード](docs/ja/production.md)を確認してください。更新セッションは単一使用とログアウト失効に対応し、旧 Refresh JWT は再ログインが必要です。
