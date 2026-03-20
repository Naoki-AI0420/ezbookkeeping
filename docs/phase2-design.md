# Phase 2 設計書: ezbookkeeping SaaS化 — Stripe統合 & ランディングページ

## 概要

ezbookkeeping（Go + Vue 3）をマルチテナントSaaS家計簿サービスとして提供するための設計。
Open SaaS テンプレート（Wasp + React + Stripe）の課金・LP設計パターンを参考に、ezbookkeepingの既存アーキテクチャ（Gin / XORM / Vue 3 + Vuetify）に統合する。

---

## 1. 料金プラン設計

### 1.1 プラン一覧

| プラン | 月額（税込） | 年額（税込） | 対象 |
|--------|-------------|-------------|------|
| **Free** | ¥0 | ¥0 | 個人・お試し |
| **Pro** | ¥980 | ¥9,800（2ヶ月分お得） | 個人・本格利用 |
| **Business** | ¥2,980 | ¥29,800（2ヶ月分お得） | 事業者・複数口座管理 |

### 1.2 機能制限マトリクス

| 機能 | Free | Pro | Business |
|------|------|-----|----------|
| 口座数 | 3 | 20 | 無制限 |
| 月間取引登録数 | 50 | 無制限 | 無制限 |
| カテゴリ数 | 10 | 無制限 | 無制限 |
| タグ数 | 5 | 無制限 | 無制限 |
| データエクスポート (CSV/TSV) | ✕ | ✓ | ✓ |
| データインポート | ✕ | ✓ | ✓ |
| レシート画像認識 (AI) | ✕ | 月10回 | 月100回 |
| 統計・インサイト | 基本のみ | 全機能 | 全機能 |
| 為替レート自動更新 | ✓ | ✓ | ✓ |
| 2FA | ✓ | ✓ | ✓ |
| API トークン | ✕ | 3個 | 10個 |
| MCP サーバー | ✕ | ✓ | ✓ |
| メールサポート | ✕ | ✓ | 優先対応 |

---

## 2. Stripe Checkout 統合方針

### 2.1 全体フロー

```
ユーザー → 料金ページ → プラン選択 → Stripe Checkout (hosted) → Webhook → DB更新 → 利用開始
                                            ↓
                                    Stripe Customer Portal → プラン変更/解約
```

### 2.2 Stripe側の設定

1. **Products & Prices（Stripe Dashboard で作成）**
   - `ezbook_pro_monthly` — ¥980/月 recurring
   - `ezbook_pro_yearly` — ¥9,800/年 recurring
   - `ezbook_business_monthly` — ¥2,980/月 recurring
   - `ezbook_business_yearly` — ¥29,800/年 recurring

2. **Webhook Events（登録対象）**
   - `invoice.paid` — 支払い成功時にサブスクリプション状態を更新
   - `customer.subscription.updated` — プラン変更・ステータス変更
   - `customer.subscription.deleted` — 解約完了

3. **Customer Portal**
   - プラン変更、支払い方法変更、請求書確認、解約をStripe側UIで処理

### 2.3 環境変数

```ini
# conf/ezbookkeeping.ini [stripe] セクション
[stripe]
enable_stripe = true
api_key = sk_live_xxx
webhook_secret = whsec_xxx
pro_monthly_price_id = price_xxx
pro_yearly_price_id = price_xxx
business_monthly_price_id = price_xxx
business_yearly_price_id = price_xxx
success_url = https://app.example.com/checkout?status=success
cancel_url = https://app.example.com/checkout?status=canceled
```

---

## 3. 技術的統合方針（Go Backend）

### 3.1 新規パッケージ構成

```
pkg/
├── payment/
│   ├── stripe_client.go        # Stripe SDK初期化
│   ├── checkout.go             # Checkout Session作成
│   ├── webhook.go              # Webhook受信・検証・処理
│   ├── portal.go               # Customer Portal URL生成
│   └── plans.go                # プラン定義・マッピング
│
├── models/
│   └── subscription.go         # サブスクリプションモデル（新規）
│
├── api/
│   └── subscriptions.go        # サブスクリプションAPI（新規）
│
├── middlewares/
│   └── subscription_guard.go   # プラン制限チェックミドルウェア（新規）
│
└── services/
    └── subscriptions.go        # サブスクリプション業務ロジック（新規）
```

### 3.2 データモデル追加

```go
// pkg/models/subscription.go
type UserSubscription struct {
    Uid                  int64  `xorm:"PK"`               // users.uid と 1:1
    StripeCustomerId     string `xorm:"VARCHAR(255) INDEX"`
    SubscriptionPlan     string `xorm:"VARCHAR(50)"`       // "free", "pro", "business"
    SubscriptionStatus   string `xorm:"VARCHAR(50)"`       // "active", "past_due", "canceled", "deleted"
    StripePriceId        string `xorm:"VARCHAR(255)"`
    StripeSubscriptionId string `xorm:"VARCHAR(255) INDEX"`
    BillingCycle         string `xorm:"VARCHAR(20)"`       // "monthly", "yearly"
    CurrentPeriodEnd     int64  `xorm:"BIGINT"`
    DatePaid             int64  `xorm:"BIGINT"`
    CreatedUnixTime      int64  `xorm:"BIGINT NOT NULL"`
    UpdatedUnixTime      int64  `xorm:"BIGINT NOT NULL"`
}
```

### 3.3 User モデル拡張

既存の `User` モデルに直接カラムを追加せず、`UserSubscription` テーブルを別途作成し `Uid` で紐づける。
これにより既存のユーザー管理ロジックへの影響を最小化する。

### 3.4 API エンドポイント（新規）

```
POST /api/v1/subscription/checkout.json        # Stripe Checkout Session作成
GET  /api/v1/subscription/status.json          # 現在のサブスクリプション状態取得
POST /api/v1/subscription/portal.json          # Customer Portal URL取得
POST /api/stripe/webhook                        # Stripe Webhook（JWT不要、署名検証）
```

### 3.5 サブスクリプション制限ミドルウェア

```go
// pkg/middlewares/subscription_guard.go
//
// 既存APIルートにミドルウェアとして挿入し、
// ユーザーのプランに応じて機能制限を適用する。
//
// 例: 口座作成時
//   Free: 口座数 <= 3 → 許可、超過 → 402 Payment Required
//   Pro:  口座数 <= 20 → 許可
//   Business: 制限なし

func SubscriptionGuard(feature string, limit int) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. JWTからユーザーID取得
        // 2. UserSubscriptionテーブルからプラン取得
        // 3. 機能制限テーブルと照合
        // 4. 制限超過 → 402 + アップグレード案内
        // 5. OK → c.Next()
    }
}
```

### 3.6 Stripe Go SDK

```go
// go.mod に追加
require github.com/stripe/stripe-go/v82
```

Open SaaS テンプレートの `checkoutUtils.ts` パターンを Go に移植:

```go
// pkg/payment/checkout.go
func CreateCheckoutSession(user *models.User, priceId string) (*stripe.CheckoutSession, error) {
    // 1. ensureStripeCustomer: 既存顧客を検索 or 新規作成
    // 2. checkout.Session作成:
    //    - Mode: "subscription"
    //    - LineItems: [{Price: priceId, Quantity: 1}]
    //    - CustomerEmail or Customer ID
    //    - SuccessURL / CancelURL
    //    - AllowPromotionCodes: true
    //    - AutomaticTax: {Enabled: true}
    // 3. Session URLを返却
    return session, nil
}
```

---

## 4. 技術的統合方針（Vue 3 Frontend）

### 4.1 新規ページ・コンポーネント

```
src/
├── views/
│   └── base/
│       ├── PricingPage.vue          # 料金プランページ（LP兼用）
│       ├── CheckoutResultPage.vue   # Checkout完了/キャンセル
│       └── SubscriptionPage.vue     # サブスクリプション管理
│
├── components/
│   └── base/
│       └── common/
│           ├── PlanCard.vue         # プランカード
│           └── UpgradeDialog.vue    # アップグレード促進ダイアログ
│
├── stores/
│   └── subscription.ts             # サブスクリプション状態管理（Pinia）
│
└── router/
    └── desktop.ts                   # 新規ルート追加
```

### 4.2 フロントエンドフロー

1. **料金ページ** (`/pricing`)
   - 3カラムのプランカード表示（Free / Pro / Business）
   - 月額/年額トグル切り替え
   - ログイン済み → 「プランを購入」ボタン → POST `/api/v1/subscription/checkout.json`
   - 未ログイン → 「ログインして購入」→ ログインページへリダイレクト

2. **Checkout完了** (`/checkout?status=success`)
   - サブスクリプション状態をポーリング（Webhook反映待ち）
   - 完了 → ダッシュボードへリダイレクト

3. **サブスクリプション管理** (`/subscription`)
   - 現在のプラン・次回請求日表示
   - 「プランを変更」→ Stripe Customer Portal へリダイレクト
   - 「解約」→ Stripe Customer Portal へリダイレクト

4. **制限到達時**
   - 402レスポンス検知 → `UpgradeDialog.vue` を表示
   - 現在のプランと推奨プランを案内

---

## 5. ランディングページ設計

### 5.1 構成（日本語、家計簿SaaS訴求）

```
┌─────────────────────────────────────┐
│ ヘッダー                              │
│   ロゴ | 機能 | 料金 | ログイン | 新規登録 │
├─────────────────────────────────────┤
│ ヒーローセクション                      │
│   「シンプルで賢い家計管理、はじめよう」    │
│   サブ: 無料から始められる              │
│   CTA: 「無料で始める」ボタン            │
│   スクリーンショット画像                 │
├─────────────────────────────────────┤
│ 特徴セクション（3×2 グリッド）           │
│   📊 見やすいダッシュボード              │
│   🏦 複数口座・複数通貨対応             │
│   📱 スマホ対応（PWA）                 │
│   🔒 2段階認証で安心セキュリティ         │
│   📸 レシート撮影で自動入力（AI）        │
│   📈 グラフで支出傾向を可視化           │
├─────────────────────────────────────┤
│ 料金プランセクション                    │
│   Free / Pro / Business カード        │
│   月額/年額トグル                      │
├─────────────────────────────────────┤
│ FAQ セクション                        │
│   - 無料プランでどこまで使える？          │
│   - いつでもプラン変更できる？            │
│   - データのセキュリティは？              │
│   - 解約はすぐできる？                  │
├─────────────────────────────────────┤
│ CTA セクション                        │
│   「今すぐ無料で家計管理を始めよう」       │
│   CTA: 「無料アカウントを作成」          │
├─────────────────────────────────────┤
│ フッター                              │
│   利用規約 | プライバシーポリシー | お問い合わせ│
└─────────────────────────────────────┘
```

### 5.2 実装方針

- ezbookkeeping の既存 `index.html` をLP化（未ログイン時に表示）
- ログイン済みユーザーは従来どおり `/desktop` or `/mobile` へリダイレクト
- Vue 3 + Vuetify でLP実装（既存のデザインシステムを活用）
- OGP / SEO メタタグ設定（日本語）
- Google Analytics or Plausible 連携（Open SaaSテンプレート参考）

### 5.3 SEOキーワード

- 家計簿アプリ、家計管理、支出管理、複数口座管理、レシート読み取り、無料家計簿

---

## 6. Docker Compose 構成（本番向け）

### 6.1 構成図

```
                    ┌─────────────┐
                    │   Caddy     │ :443 (HTTPS自動証明書)
                    │  (Reverse   │ :80  (HTTP→HTTPS redirect)
                    │   Proxy)    │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ ezbookkeeping│ :8080
                    │   (Go+Vue)  │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
       ┌──────▼──────┐ ┌──▼───┐ ┌─────▼─────┐
       │  PostgreSQL  │ │MinIO │ │   Redis    │
       │   :5432      │ │:9000 │ │   :6379    │
       └─────────────┘ └──────┘ └───────────┘
                                (セッションキャッシュ
                                 ※将来拡張用)
```

### 6.2 docker-compose.yml

```yaml
version: "3.8"

services:
  # --- Reverse Proxy ---
  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./docker/Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      - app

  # --- Application ---
  app:
    build:
      context: .
      dockerfile: Dockerfile
    restart: unless-stopped
    expose:
      - "8080"
    environment:
      EBK_SERVER_DOMAIN: "${DOMAIN}"
      EBK_SERVER_ROOT_URL: "https://${DOMAIN}/"
      EBK_DATABASE_TYPE: "postgres"
      EBK_DATABASE_HOST: "postgres:5432"
      EBK_DATABASE_NAME: "ezbookkeeping"
      EBK_DATABASE_USER: "${DB_USER}"
      EBK_DATABASE_PASSWD: "${DB_PASSWD}"
      EBK_STORAGE_TYPE: "minio"
      EBK_STORAGE_MINIO_ENDPOINT: "minio:9000"
      EBK_STORAGE_MINIO_ACCESS_KEY_ID: "${MINIO_ACCESS_KEY}"
      EBK_STORAGE_MINIO_SECRET_ACCESS_KEY: "${MINIO_SECRET_KEY}"
      EBK_STRIPE_ENABLE_STRIPE: "true"
      EBK_STRIPE_API_KEY: "${STRIPE_API_KEY}"
      EBK_STRIPE_WEBHOOK_SECRET: "${STRIPE_WEBHOOK_SECRET}"
      EBK_STRIPE_PRO_MONTHLY_PRICE_ID: "${STRIPE_PRO_MONTHLY_PRICE_ID}"
      EBK_STRIPE_PRO_YEARLY_PRICE_ID: "${STRIPE_PRO_YEARLY_PRICE_ID}"
      EBK_STRIPE_BUSINESS_MONTHLY_PRICE_ID: "${STRIPE_BUSINESS_MONTHLY_PRICE_ID}"
      EBK_STRIPE_BUSINESS_YEARLY_PRICE_ID: "${STRIPE_BUSINESS_YEARLY_PRICE_ID}"
    volumes:
      - app_data:/ezbookkeeping/data
      - app_log:/ezbookkeeping/log
    depends_on:
      postgres:
        condition: service_healthy
      minio:
        condition: service_started

  # --- Database ---
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ezbookkeeping
      POSTGRES_USER: "${DB_USER}"
      POSTGRES_PASSWORD: "${DB_PASSWD}"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER}"]
      interval: 5s
      timeout: 5s
      retries: 5

  # --- Object Storage ---
  minio:
    image: minio/minio:latest
    restart: unless-stopped
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: "${MINIO_ACCESS_KEY}"
      MINIO_ROOT_PASSWORD: "${MINIO_SECRET_KEY}"
    volumes:
      - minio_data:/data

volumes:
  caddy_data:
  caddy_config:
  app_data:
  app_log:
  postgres_data:
  minio_data:
```

### 6.3 Caddyfile

```
{$DOMAIN} {
    reverse_proxy app:8080

    # Stripe Webhook はリクエストボディを変更しない
    @stripe path /api/stripe/webhook
    handle @stripe {
        reverse_proxy app:8080
    }
}
```

### 6.4 .env（テンプレート）

```env
DOMAIN=kakeibo.example.com
DB_USER=ezbookkeeping
DB_PASSWD=<strong-password>
MINIO_ACCESS_KEY=<minio-access-key>
MINIO_SECRET_KEY=<minio-secret-key>
STRIPE_API_KEY=sk_live_xxx
STRIPE_WEBHOOK_SECRET=whsec_xxx
STRIPE_PRO_MONTHLY_PRICE_ID=price_xxx
STRIPE_PRO_YEARLY_PRICE_ID=price_xxx
STRIPE_BUSINESS_MONTHLY_PRICE_ID=price_xxx
STRIPE_BUSINESS_YEARLY_PRICE_ID=price_xxx
```

---

## 7. MVP スコープ（最小限で課金開始できる範囲）

### 7.1 MVP に含めるもの

| # | タスク | 優先度 | 工数目安 |
|---|--------|--------|----------|
| 1 | **UserSubscription モデル + マイグレーション** | 必須 | S |
| 2 | **Stripe Checkout Session API** | 必須 | M |
| 3 | **Stripe Webhook 受信・処理** | 必須 | M |
| 4 | **Customer Portal URL API** | 必須 | S |
| 5 | **サブスクリプション状態取得 API** | 必須 | S |
| 6 | **口座数制限ミドルウェア（Free: 3）** | 必須 | S |
| 7 | **月間取引数制限ミドルウェア（Free: 50）** | 必須 | M |
| 8 | **フロントエンド: 料金ページ** | 必須 | M |
| 9 | **フロントエンド: Checkout完了画面** | 必須 | S |
| 10 | **フロントエンド: サブスクリプション管理画面** | 必須 | S |
| 11 | **フロントエンド: アップグレード促進ダイアログ** | 必須 | S |
| 12 | **ランディングページ（最小構成）** | 必須 | L |
| 13 | **Docker Compose 本番構成** | 必須 | M |
| 14 | **Stripe テスト環境での E2E 動作確認** | 必須 | M |

**工数: S=0.5日, M=1-2日, L=3-5日**
**MVP 合計目安: 2-3週間**

### 7.2 MVP に含めないもの（Phase 3 以降）

- データエクスポート/インポート制限の実装
- AI レシート認識の回数制限
- API トークン数制限
- MCP サーバー制限
- 管理者ダッシュボード（売上・ユーザー統計）
- メール通知（解約リマインダー、支払い失敗通知）
- Google Analytics / Plausible 連携
- SEO 最適化（OGP、構造化データ）
- 利用規約・プライバシーポリシーページ
- お問い合わせフォーム

### 7.3 MVP リリース判定基準

1. Free プランでユーザー登録 → 口座3つまで作成可能
2. 料金ページから Pro/Business プランを Stripe Checkout で購入可能
3. 購入後、制限が解除される
4. Stripe Customer Portal でプラン変更・解約が可能
5. 解約後、次の請求期間終了時に Free プランに戻る
6. Docker Compose で `docker compose up -d` 一発でデプロイ可能

---

## 8. 実装順序（推奨）

```
Week 1: バックエンド基盤
  ├── UserSubscription モデル + マイグレーション
  ├── Stripe SDK 統合 + クライアント初期化
  ├── Checkout Session API
  ├── Webhook 受信・署名検証・DB更新
  └── Customer Portal API

Week 2: 制限 + フロントエンド
  ├── サブスクリプション制限ミドルウェア（口座数・取引数）
  ├── フロントエンド: 料金ページ
  ├── フロントエンド: Checkout完了 + サブスクリプション管理
  └── フロントエンド: アップグレードダイアログ

Week 3: LP + インフラ + テスト
  ├── ランディングページ実装
  ├── Docker Compose 本番構成
  ├── Stripe テスト環境 E2E
  └── バグ修正 + リリース準備
```

---

## 9. セキュリティ考慮事項

1. **Webhook 署名検証**: Stripe の `stripe-signature` ヘッダーを `stripe.ConstructEvent()` で必ず検証
2. **Stripe API キーの管理**: 環境変数のみ、ソースコードにハードコードしない
3. **HTTPS 必須**: Caddy による自動TLS証明書（Let's Encrypt）
4. **冪等性**: Webhook は同一イベントが複数回送信される可能性があるため、冪等に処理
5. **レート制限**: Webhook エンドポイントにもレート制限を適用
6. **CSRF**: Checkout Session 作成時にユーザー認証を必須化

---

## 10. Open SaaS テンプレートからの主な参考ポイント

| Open SaaS パターン | ezbookkeeping への適用 |
|---|---|
| `ensureStripeCustomer()` — 顧客の作成/取得 | `pkg/payment/checkout.go` に同等ロジック |
| `createStripeCheckoutSession()` — セッション作成 | 同上。subscription モードのみ（one-time不要） |
| Webhook: `invoice.paid`, `subscription.updated/deleted` | `pkg/payment/webhook.go` で同じ3イベントを処理 |
| `PaymentPlanId` enum + Price ID マッピング | `pkg/payment/plans.go` でプラン定義 |
| `fetchCustomerPortalUrl()` | `pkg/payment/portal.go` に移植 |
| PricingPage コンポーネント（3カラム + ベストディール表示） | `PricingPage.vue` として Vue 3 + Vuetify で再実装 |
| `subscriptionStatus` フィールド on User model | `UserSubscription` テーブルとして分離 |
| Admin dashboard (revenue, users) | Phase 3 で実装予定 |
