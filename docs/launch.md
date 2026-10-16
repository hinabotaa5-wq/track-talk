# 公開チェックリスト

料金・無料枠・商用条件は変動するため、公開直前に各公式ドキュメントを確認してください。

## Supabase

- プロジェクトを作成し、Auth のメール確認と Redirect URLs にサイトURLを登録する
- JWT secret（Dashboard の JWT Settings）を API の `SUPABASE_JWT_SECRET` に入れる。空のときは API がプロジェクトの JWKS で検証する
- `DATABASE_URL` は Go API のみ。anon key 以外をフロントに置かない
- 全テーブルで RLS 有効（マイグレーション済み）。PostgREST からの直接CRUDは許可しない
- 退会時に Auth ユーザーを削除するため、service_role key を API の `SUPABASE_SERVICE_ROLE_KEY` に入れる。フロントエンドには置かない
- 標準SMTPは低レートのため、公開前にカスタムSMTP（Resend 等）を設定する
- 無料プランは非アクティブで一時停止する可能性がある

## Cloud Run（Go API）

- 初回は [Cloud Run で動かす手順](cloud-run-deploy.md) の `deploy/cloud-run/setup.sh` で作る。以降は Actions の「Deploy API」を手動で流してデプロイする（マージでは自動デプロイしない）
- 環境変数: `DATABASE_URL`, `SUPABASE_URL`, `SUPABASE_JWT_SECRET`, `SUPABASE_SERVICE_ROLE_KEY`, `CORS_ORIGINS`, `CLOUDINARY_CLOUD_NAME`, `CLOUDINARY_API_KEY`, `CLOUDINARY_API_SECRET`
- `CORS_ORIGINS` は画面の本番 URL
- ヘルスチェック: `/health`
- 予算（既定は月100円）を超えると API が止まる。再開は手作業
- VM に移す場合の手順は [Oracle Cloud で動かす手順](oracle-deploy.md)

## Vercel（Next.js）

- リポジトリの `main` が本番。`apps/web` をビルドする
- `NEXT_PUBLIC_API_BASE_URL`, `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY`, `NEXT_PUBLIC_SITE_URL`
- `NEXT_PUBLIC_API_BASE_URL` は Cloud Run の API の URL

## 管理者

`profiles.is_admin = true` を SQL で自分のUUIDに設定する。通報一覧は `GET /v1/admin/reports`。問い合わせ一覧は `GET /v1/admin/inquiries`。非表示は `POST /v1/admin/posts/{id}/hide`。
