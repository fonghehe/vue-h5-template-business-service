# デプロイ

## イメージのビルド

イメージはマルチステージで、**非 root** ユーザーとして動作し、ビルドバージョンがコンパイル時に注入されます。

```bash
docker build --build-arg VERSION=$(git describe --tags --always --dirty) \
  -t vue-h5-template-business-service:latest .
```

または make 経由：

```bash
make docker
```

## docker compose で実行

```bash
docker compose up -d --build
```

既存 Compose は**開発用**で、API、永続ボリューム付き PostgreSQL 17、キャッシュ専用で永続化しない Redis 8 を起動します。開発資格情報と `APP_ENV=development` が固定されており、`.env.example` をコピーしても本番構成にはなりません。API は `http://localhost:8002`。`POSTGRES_PORT=5433 docker compose up -d --build` は DB のホストポートだけを変更します。

## 本番チェックリスト

- **シークレットを置き換える** — `JWT_SECRET`、`MOCK_PAYMENT_WEBHOOK_SECRET`（各 32 文字以上）、PostgreSQL 資格情報。Compose の既定値は使いません。
- **`APP_ENV=production` を設定** — 下記のフェイルファスト検証が有効になります。
- **TLS をアプリの上流で終端**（nginx、ロードバランサー、マネージドゲートウェイ）し、`REFRESH_COOKIE_SECURE=true` を設定。
- **`CORS_ORIGINS` を実際のフロントエンドドメインに固定**。本番起動は `*` とループバックオリジンを拒否します。
- **JSON ログを使う**（`LOG_FORMAT=json`）してコレクターに送る。
- **デモデータを無効化**（`SEED=false`）。本番では `true` の起動を拒否します。実ユーザー/クーポンは承認済みの運用経路で準備してください。公開登録やクーポン管理 API はありません。
- **マイグレーションを調整**：`AUTO_MIGRATE=true` は起動時に記録済み移行を実行します。複数レプリカでは移行担当を一つにするか事前実行します。既存 Product は Seed 無効でもデータ移行でデフォルト SKU/在庫を得ます。
- **PostgreSQL を基準にする**：Redis は任意の商品キャッシュです。IP 制限はインスタンス内だけなので、複数レプリカには共有エッジ制限を設けます。
- **`/metrics` を必要に応じて制限**：ルート自体に JWT はありません。HTTP 件数/時間、注文作成/支払/期限切れ、在庫確保失敗を監視できます。
- **決済は Mock**：HMAC コールバックは実際の資金決済ではありません。実事業者の署名、イベント意味、照合が別途必要です。
- **`JWT_SECRET`/`JWT_ISSUER`/`JWT_AUDIENCE` を AI サービスと同一に保つ**ことで、ここで発行したトークンがそこで検証されます。
- **ライブネス/レディネス**をそれぞれ `/health` と `/ready` に向ける。

## グレースフルシャットダウン

`SIGINT`/`SIGTERM` でサーバーは接続の受け付けを停止し、処理中のリクエストを `SHUTDOWN_TIMEOUT` までドレインし、
期限切れ Worker、キャッシュ、DB プールを閉じます。次のインスタンスが期限処理を引き継ぎ、PostgreSQL `SKIP LOCKED` が二重取得を防ぎます。

## CI

GitHub Actions は `main` へのすべての PR と push で実行されます：フォーマットチェック、`go vet`、カバレッジ付き
`go test -race`、`golangci-lint`、PostgreSQL サービスコンテナを使う並行テスト、Docker ビルドです。別の `deploy-docs.yml` は pnpm で三言語 VitePress を型検査・ビルドし GitHub Pages にアップロードします。ビルド成功だけでは Pages の公開状態を証明しません。
