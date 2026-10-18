# DB のリージョン移行計画（Supabase: シンガポール → 東京）

API は Cloud Run の東京（`asia-northeast1`）にあるが、DB（Supabase）は **シンガポール（`ap-southeast-1`）** にある。クエリのたびに東京↔シンガポールを往復するため、これを東京（`ap-northeast-1`）に移す。

## 方針

Supabase のプロジェクトのリージョンは後から変えられないので、**東京に新しいプロジェクトを作り、データを移して接続先を切り替える**。短い停止を許容し、dump / restore で一括移行する。

## 分かっていること（コードから）

- アプリ側のマイグレーション（`apps/api/migrations`）に `auth.*` への参照はない。アプリのテーブルは `public` スキーマだけを移せばよい
- API が DB に触るのは `DATABASE_URL`（pgxpool）だけ。接続先の切り替えは環境変数 1 つで済む
- 画像は Cloudinary にあり、Supabase Storage は使っていない（移行不要）
- 認証は Supabase Auth。API は `SUPABASE_URL` の JWKS か `SUPABASE_JWT_SECRET` で検証し、退会時に `SUPABASE_SERVICE_ROLE_KEY` で Auth ユーザーを削除する

## リハーサルの結果

東京に作った新プロジェクトに、本番（シンガポール）のデータを流して確かめた。

| 項目 | 結果 |
| --- | --- |
| `public` の dump（旧） | 14 秒 |
| `public` の restore（新） | 17 秒 |
| `public` の行数 | 51 テーブルすべて一致 |
| マイグレーションの版 | 一致（65、dirty でない） |
| `auth.users` | 4 → 4（一致） |
| `auth.identities` | 6 → 6（一致） |
| 新 DB でのパスワードログイン | 確認済み（`auth` を流したユーザーで成功） |
| 新 DB に向けた `go test ./...` | 4 件が失敗したが、どれも移行が原因ではない（下記） |

- データの移行そのものは、実測で `public` が 1 分に満たない。停止時間は、接続先の差し替えと確認を含めて **30 分を見込めば足りる**
- 本番の版は 65。リポジトリにはそれより新しいマイグレーションがあり、切り替え後の API のデプロイで新 DB に適用される

### `go test` の失敗 4 件

RLS・マイグレーション・API の動作に関わる失敗はなかった。失敗した 4 件の原因は次のとおり。

- `TestDeleteCompetitionsBeforeSeason`、`TestDeletePastCompetitionSeasonsKeepsCurrentAndNext`、`TestTodayMeetThreadsCreatesOneThreadPerTargetMeet`: 新 DB に流した本番の大会データ（`competitions`、`domestic_majors`）と、テストの前提がぶつかった。これらのテーブルはテストの TRUNCATE の対象外。空のローカル DB では通る
- `TestReviewRequestNeedsDemoCode`: 手元の Mac のタイムゾーン（日本時間）で、期限が `Z` でなく `+09:00` で返る。ローカル DB でも失敗し、`TZ=UTC` を付けると通る。CI は UTC なので影響しない

## 要判断・要確認

1. **Auth ユーザーの扱い**: 方式 A（`auth.users`・`auth.identities` のデータだけを移す）に決め、リハーサルで通った。パスワードのハッシュと Google 連携が引き継がれ、ユーザーは再登録不要。ただし新プロジェクトの JWT 鍵は旧と別物なので、**切り替え後は全員が一度ログインし直す**
2. **Google ログイン**: 未検証。`auth.identities` は移るが、新プロジェクトで Google を有効にし、Google Cloud コンソールの OAuth クライアントに新プロジェクトのコールバック URL（`<Project URL>/auth/v1/callback`）を追加する必要がある。画面から実際にログインして確かめる
3. **Supabase のプラン**: 組織は無料プランで、同時に持てるプロジェクトの数に上限がある。旧と新を並べる間の費用と上限を確認する

## 手順

### 0. 準備（停止なし）

1. 東京リージョンに新プロジェクトを作る（旧と同じ Postgres のメジャーバージョン。現在は 17）
2. 新プロジェクトの接続プーラー URL（Cloud Run は IPv4 のため、プーラーが必要）、`SUPABASE_URL`、anon key、JWT 設定、service_role key を控える。プーラーのユーザー名は `postgres.<プロジェクトID>` の形になる
3. Authentication の設定を旧プロジェクトと揃える
   - URL Configuration: Site URL と Redirect URLs（Netlify のサイトの URL）
   - Providers: Google を有効にし、OAuth クライアントのコールバック URL を追加
   - メールのテンプレートと、メール送信（カスタム SMTP を使っていれば同じもの）
4. 手元に PostgreSQL 17 のクライアントを入れる。サーバーが 17 のため、16 の `pg_dump` は `server version mismatch` で止まる

```bash
brew install postgresql@17
export PATH="/opt/homebrew/opt/postgresql@17/bin:$PATH"   # そのターミナルのタブだけで有効
pg_dump --version                                         # 17.x と出ること
```

### 1. リハーサル（旧 DB は読むだけ）

接続文字列は、チャットや履歴に残らないよう、`read -rs` で変数に入れる。**同じコマンドの中で**変数を使うこと（別のコマンドに分けると、環境変数が引き継がれない場合がある）。

`public`（アプリのデータ）:

```bash
read -rs "OLD_DATABASE_URL?旧のプーラー URL: "; echo; read -rs "NEW_DATABASE_URL?新のプーラー URL: "; echo; OLD_DATABASE_URL="$OLD_DATABASE_URL" NEW_DATABASE_URL="$NEW_DATABASE_URL" bash deploy/db-migration/rehearse.sh
```

[`deploy/db-migration/rehearse.sh`](../deploy/db-migration/rehearse.sh) は旧 DB を読むだけで、新 DB の `public` にテーブルがあれば止まる。dump → restore のあと、テーブルごとの行数と `schema_migrations` の版を旧と突き合わせる。Supabase には `public`・`auth` が最初からあるので、スキーマ自体の作成は流さない。

`auth`（ユーザー）:

```bash
read -rs "OLD_DATABASE_URL?旧のプーラー URL: "; echo; read -rs "NEW_DATABASE_URL?新のプーラー URL: "; echo; OLD_DATABASE_URL="$OLD_DATABASE_URL" NEW_DATABASE_URL="$NEW_DATABASE_URL" bash deploy/db-migration/rehearse-auth.sh
```

[`deploy/db-migration/rehearse-auth.sh`](../deploy/db-migration/rehearse-auth.sh) は、`auth.users` と `auth.identities` の**データだけ**を流す（テーブルの定義は Supabase が新プロジェクトに作っている）。次は移さない。

- `sessions`・`refresh_tokens`・`flow_state`・`one_time_tokens`: 使い捨てで、新しい JWT 鍵では無効になる
- `auth.schema_migrations`: Supabase が管理している

多要素認証（`mfa_factors`）を使っているときは、環境変数 `TABLES` に `auth.mfa_factors` を足す。新 DB の `auth.users` が空でなければ止まる。

dump にはパスワードのハッシュとメールアドレスが入る。**確認が済んだら、スクリプトの最後の行に出る場所を `rm -r` で消す**。

確認:

1. 新プロジェクトの Project URL と anon key で、パスワードログインを試す（`200` ならハッシュが引き継がれている）

```bash
read -r "URL?新の Project URL: "; read -rs "KEY?新の anon key: "; echo; read -r "EMAIL?メールアドレス: "; read -rs "PW?パスワード: "; echo; curl -s -o /dev/null -w "%{http_code}\n" "$URL/auth/v1/token?grant_type=password" -H "apikey: $KEY" -H "Content-Type: application/json" -d "{\"email\":\"$EMAIL\",\"password\":\"$PW\"}"
```

2. 新 DB に向けて `go test` を通す。テストは `public` の投稿やプロフィールを TRUNCATE するので、**必ず新 DB のリハーサル用の URL で行い、本番（旧）には向けない**。ローカル DB への保護を外すため `ALLOW_REMOTE_DB=1` が要る。タイムゾーンで 1 件落ちるので `TZ=UTC` を付ける

```bash
read -rs "NEW_DATABASE_URL?新のプーラー URL: "; echo; cd apps/api && TZ=UTC DATABASE_URL="$NEW_DATABASE_URL" ALLOW_REMOTE_DB=1 go test ./...
```

   新 DB に本番の大会データが入っていると、大会関連の 3 件が落ちる（上の「`go test` の失敗 4 件」）。

### 2. 切り替え（停止 30 分前後）

**切り替え用の新プロジェクトは、リハーサルとは別に、空の状態から作る。** `rehearse.sh` は新 DB の `public` にテーブルがあると止まり、リハーサルのデータをテーブルごと安全に消す方法は用意していない。無料プランでは同時に持てるプロジェクトの数に上限があるので、リハーサル用のプロジェクトを削除してから、東京に作り直す。Auth の設定（手順 0 の 3）も、作り直したプロジェクトでやり直す。

1. Cloud Run の API を止めるか、書き込みを止める（`ingress` を絞る、または台数を 0 にする）。フォーム添削の Cloud Scheduler ジョブ `track-talk-coaching-tick` と GitHub Actions の `imports`（取り込み）も一時停止する
2. 最終の dump → restore を流す（`rehearse.sh`、`rehearse-auth.sh` の順）
3. 行数と `schema_migrations` を再確認する
4. 次を新プロジェクトの値に差し替える

| 場所 | 値 |
| --- | --- |
| Cloud Run `track-talk-api` | `DATABASE_URL`, `SUPABASE_URL`, `SUPABASE_JWT_SECRET`, `SUPABASE_SERVICE_ROLE_KEY` |
| Netlify（画面） | `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY` → 再デプロイ |
| GitHub Secrets | `DATABASE_URL`（`imports.yml` が使う） |
| `deploy/cloud-run/env.yaml`（手元） | 上と同じ値 |

```bash
gcloud run services update track-talk-api --region asia-northeast1 \
  --update-env-vars "DATABASE_URL=...,SUPABASE_URL=..."
```

5. 画面の再デプロイが終わったら API の `/health` を確認し、ログイン（メールと Google の両方）、投稿、画像のアップロード、退会前の画面表示を通しで試す
6. 止めていたもの（API の受付、tick、`imports`）を戻す

### 3. 切り替え後

- 旧プロジェクトは**少なくとも 1 週間は消さず**に残し（書き込みは止まった状態）、問題があれば `DATABASE_URL` などを戻せるようにする
- 旧プロジェクトの最終 dump を手元に保管してから削除する
- リハーサルや切り替えで作った dump（`/var/folders/.../T/tmp.*`）を消す。本番のデータとパスワードのハッシュが入っている
- `docs/cloud-run-deploy.md` と `docs/launch.md` の Supabase に関する記述、`README.md` を更新する

## ロールバック

切り替え後に問題が出たら、上の表の値を旧プロジェクトのものに戻す。ただし切り替え後に新プロジェクトへ入った書き込みは旧に無いので、戻すのは**早い段階（数時間以内）に限る**。それ以降は新プロジェクト側で直す。

## 期待する効果

DB 往復 1 回あたり、東京↔シンガポール（約 70ms）から東京内（数 ms）になる。API が 1 リクエストで複数クエリを発行する画面ほど体感が速くなる。切り替え後に主要 API の応答時間を測って確かめる。

## 未決事項

- [x] 現在のリージョンと移行先の確認（シンガポール → 東京）
- [x] `public` の dump / restore のリハーサル（行数・版が一致）
- [x] Auth ユーザーを移す方式（A）の事前検証（データの一致とパスワードログインを確認）
- [x] 新 DB に向けた `go test`（失敗 4 件は移行と無関係）
- [ ] 新プロジェクトの Auth 設定（Site URL・Google ログイン・メール）を旧と揃え、画面から確認する
- [ ] 切り替え用の新プロジェクトを作り直す（プロジェクト数の上限を確認）
- [ ] 実施日時（利用の少ない時間帯）と、利用者への事前告知の有無
