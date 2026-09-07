# API リファレンス

業務 JSON ルートはエラーや未知のパスも共通エンベロープです。`/metrics` は Prometheus テキスト、CORS プリフライトは HTTP 204 で、JSON ではありません。

## レスポンスエンベロープ

```jsonc
// 成功
{ "code": 0, "message": "ok", "data": { "…": "…" }, "error": null, "requestId": "01J…" }

// 失敗
{ "code": 4010, "message": "username or password is incorrect", "data": null, "error": null, "requestId": "01J…" }
```

フロントエンドは `code === 0` で分岐します。HTTP ステータスはエラーカテゴリを反映しますが、アプリケーションの
`code` が信頼できるシグナルです。

## エラーコード

| コード | 意味 | HTTP |
|---|---|---|
| `0` | 成功 | 200 / 201 |
| `4000` | 不正なリクエスト | 400 |
| `4001` | バリデーション失敗 | 422 |
| `4010` | 未認証 | 401 |
| `4030` | 禁止 | 403 |
| `4040` | 見つからない | 404 |
| `4090` | 競合 | 409 |
| `4130` | ペイロード過大 | 413 |
| `4290` | レート制限 | 429 |
| `5000` | 内部エラー | 500 |
| `5030` | 利用不可 | 503 |

## 認証

アクセストークンは JWT（`HS256`）です。リフレッシュトークンは既定で `/api/auth` に限定された `HttpOnly` Cookie（`vh5_refresh`）として配信され、JavaScript から読めません。ログアウトは Cookie を消去しますが、発行済みのステートレス access token をサーバー側で即時失効しません。

| メソッド | パス | 認証 | 説明 |
|---|---|---|---|
| `POST` | `/api/auth/login` | — | 認証情報をアクセストークンと交換し、リフレッシュ Cookie を設定。 |
| `POST` | `/api/auth/refresh` | リフレッシュ Cookie またはボディ | 期限切れのアクセストークンをローテーション。 |
| `POST` | `/api/auth/logout` | — | リフレッシュ Cookie を消去。 |

### POST /api/auth/login

**ボディ** `{ "username": string, "password": string }`

**レスポンス `data`**

```jsonc
{
  "id": 1,
  "username": "user",
  "realName": "テストユーザー",
  "avatar": "https://…",
  "roles": ["user"],
  "accessToken": "eyJ…",
  "expiresIn": 7200
}
```

## ユーザー

| メソッド | パス | 認証 | 説明 |
|---|---|---|---|
| `GET` | `/api/user/info` | Bearer | 認証済みアカウント。 |
| `GET` | `/api/user/favorites` | Bearer | お気に入り登録した商品（新しい順）。 |

## 商品

| メソッド | パス | 認証 | 説明 |
|---|---|---|---|
| `GET` | `/api/product/list` | — | 公開カタログの 1 ページ。 |
| `GET` | `/api/product/detail?id=1` | — | 販売中の商品 1 件。 |
| `POST` | `/api/product/favorite` | Bearer | 商品のお気に入り登録 / 解除。 |

### GET /api/product/list

クエリ：`page`（既定 `1`）、`pageSize`（既定 `10`、最大 `50`）、`keyword`、`category`、`priceMin`、`priceMax`、`sort`（`newest`、`sales`、`price_asc`、`price_desc`、不明な値は featured 優先）。無効・非正数のページ指定は既定値に戻り、過大な pageSize は 50 に丸められます。価格は非負整数の最小通貨単位で、上下限は**同じ** active SKU が満たします。

**レスポンス `data`** — ページエンベロープ：

```jsonc
{
  "items": [
    { "id": 1, "title": "…", "imgUrl": "…", "price": "388", "vipPrice": "378",
      "shopDesc": "自営", "delivery": "メーカー配送", "shopName": "…", "description": "…" }
  ],
  "total": 14,
  "page": 1,
  "pageSize": 10,
  "hasMore": true
}
```

旧 `price` / `vipPrice` の文字列は H5 表示互換用です。注文価格はサーバーが再取得した SKU の `int64` 最小通貨単位です。

`GET /api/product/detail?id=<productId>` は販売中 Product と `skus`（`skuCode`、`name`、JSON `attributes`、整数 `price`/`originalPrice`、`status`）、最新 `inventory`（`available`、`reserved`、`version`）を返します。カタログが Redis にあっても在庫は PostgreSQL から読みます。公開一覧は SKU をプリロードしません。

## カート、注文、支払い

| メソッド | パス | 説明 |
|---|---|---|
| `GET` / `DELETE` | `/api/cart` | 現在のユーザーのカートを取得 / クリア。 |
| `POST` | `/api/cart/items` | SKU を追加（数量 1–99）。クライアント価格は受け付けません。 |
| `PUT` / `DELETE` | `/api/cart/items/:id` | 数量変更 / 自分の項目を削除。 |
| `GET` / `POST` | `/api/orders` | 自分の注文一覧 / `Idempotency-Key` 付き注文作成。 |
| `GET` | `/api/orders/:id` | 所有者のみ取得可能。別ユーザーの注文は 404。 |
| `POST` | `/api/orders/:id/cancel` | 支払い待ち注文を取り消し、予約を解放。 |
| `POST` | `/api/orders/:id/payment` | 所有する期限内の待機注文に Mock Payment を作成/返却。実課金なし。 |
| `POST` | `/api/payments/mock/webhook` | HMAC 検証と DB 重複排除を行うコールバック。 |

注文作成は 1 つの PostgreSQL トランザクションで最新 SKU 価格の取得、クーポン検証、在庫予約、
注文スナップショット、クーポン予約、カート消去を行います。SKU・注文・支払い金額は `int64` のセント単位です。

カート追加 body は `{"skuId":1,"quantity":2}` で、その SKU の数量を**設定**します（既存数量に 2 を加算しません）。更新は `{"quantity":3}`。両方とも 1–99 と販売可能な Product/SKU が必要で、注文時の異なるカート項目数は最大 100 です。

注文 body は `{}` または `{"couponCode":"WELCOME10"}`。最大 128 文字の `Idempotency-Key` が必須です。初回成功は HTTP 201、同一キー/要求の再送は元注文を HTTP 200、同じキーで別クーポンを指定すると 409。注文一覧は `page`/`pageSize`（最大 50）に対応します。

## Mock 決済 Webhook

`POST /api/payments/mock/webhook` は `eventId`、`orderNo`、`reference`、整数 `amount`、`status`（`SUCCESS` / `FAILURE`）を受けます。`X-Mock-Signature` は `MockPaymentProvider.Sign` による正規化フィールドの HMAC です。`internal/service/payment_test.go` に署名例があります。DB の `(provider,eventId)` 一意制約は重複支払いを防ぎ、同じ ID の別内容は 409。`FAILURE` は決済試行だけを失敗にして注文は待機状態です。実決済ではありません。

## 管理者

運用者向けルートには `admin` ロールが必要です。商品を変更できるのはこれらのルートだけです。

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/admin/products` | 非表示の商品も含むカタログのページ（`status`、`featured` フィルター）。 |
| `POST` | `/api/admin/products` | 商品を作成。 |
| `GET` | `/api/admin/products/:id` | ステータスに関わらず商品を取得。 |
| `PATCH` | `/api/admin/products/:id` | 商品を部分的に更新。 |
| `DELETE` | `/api/admin/products/:id` | 商品をソフト削除。 |
| `POST` | `/api/admin/products/:id/skus` | SKU と初期在庫を同一トランザクションで作成。 |
| `PUT` | `/api/admin/skus/:id` | SKU を更新。 |
| `PUT` | `/api/admin/orders/:id/status` | `PAID → PROCESSING → COMPLETED` を進行。 |

### 商品ペイロード

`name`、`categoryId`、`brand`、`cover`、`title`、`imgUrl`、`price`、`vipPrice`、`shopDesc`、`delivery`、`shopName`、`description`、`stock`、`status`（`draft` | `on_sale` | `sold_out`）、`featured`。作成時は旧 `title`、`imgUrl`、文字列 `price`/`vipPrice`、`shopName` が必要です。`PATCH` で省略した値は変わりません。`stock` は旧互換用で、正規の在庫ではありません。

SKU 作成 body は `skuCode`、`name`、`attributes`（文字列 map）、`price`、`originalPrice`（整数）、`status`（`active` | `inactive`）、`available`（初期在庫）。SKU PUT は同じ項目を検証しますが在庫を直接変更しません。公開クーポン管理 API はありません。[取引フロー](/ja/commerce)にトランザクションを説明します。

## ヘルス

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/health` | ライブネス。データベースには一切触れません。 |
| `GET` | `/ready` | レディネス。データベースに到達できない間は失敗します。 |
| `GET` | `/metrics` | Prometheus の HTTP・注文・在庫メトリクス。 |
| `GET` | `/api/health` | 互換ライブネスパス。 |
