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

## Oracle Cloud（Go API）

- `main` の CI が成功すると `.github/workflows/deploy.yml` がイメージを ghcr.io に置き、VM の `/opt/track-talk` で `docker compose pull` と `up` をする
- 初回の VM、SSH、GitHub Secrets は [Oracle Cloud で動かす手順](oracle-deploy.md)
- 環境変数は `deploy/.env.example`（`DATABASE_URL`, `SUPABASE_URL`, `SUPABASE_JWT_SECRET`, `SUPABASE_SERVICE_ROLE_KEY`, `CORS_ORIGINS` など）
- ヘルスチェック: `https://api.<ドメイン>/health`

## Netlify（Next.js）

- `netlify.toml` の base は `apps/web`。Node.js 22、`@netlify/plugin-nextjs`
- `NEXT_PUBLIC_API_BASE_URL`, `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY`, `NEXT_PUBLIC_SITE_URL`
- `NEXT_PUBLIC_API_BASE_URL` は Oracle の `https://api.<ドメイン>`
- クレジット制の無料枠と利用規約を確認する

## 管理者

`profiles.is_admin = true` を SQL で自分のUUIDに設定する。通報一覧は `GET /v1/admin/reports`。問い合わせ一覧は `GET /v1/admin/inquiries`。非表示は `POST /v1/admin/posts/{id}/hide`。
