# DB のリージョン移行（Supabase: シンガポール → 東京）

API は Cloud Run の東京（`asia-northeast1`）にあるが、DB（Supabase）は **シンガポール（`ap-southeast-1`）** にあった。クエリのたびに東京↔シンガポールを往復していたため、DB を東京（`ap-northeast-1`）に移した。2026年10月19日に実施。

Supabase のプロジェクトのリージョンは後から変えられない。**東京に新しいプロジェクトを作り、接続先を切り替える**。

## 今回やったこと（データは持ち越していない）

利用者のデータは持ち越さないと決め、**空の新プロジェクトにテーブルだけを作って**切り替えた。旧プロジェクトの利用者は、新プロジェクトで再登録になる。

### 1. 新プロジェクトを作る

1. 東京（Northeast Asia (Tokyo)）に新しいプロジェクトを作る。作成画面の Region は「Asia-Pacific」のままだと東京とは限らないので、「Need a specific region?」から選ぶ。「Automatically expose new tables」は外す
2. 次を控える（チャットには貼らない）
   - 接続プーラーの URL（Connect → Session pooler）。Cloud Run と GitHub Actions は IPv4 のため、直接接続ではなくプーラーを使う。ユーザー名は `postgres.<プロジェクトID>` の形になる
   - Project URL、anon key、service_role key
   - データベースのパスワード。特殊文字（`@` `/` `:` `#` `?` `%` など）があると接続文字列が壊れるので、英数字だけにする

### 2. テーブルを作る

`main` のマイグレーションを、新プロジェクトに流す。作業中のブランチに未リリースのマイグレーションがあると、それも新 DB に入ってしまうので、**`main` の一時的な作業ツリーから流す**。

```bash
read -rs "NEW_DATABASE_URL?新のプーラー URL: "; echo; D="$(mktemp -d)/tt-main" && git worktree add -q --detach "$D" main && (cd "$D/apps/api" && DATABASE_URL="$NEW_DATABASE_URL" ALLOW_REMOTE_DB=1 SUPABASE_JWT_SECRET=dummy go run ./cmd/migrate up) && psql "$NEW_DATABASE_URL" -Atc "select version, dirty from public.schema_migrations; select count(*) from information_schema.tables where table_schema='public'"; git worktree remove --force "$D"
```

- `ALLOW_REMOTE_DB=1` は、ローカル DB 以外への接続を許すための指定。`SUPABASE_JWT_SECRET=dummy` は設定の読み込みを通すためだけの仮の値
- 版が `main` の最新で、dirty でないこと（`66|f`）、テーブルが 51 個あることを確かめる

### 3. Auth の設定を旧プロジェクトと揃える

| 項目 | 値 |
| --- | --- |
| Site URL | `https://track-talk.netlify.app` |
| Redirect URLs | `https://track-talk.netlify.app/auth/callback`、`https://track-talk.netlify.app/login`、`http://localhost:3000/auth/callback`、`http://localhost:3000/login` |
| Providers | Email と Google |

- 旧プロジェクトの Site URL は前に使っていた Vercel のままになっていた（Netlify に直す価値はあるが、移行とは別の修正）
- Google は Client ID と Secret を入れる。旧プロジェクトの画面では Secret が隠れるので、Google Cloud コンソールの「認証情報」で、同じ OAuth クライアントからコピーする。その「承認済みのリダイレクト URI」に、新プロジェクトの `https://<新プロジェクトのID>.supabase.co/auth/v1/callback` を**追加**する。旧プロジェクトのものは、切り替えが終わるまで消さない
- メールのテンプレートは、旧で文面を変えていれば同じものを入れる

### 4. 切り替える前に、手元で画面から試す

画面と API の両方を、新プロジェクトに向けてローカルで起動し、登録、Google ログイン、投稿を確かめる。環境変数はファイルを書き換えず、コマンドの先頭に付けて渡す。

- API（`main` の一時的な作業ツリーで）: `DATABASE_URL`（新）、`ALLOW_REMOTE_DB=1`、`SUPABASE_URL`（新）、`CORS_ORIGINS=http://localhost:3000` を付けて `go run ./cmd/api`。`SUPABASE_JWT_SECRET` は付けない（空のときは Project URL の公開鍵で検証する）
- 画面: `NEXT_PUBLIC_SUPABASE_URL`、`NEXT_PUBLIC_SUPABASE_ANON_KEY`、`NEXT_PUBLIC_API_BASE_URL=http://localhost:8080`、`NEXT_PUBLIC_SITE_URL=http://localhost:3000` を付けて `npm run dev`

### 5. 本番を切り替える

**順番が大事。** 手順 3 の前に Netlify を再デプロイすると、画面は新プロジェクトでログインし、API は旧プロジェクトの鍵で検証するので、ログインしたあとの API が `401` になる。

1. **API を `main` の最新でデプロイする**（Actions の「Deploy API」を Run workflow、ブランチは `main`）。Cloud Run のイメージのマイグレーションが、新 DB の版より古いと、`migrate: no migration found for version …` で起動に失敗する（今回、実際にこれで 1 度失敗した。失敗したリビジョンにはトラフィックが乗らず、前のリビジョンが動き続ける）
2. Cloud Run `track-talk-api` の環境変数を、新プロジェクトの値にする: `DATABASE_URL`、`SUPABASE_URL`、`SUPABASE_JWT_SECRET`（旧で設定していれば新の値、空なら空のまま）、`SUPABASE_SERVICE_ROLE_KEY`。新しいリビジョンが立ち上がり、`/health` が `{"status":"ok"}` を返すことを確かめる
3. Netlify の `NEXT_PUBLIC_SUPABASE_URL` と `NEXT_PUBLIC_SUPABASE_ANON_KEY` を新プロジェクトの値にし、Actions の「Deploy Web」で再デプロイする（マージでは自動で流れない）
4. 本番の画面で、登録、ログイン（メールと Google）、投稿を通しで試す
5. GitHub Secrets の `DATABASE_URL` を、新のプーラー URL にする
6. 大会データは空なので、Actions の「Imports」を手動で流して入れ直す

   | 取り込み | 自動で流れる時期 | 備考 |
   | --- | --- | --- |
   | `domestic-majors` | 毎月 1 日 | 今回 34 件が入った。`2027` のページの `404` は、まだ公開されていないためで、毎回出る |
   | `major-hosts` | 4 月・10 月の 1 日 | 下記の `429` で失敗する |
   | `competitions` | 毎日 06:00（日本時間） | Gemini を使う。自動で入る |

### 6. 切り替え後

- 旧プロジェクトは**少なくとも 1 週間は消さず**に残し、問題があれば環境変数を戻せるようにする。消すときは、必要なら最終の dump を手元に保管してから
- 旧プロジェクトの Google の「承認済みのリダイレクト URI」の旧 callback も、旧プロジェクトを消すときに外す
- 動作確認で作ったテスト用のユーザーと投稿は、新プロジェクトの Authentication → Users などから消せる

## 既知の問題（移行とは無関係）

- `major-hosts` は、Wikipedia の API が `429 Too Many Requests` を返し、**移行の前から**失敗している（`imports` の定期の実行も、10 月 14 日から連続して失敗）。`User-Agent` に連絡先を含める、`429` のときに待って再試行する、などの修正が考えられる
- `go test` は、手元の Mac のタイムゾーン（日本時間）で、`TestReviewRequestNeedsDemoCode` が失敗する（`TZ=UTC` を付けると通る。CI は UTC なので影響しない）

## データを持ち越す場合（今回は不要だった）

利用者のデータを持ち越す必要があるときは、次のスクリプトでリハーサルできる。今回、本番（シンガポール）から新プロジェクトへ流して確かめた。

| 項目 | 結果 |
| --- | --- |
| `public` の dump / restore | 14 秒 / 17 秒、51 テーブルの行数が一致、マイグレーションの版も一致 |
| `auth.users` / `auth.identities` | 4 → 4 / 6 → 6、パスワードログインを確認 |

- `rehearse.sh`(移行後に削除): `public` を dump して restore し、行数とマイグレーションの版を旧と突き合わせる。旧 DB は読むだけで、新 DB の `public` にテーブルがあると止まる
- `rehearse-auth.sh`(移行後に削除): `auth.users` と `auth.identities` のデータだけを流す。`sessions`・`refresh_tokens` などの使い捨てのテーブルは移さない。新プロジェクトの JWT 鍵は旧と別物なので、利用者は一度ログインし直しになる
- `pg_dump` は、サーバーと同じ（または新しい）メジャーバージョンが要る。サーバーが 17 のとき、手元が 16 だと `server version mismatch` で止まる。`brew install postgresql@17` のあと、`PATH` の先頭に `/opt/homebrew/opt/postgresql@17/bin` を足す
- 接続文字列は `read -rs` で変数に入れ、**同じコマンドの中で**使う（別のコマンドに分けると、環境変数が引き継がれない場合がある）
- dump にはパスワードのハッシュとメールアドレスが入る。終わったら、保存先のディレクトリごと消す
- 新 DB に向けて `go test` を通すと、新 DB に本番の大会データがある場合、大会関連の 3 件が落ちる（`competitions`、`domestic_majors` はテストの TRUNCATE の対象外）。テストは投稿やプロフィールを TRUNCATE するので、**本番には向けない**

## ロールバック

切り替え後に問題が出たら、手順 5 の環境変数を旧プロジェクトの値に戻す（旧プロジェクトを残しているうちは可能）。切り替え後に新プロジェクトへ入った書き込みは旧には無い。

## 効果の確認

DB 往復 1 回あたり、東京↔シンガポール（約 70ms）から東京内（数 ms）になる。API が 1 リクエストで複数クエリを発行する画面ほど体感が速くなる。切り替え後に、主要 API の応答時間を測って確かめる。
