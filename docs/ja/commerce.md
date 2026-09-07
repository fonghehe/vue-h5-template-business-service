# コマースフロー

コードに実装された `Product → SKU → Inventory → Cart → Coupon → Order → Payment → Cancel/Timeout` を追います。PostgreSQL を基準とする単一サービスであり、**実決済ゲートウェイ**や総合マーケットプレイスではありません。

## 商品、SKU、価格

`internal/model/model.go` は既存 H5 向け Product フィールド（`title`、`imgUrl`、文字列の `price` / `vipPrice`）を維持します。販売単位は `internal/model/commerce.go` の ProductSKU で、`skuCode`、JSON 属性、整数の最小通貨単位による価格、状態、Inventory 行を持ちます。管理者の `POST /api/admin/products/:id/skus` は SKU と初期在庫を一つのトランザクションで作成します。公開商品詳細には SKU と最新在庫が含まれます。

商品一覧はキーワード、カテゴリ、active SKU 価格範囲、ソート、上限付きページングに対応します。価格範囲は**同じ** SKU が満たす必要があります。任意の Redis はカタログと SKU だけをキャッシュし、`ProductService.Detail` は毎回 PostgreSQL から在庫を取得します。商品/SKU 更新時にはキャッシュを無効化します。旧 Product.Stock と文字列価格は注文に使用しません。

## カートとクーポン

認証後に `GET /api/cart`、`POST /api/cart/items`、`PUT /api/cart/items/:id`、`DELETE /api/cart/items/:id`、`DELETE /api/cart` を使用します。追加は `{"skuId":1,"quantity":2}` でその SKU の数量を設定し、更新は `{"quantity":3}` です。数量は 1–99、商品と SKU の両方が販売可能でなければなりません。カート行に**価格は保存しません**。

クーポンは `FIXED_DISCOUNT`、`PERCENTAGE`（basis points: `1000` = 10%）、`THRESHOLD_DISCOUNT` の三種類です。注文時に期間、最低金額、全体/ユーザー使用上限、任意の所有者を検証します。CouponUsage は注文待ちで `RESERVED`、決済後 `CONSUMED`、キャンセル/期限切れ後 `RELEASED` です。デモコード `WELCOME10` と `SAVE20` は `SEED=true` のときのみ投入され、公開のクーポン作成 API はありません。

## 注文作成

ログインして access token を取得し、`/api/product/detail?id=1` の実際の `skus[].id` を使います：

```bash
curl -X POST http://localhost:8002/api/cart/items \
  -H 'Authorization: Bearer <accessToken>' \
  -H 'Content-Type: application/json' \
  -d '{"skuId":1,"quantity":1}'

curl -X POST http://localhost:8002/api/orders \
  -H 'Authorization: Bearer <accessToken>' \
  -H 'Idempotency-Key: checkout-001' \
  -H 'Content-Type: application/json' \
  -d '{"couponCode":"WELCOME10"}'
```

例の `skuId` は詳細レスポンスの値に置き換えてください。クーポン不要なら `{}` を送ります。`Idempotency-Key` は必須で最大 128 文字。新しい注文には新しいキーを使います。初回成功は `201`、同一キー/リクエストの再送は元の注文を `200` で返し、異なるクーポンコードへの再利用は `409` です。

`internal/service/commerce_service.go` の `OrderService.Create` は PostgreSQL の一つのトランザクションで実行します：

```text
BEGIN
  カート取得（空または 100 項目超を拒否）
  SKU ID 順にロック/再読込し、サーバー価格を整数で計算
  クーポンをロックし使用上限を検証
  条件付き UPDATE で在庫を確保
  注文、OrderItem スナップショット、初期状態履歴を作成
  クーポンを確保しカートを消去
COMMIT（いずれかが失敗すれば全てロールバック）
```

OrderItem は `productName`、`skuName`、`unitPrice`、`quantity` を保存するため、その後の商品編集は過去の注文に影響しません。`payableAmount = originalAmount - discountAmount` はサーバーで計算します。DB の `(user_id, idempotency_key)` 一意索引が最終的な重複防止であり、Redis に正しさを依存しません。

## 在庫不変条件と状態遷移

`CommerceRepository.ReserveInventory` は `available >= quantity` を条件とする一つの SQL UPDATE と `RowsAffected` を使います。PostgreSQL は行更新ロック下で条件を判定し、DB 制約も `available >= 0`、`reserved >= 0` を要求します。決済成功は `reserved` を売上として減らし、キャンセル/期限切れは `available` に戻します。

| 元の状態 | 次の状態 | 発火条件 |
|---|---|---|
| `PENDING_PAYMENT` | `PAID` | 検証済み成功 Webhook |
| `PENDING_PAYMENT` | `CANCELLED` | 所有者によるキャンセル |
| `PENDING_PAYMENT` | `EXPIRED` | 期限切れ Worker |
| `PAID` | `PROCESSING` | 管理者 |
| `PROCESSING` | `COMPLETED` | 管理者 |

`internal/service/order_state.go` の `TransitionOrder` は他の遷移を `409` で拒否し、状態と**同一トランザクション**で記録する履歴を返します。`GET /api/orders/:id` は所有者限定で、他人の注文は `404` です。

## Mock 決済と重複 Webhook

`POST /api/orders/:id/payment` は所有する期限内の待機注文に Mock Payment を一つ作成または返します。**実際の課金や即時の PAID 化はしません**。`PaymentProvider` と `MockPaymentProvider` は `internal/service/payment.go` にあります。`POST /api/payments/mock/webhook` は `X-Mock-Signature` を必要とし、`MOCK_PAYMENT_WEBHOOK_SECRET` で正規化イベントを HMAC-SHA256 署名します。成功時には注文番号、参照、金額、状態を検証します。

`internal/service/payment_test.go` の `TestPaymentWebhookIsIdempotent` は実際の `provider.Sign(callback)` と同じイベントの十回送信を示します。`(provider, event_id)` 一意制約とペイロードハッシュにより同じ再送は安全で、同じ ID に違う内容は `409` です。失敗イベントは決済試行のみを失敗にし、注文は再試行/キャンセル/期限切れまで待機します。実決済導入時にはその事業者固有の署名、イベント、照合を設計してください。

## 期限切れと複数インスタンス

`cmd/server/main.go` は期限切れ Worker を起動します。`ORDER_EXPIRATION_INTERVAL` ごとにバッチを取得し、PostgreSQL の `FOR UPDATE SKIP LOCKED` で複数インスタンスに異なる注文を割り当てます。一つのトランザクションで `EXPIRED`、在庫・クーポン確保の解除、履歴追加を行います。状態ロックで二重解放を防ぎ、グレースフルシャットダウンで Worker も停止します。

## 保証のテスト

```bash
go test ./internal/service ./internal/repository
go test -race ./...
```

SQLite テストはロールバック、クーポン、冪等性、重複 Webhook、状態遷移を扱います。PostgreSQL 並行テストには `TEST_DATABASE_URL` が必要で、CI が設定します。`TestPostgresConcurrentOrdersNeverOversell` は在庫 10 に 100 人が同時注文し、10 件のみ成功し在庫が負にならないことを確認します。テスト構成は[開発](/ja/development)、運用上の制限は[デプロイ](/ja/deployment)を参照してください。
