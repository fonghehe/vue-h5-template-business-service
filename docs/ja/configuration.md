# 設定

すべての設定は環境変数から読み込まれます（必要に応じて `.env` ファイルで補完できます）。設定は**フェイルファスト**です：
無効または安全でない値は、デフォルトに静かにフォールバックせず、起動を中止します。

## 全リファレンス

| 変数 | デフォルト | 説明 |
|---|---|---|
| `APP_ENV` | `development` | `development`、`test`、`production` のいずれか。 |
| `PORT` | `8002` | HTTP 待ち受けポート。 |
| `LOG_LEVEL` | `info` | `debug`、`info`、`warn`、`error` のいずれか。 |
| `LOG_FORMAT` | `text` | ローカル開発は `text`、本番では `json`（必須）。 |
| `DATABASE_DRIVER` | `postgres` | `postgres` または `sqlite`（sqlite は隔離されたデモ・テスト専用）。 |
| `DATABASE_URL` | `postgres://…` | ドライバ固有の DSN。 |
| `DB_MAX_OPEN_CONNS` | `25` | データベースへの最大同時接続数。 |
| `DB_MAX_IDLE_CONNS` | `5` | プールに保持する最大アイドル接続数。 |
| `DB_CONN_MAX_LIFETIME` | `30m` | 接続を再利用できる時間。 |
| `AUTO_MIGRATE` | `true` | 起動時にスキーママイグレーションを実行。 |
| `SEED` | `true` | 空のデータベースにデモデータを投入。 |
| `JWT_SECRET` | dev placeholder | AI サービスと**必ず**一致させる。32 文字以上。 |
| `JWT_ISSUER` | `vue-h5-template` | `iss` クレーム。AI サービスと一致させる。 |
| `JWT_AUDIENCE` | `vue-h5-template-api` | `aud` クレーム。AI サービスと一致させる。 |
| `ACCESS_TOKEN_TTL` | `2h` | アクセストークンの有効期間。 |
| `REFRESH_TOKEN_TTL` | `336h` | リフレッシュトークンの有効期間（`ACCESS_TOKEN_TTL` より長く）。 |
| `REFRESH_COOKIE_NAME` | `vh5_refresh` | リフレッシュトークンを運ぶ HttpOnly Cookie。 |
| `REFRESH_COOKIE_SECURE` | `false` | リフレッシュ Cookie に `Secure` を付与（本番では `true` 必須）。 |
| `CORS_ORIGINS` | `http://localhost:5173,…` | カンマ区切りのブラウザオリジン許可リスト。 |
| `RATE_LIMIT_ENABLED` | `true` | IP 単位のトークンバケットを有効化。 |
| `RATE_LIMIT_RPS` | `20` | IP ごとの持続リクエスト数/秒。 |
| `RATE_LIMIT_BURST` | `40` | IP ごとのバケット容量。 |
| `SHUTDOWN_TIMEOUT` | `15s` | グレースフルシャットダウンの上限。 |
| `READ_TIMEOUT` / `WRITE_TIMEOUT` / `IDLE_TIMEOUT` | `15s` / `30s` / `60s` | HTTP サーバーのタイムアウト。 |

## 本番環境での検証

`APP_ENV=production` の場合、起動は以下のすべてが満たされない限り実行を拒否します：

- `JWT_SECRET` がデフォルトのプレースホルダーではなく、32 文字以上であること。
- `REFRESH_COOKIE_SECURE` が `true` であること。
- `CORS_ORIGINS` が明示的なオリジンを列挙していること（`*` やループバックオリジン（`localhost`、`127.0.0.1` など）は不可）。
- `LOG_FORMAT` が `json` であること。

これは意図的なものです：誤設定のサービスが起動してしまうのは、起動時にクラッシュするよりもはるかに危険です。

## 隔離実行のための SQLite

`DATABASE_DRIVER=sqlite` は、完全な PostgreSQL が不要なデモや隔離テスト向けにサポートされています。本番用途では
**想定されておらず**、本番では PostgreSQL を使用してください。

```bash
DATABASE_DRIVER=sqlite DATABASE_URL=/tmp/vue-h5-business.db make run
```
