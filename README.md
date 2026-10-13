# TRACK TALK

陸上競技のファン・選手・指導者が、大会・記録・レース・練習について語り合うコミュニティ（MVP）です。都道府県陸協や国際大会の日程を自動で取り込み、大会一覧として表示します。

## 主な機能

- **投稿**: テキスト（画像つき）・動画（Cloudinary）・YouTube 引用・練習会・募集の各種投稿、いいね・保存・コメント
- **練習会**: 参加申請 → 承認／見送り、解散、参加後の評価
- **大会一覧**: 都道府県（`01`〜`47`）と国際大会（`00`）の日程、五輪・世界陸上の開催地
- **マイページ**: 自分の投稿・保存・通知・問い合わせ、退会
- **管理**: 通報・問い合わせの確認、投稿／コメントの非表示

## 構成

| 場所 | 役割 | 技術 |
| --- | --- | --- |
| `apps/web` | 画面・ルーティング | Next.js 16 / React 19 / TypeScript / Tailwind CSS 4 / Zod |
| `apps/api` | 業務ロジック・認可（`/v1`） | Go 1.25 / chi / pgx / sqlc / golang-migrate |
| PostgreSQL + Supabase Auth | DB・認証 | マイグレーションは `apps/api/migrations` |
| Cloudinary | 画像・動画の保存 | API が署名を発行し、ブラウザから直接アップロード |
| Gemini API | 都道府県の大会日程ページから大会を抽出 | `apps/api/internal/competitions` |

業務ロジックは Go API に集約しています。Next.js から PostgREST でテーブルを直接操作しません（全テーブルで RLS 有効）。

```
apps/
  api/
    cmd/api/                 API サーバー
    cmd/migrate/             マイグレーション
    cmd/importcompetitions/  大会日程の取り込み
    cmd/importmajors/        五輪・世界陸上の開催地の取り込み
    internal/                auth, cloudinary, competitions, config, db, httpx, migrate
    migrations/  queries/    SQL（queries は sqlc 用）
  web/
    app/  components/  lib/  e2e/  emails/
docs/launch.md               公開チェックリスト
```

## ローカル起動

必要なもの: Go 1.25、Node.js 22、Docker（PostgreSQL 用）

```bash
cp .env.example .env
# .env に Supabase の URL / anon key / JWT secret（必要なら Cloudinary）を入れる
docker compose up -d postgres
make migrate
make api        # http://localhost:8080/health
# 別ターミナル
cd apps/web && npm install && cd ../..
make web        # http://localhost:3000
```

SQL を変更したら `make sqlc` でコードを再生成します（sqlc が必要）。

## 環境変数

`.env.example` が雛形です。API はリポジトリ直下の `.env` も読み込みます。

| 変数 | 使う場所 | 説明 |
| --- | --- | --- |
| `DATABASE_URL` | API・取り込み | PostgreSQL の接続文字列（必須） |
| `SUPABASE_URL` / `SUPABASE_JWT_SECRET` / `SUPABASE_JWT_AUD` | API | JWT の検証（URL か secret のどちらかが必須） |
| `SUPABASE_SERVICE_ROLE_KEY` | API | 退会時に Auth ユーザーを削除する |
| `CLOUDINARY_CLOUD_NAME` / `CLOUDINARY_API_KEY` / `CLOUDINARY_API_SECRET` | API | アップロード署名 |
| `API_ADDR` / `CORS_ORIGINS` / `LOG_LEVEL` | API | 既定は `:8080` / `http://localhost:3000` / `info` |
| `GEMINI_API_KEY` | 大会取り込み | 都道府県の取り込みに必要 |
| `GEMINI_DAILY_REQUESTS` | 大会取り込み | 1回の実行で使う Gemini リクエストの上限（既定 15） |
| `NEXT_PUBLIC_API_BASE_URL` / `NEXT_PUBLIC_SUPABASE_URL` / `NEXT_PUBLIC_SUPABASE_ANON_KEY` / `NEXT_PUBLIC_SITE_URL` | Web | ブラウザに公開される値 |

フロントに置くのは `NEXT_PUBLIC_*` と **anon key のみ**です。`service_role`・`DATABASE_URL`・Cloudinary の secret・Gemini のキーは API／ホスティング／GitHub Secrets の秘密領域だけに置いてください。

## 大会データの取り込み

`apps/api` で実行します。

```bash
go run ./cmd/importcompetitions                           # 期限切れの国際大会と都道府県を取り込む
go run ./cmd/importcompetitions -prefecture 13            # 指定した県だけ（00 は国際大会）
go run ./cmd/importcompetitions -prefecture 46 -dry-run   # DB に書かずに結果を表示
go run ./cmd/importcompetitions -prefecture 46 -dry-run -url https://example.com/schedule  # 未登録のページを試す
go run ./cmd/importmajors                                 # 五輪・世界陸上の開催地
```

- **都道府県**: `internal/competitions/sources.go` に登録した各陸協の日程ページ（一部は PDF）を取得し、表を抜き出して Gemini で大会情報に変換します。前回から 10 日経った県が対象で、古い順に Gemini の上限（`GEMINI_DAILY_REQUESTS`）に収まるところまで処理します。収まらなかった県は次回に回ります。
- **国際大会**: World Athletics と Diamond League のカレンダーから Gemini を使わずに取り込みます。7 日ごとに更新します。
- **五輪・世界陸上の開催地**: Wikipedia から取り込みます。

### 定期実行（GitHub Actions）

`.github/workflows/imports.yml` で実行しています。手動実行は Actions の「Imports」→ Run workflow から行えます。

| ジョブ | スケジュール | 必要な Secrets |
| --- | --- | --- |
| `competitions` | 毎日 06:00 JST | `DATABASE_URL`, `GEMINI_API_KEY` |
| `major-hosts` | 4月・10月の1日 09:00 JST | `DATABASE_URL` |

## テスト・Lint

```bash
make test-api                                  # Go（統合テストは DATABASE_URL の PostgreSQL が必要）
make test-web                                  # Vitest
cd apps/web && npm run lint && npx tsc --noEmit
cd apps/web && npm run test:e2e                # Playwright
```

CI（`.github/workflows/ci.yml`）は push／PR ごとに API テスト（PostgreSQL サービスコンテナ付き）と Web の Lint・単体テスト・型チェックを実行します。

## デプロイ

- **API**: Render（`apps/api/Dockerfile`、ヘルスチェック `/health`）。`render.yaml` は設定の参考で、Render のサービスは手動作成のため自動では反映されません。
- **Web**: `apps/web` をビルドします（Netlify 用の設定は `netlify.toml`）。
- **DB / Auth**: Supabase

公開前の手順・確認事項は [docs/launch.md](docs/launch.md) を参照してください。料金・無料枠・商用条件は変わるため、公開直前に各公式ドキュメントで確認してください。

## 管理者

`profiles.is_admin = true` を SQL で自分のユーザーに設定すると、管理用 API（`/v1/admin/...`）を使えます。詳細は [docs/launch.md](docs/launch.md) を参照してください。
