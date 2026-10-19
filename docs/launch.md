# 公開チェックリスト

料金・無料枠・商用条件は変動するため、公開直前に各公式ドキュメントを確認してください。

## Supabase

- プロジェクトを作成し、Auth のメール確認と Redirect URLs にサイトURLを登録する
- JWT secret（Dashboard の JWT Settings）を API の `SUPABASE_JWT_SECRET` に入れる。空のときは API がプロジェクトの JWKS で検証する
- `DATABASE_URL` は Go API のみ。anon key 以外をフロントに置かない
- 全テーブルで RLS 有効（マイグレーション済み）。PostgREST からの直接CRUDは許可しない
- 退会時に Auth ユーザーを削除するため、service_role key を API の `SUPABASE_SERVICE_ROLE_KEY` に入れる。フロントエンドには置かない
- 標準SMTPは低レートのため、公開前にカスタムSMTP（Resend 等）を設定する。コーチングを公開するときは、API の `RESEND_API_KEY` と `MAIL_FROM`（保護者の同意のリンクの送信）も同じ Resend のドメインで設定する
- 無料プランは非アクティブで一時停止する可能性がある

## Cloud Run（Go API）

- 初回は [Cloud Run で動かす手順](cloud-run-deploy.md) の `deploy/cloud-run/setup.sh` で作る。以降は Actions の「Deploy API」を手動で流してデプロイする（マージでは自動デプロイしない）
- 環境変数: `DATABASE_URL`, `SUPABASE_URL`, `SUPABASE_JWT_SECRET`, `SUPABASE_SERVICE_ROLE_KEY`, `CORS_ORIGINS`, `CLOUDINARY_CLOUD_NAME`, `CLOUDINARY_API_KEY`, `CLOUDINARY_API_SECRET`, `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT`
- `COACHING_ENABLED` はオンラインコーチングを提供するまで設定しない（Web の `NEXT_PUBLIC_COACHING_ENABLED` も同じ）。空のうちは保護者の同意・決済、パーソナル指導、フォーム添削、プッシュ通知、コーチの報酬と送金、運営の送金が 404 になり、コーチの応募と運営の審査だけが残る
- `CORS_ORIGINS` は画面の本番 URL
- ヘルスチェック: `/health`
- 予算（既定は月100円）を超えると API が止まる。再開は手作業
- VM に移す場合の手順は [Oracle Cloud で動かす手順](oracle-deploy.md)

## Stripe（コーチングの支払い・送金）

コーチングを提供するまでは、`COACHING_ENABLED` を空にしたまま準備だけ進められる（空のうちは支払いも Webhook も 404）。
API は、Stripe の鍵だけ入れて Webhook の鍵が無いときや、本番の鍵で https でないときは、起動しない（お金だけ動いて期間が始まらない状態を防ぐため）。

- ダッシュボードで本番に切り替え、本番用の鍵を作る（サンドボックスの鍵・Webhook の鍵は本番では使えない）。秘密鍵は `STRIPE_SECRET_KEY`（`sk_live_...`、または必要な権限だけの制限付きキー `rk_live_...`）
- 支払い方法（カード・Apple Pay・Google Pay・PayPay）は、ダッシュボードの「支払い方法」で有効にしたものが支払い画面に出る。PayPay は審査が要る場合がある
- Connect を有効にする（コーチへの送金。コーチは Express アカウントで、国は日本）
- Webhook の送り先を作る。URL は `https://（API の URL）/webhooks/stripe`
  - 自分のアカウント用: `checkout.session.completed`、`checkout.session.async_payment_succeeded`、`charge.dispute.created`。署名の鍵を `STRIPE_WEBHOOK_SECRET` に入れる
  - 接続アカウント用: `account.updated`。署名の鍵（別の値）を `STRIPE_CONNECT_WEBHOOK_SECRET` に入れる
- `STRIPE_SECRET_KEY` と `STRIPE_WEBHOOK_SECRET` を Cloud Run の環境変数に入れて、「Deploy API」を流す。起動ログの `stripe mode=live` で本番の鍵だと確かめる
- 提供を始めるときに、`COACHING_ENABLED`（API）と `NEXT_PUBLIC_COACHING_ENABLED`（Web）を入れ、Web も「Deploy Web」を流す
- 公開前に、少額で支払い→返金までを通して確かめる（本番の鍵で自分のカードを使い、運営の「パーソナル」タブから返金する）
- 特定商取引法に基づく表記のページ（事業者名・所在地・連絡先・返金の条件など）が公開前に必要。事業者の情報は運営が用意する

## Netlify（Next.js）

- 本番は Actions の「Deploy Web」を手動で流す（`main` をビルドフックでビルド）。`apps/web` をビルドする
- `NEXT_PUBLIC_API_BASE_URL`, `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY`, `NEXT_PUBLIC_SITE_URL`
- `NEXT_PUBLIC_API_BASE_URL` は Cloud Run の API の URL

## 管理者

`profiles.is_admin = true` を SQL で自分のUUIDに設定する。通報一覧は `GET /v1/admin/reports`。問い合わせ一覧は `GET /v1/admin/inquiries`。非表示は `POST /v1/admin/posts/{id}/hide`。
