# API リファレンス

ビジネスサービスは JSON HTTP API を公開しています。エラーや未知のルートを含むすべてのエンドポイントが、同じエンベロープを返します。

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

アクセストークンは JWT（`HS256`）です。リフレッシュトークンは認証パスにスコープされた `HttpOnly` クッキー
（`vh5_refresh`）として配信されるため、JavaScript から読み取られることはありません。

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

クエリパラメータ：`page`（デフォルト `1`）、`pageSize`（デフォルト `10`、最大 `50`）、`keyword`、`sort`。

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

金額（`price`、`vipPrice`）はデータベースとクライアントの間で丸めが発生しないよう、**文字列**として保存・返却されます。

## 管理者

運用者向けルートには `admin` ロールが必要です。商品を変更できるのはこれらのルートだけです。

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/admin/products` | 非表示の商品も含むカタログのページ（`status`、`featured` フィルター）。 |
| `POST` | `/api/admin/products` | 商品を作成。 |
| `GET` | `/api/admin/products/:id` | ステータスに関わらず商品を取得。 |
| `PATCH` | `/api/admin/products/:id` | 商品を部分的に更新。 |
| `DELETE` | `/api/admin/products/:id` | 商品をソフト削除。 |

### 商品ペイロード

`title`、`imgUrl`、`price`、`vipPrice`、`shopDesc`、`delivery`、`shopName`、`description`、`stock`、`status`
（`draft` | `on_sale` | `sold_out`）、`featured`。`PATCH` では省略されたフィールドは変更されません。

## ヘルス

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/health` | ライブネス。データベースには一切触れません。 |
| `GET` | `/ready` | レディネス。データベースに到達できない間は失敗します。 |
