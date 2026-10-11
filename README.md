# TRACK TALK

陸上競技の大会・記録・レースについてファン同士が議論するコミュニティ（MVP）。

- フロントエンド: Next.js（画面・ルーティング）
- API: Go + chi（業務ロジック・認可）
- DB / Auth: PostgreSQL + Supabase Auth

業務ロジックは Go API に集約しています。Next.js から PostgREST でテーブルを直接操作しません。

## ローカル起動

```bash
cp .env.example .env
# .env に Supabase の URL / anon key / JWT secret を入れる
docker compose up -d postgres
make migrate
make api
# 別ターミナル
make web
```

- API: http://localhost:8080/health
- Web: http://localhost:3000

フロントに置くのは `NEXT_PUBLIC_SUPABASE_URL` と **anon key のみ**です。`service_role` と `DATABASE_URL` は API / ホスティングの秘密領域だけに置いてください。

## テスト

```bash
make test-api
make test-web
cd apps/web && npm run test:e2e
```

API の統合テストは `DATABASE_URL` の PostgreSQL が必要です。CI では Compose 相当のサービスコンテナで実行します。

## 公開

手順と確認事項は [docs/launch.md](docs/launch.md) を参照してください。料金・無料枠・商用条件は公開直前に公式ドキュメントで確認してください。
