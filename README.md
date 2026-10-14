# TRACK TALK

陸上競技のファン・選手・指導者が、大会・記録・レースについて匿名で語り合い、クラブチームの仲間や練習相手を見つけられるコミュニティ（MVP）です。都道府県陸協と国際大会の日程は自動で取り込み、月ごとの大会一覧として表示します。

## サービスの内容

陸上競技について語り合う、匿名のコミュニティです。プロフィールにニックネームや自己紹介の欄はなく、投稿やコメントに投稿者名は表示されません。スマホで使う前提の画面で、下部のタブ（ホーム／投稿／大会／マイページ）から各機能へ移動します。PC では同じタブがヘッダーに並びます。

### ログイン

- メールアドレスとパスワード（8文字以上）、または Google アカウントで登録・ログインします（Supabase Auth）。メール登録では確認メールが届きます。パスワード再設定もできます。
- ログインしなくても、ホームの投稿一覧・投稿の詳細・コメントは見られます。投稿・大会一覧・マイページ・いいね・保存・コメント・通報にはログインが必要です。

### ホーム（`/`）

- 投稿をカードで並べます（スマホ2列、PC 3〜4列、20件ずつ）。
- 並び替え: **人気**（直近14日の投稿をいいね数順）／**新しい順**／**募集**（クラブチーム募集だけ）／**保存**（自分が保存した投稿）
- キーワード検索: タイトル・本文・募集の各項目を部分一致で探します。都道府県名を入れると、その県を活動場所にした募集も見つかります。

### 投稿（`/upload`）

投稿の種類は5つです。

| 種類 | 内容 | 主なルール |
| --- | --- | --- |
| 文章 | タイトル＋本文でスレッドを立てる | タイトル120字・本文10000字まで。写真1枚（10MB まで）を付けられる |
| YouTube の URL | 動画を引用してスレッドを作る | タイトルとサムネイルは YouTube から取得。1つの動画につきスレッドは1つで、既にあればそのスレッドを開く |
| 自分の動画 | 撮った動画をアップロード | 100MB まで。Cloudinary へブラウザから直接アップロードする |
| クラブチームの募集 | チームのメンバーを募る | チーム名・活動場所（都道府県を1つ以上＋詳細）・活動時間・男女比・平均年齢・募集年齢・SNS の URL（https、4件まで）・写真（1枚まで）・詳細。通知は飛ばず、募集タブに出る |
| 練習相手募集 | 一緒に練習する相手を募る | 練習場所・日時（未来のみ）・種目（100m〜十種競技、リレー、その他）・募集人数（1〜99人）・詳細（2000字まで） |

自分の投稿は編集・削除できます。他の人の投稿には、いいね・保存・コメント（2000字まで）・通報ができます。自分のコメントも編集・削除できます。

### 練習相手募集の流れ

1. 募集者が練習の日時・場所・種目・人数を投稿する
2. 参加したい人が「参加したい」で申請する → 募集者に通知
3. 募集者が申請を **承認** か **見送り** にする → 承認された人に通知。承認数が定員に達すると「満員」になる
4. 練習が終わったら募集者が **解散** する → 承認された参加者がいれば、募集者と参加者の全員に「評価しあってください」の通知
5. 解散後、募集者と参加者がお互いを **良い／普通／悪い** と、ひとこと（200字まで）で評価する。評価は1回だけで、あとから変えられない
6. 評価は募集者の「これまでの評価」として、練習の投稿に表示される

満員か解散の後は、募集者と承認された参加者だけがコメントできます（他の人はコメントを見るだけ）。

### 大会（`/competitions`）

- 月と地域（47都道府県または「国際」）を選んで、その月の大会を一覧にします。初期表示は今月（日本時間）です。最後に選んだ地域はブラウザに保存され、次回も同じ地域で開きます。
- 各大会には日程・会場と、主催者のページ（都道府県は「要項」、国際大会は「公式サイト」）へのリンクがあります。
- 今後の世界陸上・オリンピックの開催地（都市・会場・日程、Wikimedia Commons の写真）も表示します。
- データは自動で取り込んでいます（下の「大会データの取り込み」）。

### 通知

ヘッダーのベルに未読数が出ます。通知の種類は次の4つです。開くと既読になり、「すべて既読」もできます。

- 自分の投稿へのコメント
- 練習への参加申請（募集者へ）
- 参加の承認（申請した人へ）
- 練習の解散と評価のお願い（募集者と参加者へ）

### マイページ（`/me`）

- 自分の投稿の一覧
- お問い合わせ（種類: 不具合／アカウント／機能の要望／その他）と送信履歴
- ガイドライン・利用規約・プライバシーポリシー
- ログアウト
- 退会: 確認のため「退会する」と入力すると、自分の投稿・コメント・いいね・保存・通知・練習への参加と評価・お問い合わせ・通報、そしてログイン用のアカウントが削除されます

### 安全のための仕組み

- コミュニティガイドライン（`/guidelines`）で、誹謗中傷・権利侵害・個人を特定する情報・練習会の悪用などを禁止しています。
- 投稿とコメントは、理由（スパム／嫌がらせ／誤情報／その他）を選んで通報できます。
- 管理者（`profiles.is_admin = true`）は、通報と問い合わせの一覧を見て、投稿やコメントを非表示にできます。管理画面はなく、API（`/v1/admin/...`）で操作します。
- API は接続元ごとに回数を制限しています（書き込みは1分に30回、通報とお問い合わせはそれぞれ1時間に10回まで）。
- アクセス数は Umami で計測しています。

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

CI（`.github/workflows/ci.yml`）は push／PR ごとに API テスト（PostgreSQL サービスコンテナ付き）、Web の Lint・単体テスト・型チェック、ログインから文章投稿までの E2E を実行します。その E2E が落ちると CI が失敗します。

## デプロイ

- **API**: Render。`render.yaml` の Docker サービスが `main` をビルドする。ヘルスチェックは `/health`。初回は Render でこのリポジトリの Blueprint を作る。
- **Web**: Vercel（`apps/web`）。`main` への push で本番が更新される。
- **DB / Auth**: Supabase

公開前の確認事項は [docs/launch.md](docs/launch.md) を参照してください。料金・無料枠・商用条件は変わるため、公開直前に各公式ドキュメントで確認してください。

## 管理者

`profiles.is_admin = true` を SQL で自分のユーザーに設定すると、管理用 API を使えます。

| API | 内容 |
| --- | --- |
| `GET /v1/admin/reports` | 通報の一覧 |
| `GET /v1/admin/inquiries` | お問い合わせの一覧 |
| `POST /v1/admin/posts/{id}/hide` | 投稿を非表示にする |
| `POST /v1/admin/comments/{id}/hide` | コメントを非表示にする |
