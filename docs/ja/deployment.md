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
cp .env.example .env
docker compose up --build
```

API と PostgreSQL 17 が、永続ボリュームとヘルスチェック付きで起動します。API は `http://localhost:8002` で待ち受けます。

## 本番チェックリスト

- **すべてのシークレットを置き換える** — `JWT_SECRET`（32 文字以上、ランダム）と PostgreSQL のパスワード。
  デフォルトをそのまま出荷しないこと。
- **`APP_ENV=production` を設定** — 下記のフェイルファスト検証が有効になります。
- **TLS をアプリの上流で終端**（nginx、ロードバランサー、マネージドゲートウェイ）し、`REFRESH_COOKIE_SECURE=true` を設定。
- **`CORS_ORIGINS` を実際のフロントエンドドメインに固定**。本番起動は `*` とループバックオリジンを拒否します。
- **JSON ログを使う**（`LOG_FORMAT=json`）してコレクターに送る。
- **`JWT_SECRET`/`JWT_ISSUER`/`JWT_AUDIENCE` を AI サービスと同一に保つ**ことで、ここで発行したトークンがそこで検証されます。
- **ライブネス/レディネス**をそれぞれ `/health` と `/ready` に向ける。

## グレースフルシャットダウン

`SIGINT`/`SIGTERM` でサーバーは接続の受け付けを停止し、処理中のリクエストを `SHUTDOWN_TIMEOUT` までドレインし、
データベースプールを閉じて終了します。

## CI

GitHub Actions は `main` へのすべての PR と push で実行されます：フォーマットチェック、`go vet`、カバレッジ付き
`go test -race`、`golangci-lint`、PostgreSQL サービスコンテナに対する Docker イメージビルド。
