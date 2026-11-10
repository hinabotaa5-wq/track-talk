# API を Cloud Run（東京）で動かす

Go API を Google Cloud Run の東京リージョン（`asia-northeast1`）で動かす手順です。
アクセスがないときは 0 台まで縮みます。実行時間・通信・関連サービスの費用は利用量で変わるため、作成前に [Cloud Run の料金](https://cloud.google.com/run/pricing) を確認してください。

```
ブラウザ ─ 画面（Netlify）
   └────── https://track-talk-api-xxxx.a.run.app ─ Cloud Run（東京、最大1台）─ Supabase（DB・Auth）
```

- **デプロイ**: マージしても自動では動かない。Actions の「Deploy API」を Run workflow で流すと、`.github/workflows/deploy-api.yml` が `main` からイメージを作って Cloud Run に送る
- **費用の上限**: 予算（既定は月100円）を超えると `budget-stop` が API をインターネットから呼べなくする。費用の反映は数時間遅れるので、少しはみ出すことはある
- **台数**: 最大1台。書き込みの回数制限がメモリ上にあるのと、急なアクセスで費用が膨らまないようにするため

## 1. プロジェクトを作る（手作業）

1. [Google Cloud コンソール](https://console.cloud.google.com/) でプロジェクトを作る
2. 請求先アカウント（カード）をつなぐ
3. 右上のアイコンから Cloud Shell を開き、このリポジトリを clone する

```bash
git clone https://github.com/hinabotaa5-wq/track-talk.git
cd track-talk
```

## 2. 環境変数を用意する

```bash
cp deploy/cloud-run/env.example.yaml deploy/cloud-run/env.yaml
nano deploy/cloud-run/env.yaml
```

設定名は雛形とAPIの設定コードを確認します。SupabaseのDB接続はIPv4で利用できる接続プーラーのURLを使います。秘密値をチャット・文書・Gitに転記しないでください。既存環境を更新するときは、一括置換を避けて必要な項目だけを更新します。
`CORS_ORIGINS` は画面の本番 URL（Netlify のサイト URL。末尾の `/` なし）です。

## 3. セットアップを流す

```bash
PROJECT_ID=<プロジェクトID> bash deploy/cloud-run/setup.sh deploy/cloud-run/env.yaml
```

請求先アカウントの通貨が日本円でないときは、`BUDGET_AMOUNT=1USD` のように通貨を合わせて指定します。

作られるもの:

| もの | 内容 |
| --- | --- |
| `track-talk-api` | API の Cloud Run サービス（公開、最大1台） |
| `track-talk-budget-stop` | 予算超過の通知を受けて API を止めるサービス（非公開） |
| 予算 `track-talk` | 50%・90%・100% で請求先の管理者にメール。100% で API を止める |
| Artifact Registry `track-talk` | イメージの置き場所。最新3つを残し、7日より古いものは消す |
| Workload Identity（`github`） | GitHub Actions が鍵なしでデプロイするための設定 |

最後に API の URL と、GitHub に登録する値が表示されます。

## 4. 本物の API に差し替える

setup.sh は、まず Google の見本イメージでサービスを立てます（Cloud Shell の docker や Cloud Build は、新しいプロジェクトだとネットワークや権限で失敗しやすいため）。
本物の API は GitHub Actions の「Deploy API」がビルドして差し替えます。Actions の画面から Run workflow で流します（マージでは自動で流れません）。

送り先は `.github/workflows/deploy-api.yml` の `env` に書いてあります（`GCP_PROJECT_ID` など。秘密の値ではない）。別のプロジェクトに作り直したら、setup.sh の最後に表示される値に書き換えます。

## 5. 切り替え

1. `curl https://<API の URL>/health` が `{"status":"ok"}` を返すことを確認する
2. 画面（Netlify）の `NEXT_PUBLIC_API_BASE_URL` を Cloud Run の URL にして再デプロイする
3. 画面からログイン、投稿、画像のアップロードができることを確認する
4. 問題なければ、前に使っていた API のサービスを削除する

## 運用

```bash
gcloud run services logs read track-talk-api --region asia-northeast1 --limit 100   # ログ
gcloud run services describe track-talk-api --region asia-northeast1 \
  --format='value(status.latestReadyRevisionName,status.url)' # リビジョンとURLだけ
```

- **環境変数を変える**: 非秘密の設定は `gcloud run services update track-talk-api --region asia-northeast1 --update-env-vars KEY=VALUE`。秘密はSecret Managerの数値バージョンへの参照を使い、実行サービスアカウントに対象の秘密だけの読み取り権限を付けます。
- **予算超過で止まったとき**: 原因を確かめてから `gcloud run services update track-talk-api --region asia-northeast1 --ingress all` で再開する。予算は月ごとにリセットされるが、再開は自動では行わない
- **予算を変える**: コンソールの「お支払い → 予算とアラート」で `track-talk` を編集する
- **ロールバック**: コンソールの Cloud Run → `track-talk-api` → リビジョンで、前のリビジョンにトラフィックを戻す


## フォーム添削の tick

フォーム添削の打診（引受期限90分）と返却目安の期限切れは、Cloud Scheduler が1分ごとに API を呼んで片付けます。API は使われないときに0台まで縮むので、自分で時計を持てないためです。setup.sh のあと、Cloud Shell で一度だけ流します。

```bash
PROJECT_ID=<プロジェクトID> bash deploy/cloud-run/coaching-tick.sh
```

秘密の値を作って API の `COACHING_TICK_SECRET` に入れ、同じ値をヘッダーに付けて `/internal/coaching/tick` を呼ぶジョブ `track-talk-coaching-tick` を作ります。Schedulerの料金・無料枠は作成前に確認してください。止めるときは `gcloud scheduler jobs pause track-talk-coaching-tick --location asia-northeast1`。

tick が止まっていても、依頼した人が依頼を開いたときに期限切れの処理を行い、期限の過ぎた打診は引き受けられません。指名制では、辞退・打診期限切れの依頼は終了してカードの仮押さえ取消しまたはPayPayの全額返金を行い、別のコーチへ再割当てしません。利用者は独立した新規申込みとして改めて申し込めます。tick は期限処理に加え、決済の確定・取消し・返金の再試行も担うため、画面を開いたときの処理だけで代用しません。

## 見慣れない DB 接続の見回り

API・imports・移行の DB 接続には `application_name` の名札（`track-talk-api` / `track-talk-import` / `track-talk-migrate`）が付きます。Cloud Scheduler が15分ごとに `/internal/security/check` を呼び、同じ DB ユーザーの接続のうち名札が許可リスト（自前の名札と Supabase 内部）にないものがあれば、`OPS_ALERT_EMAIL` へ「名札・接続開始時刻・状態」だけをメールします。クエリ本文や IP は送りません。setup.sh のあと、Cloud Shell で一度だけ流します。

```bash
PROJECT_ID=<プロジェクトID> bash deploy/cloud-run/security-check.sh
```

秘密の値を作って API の `SECURITY_CHECK_SECRET` に入れ、同じ値をヘッダーに付けて呼ぶジョブ `track-talk-security-check` を作ります。Schedulerの料金・無料枠は作成前に確認してください。止めるときは `gcloud scheduler jobs pause track-talk-security-check --location asia-northeast1`。Supabase 内部の接続で誤検知したら、その `application_name` を `security_check.go` の許可リストに足してください。15分ごとの見回りのため短い接続は見逃すことがあり、名札は偽装可能で、用途別のDBユーザー分離などの権限制御の代わりにはなりません。

Secret Managerの「データ読み取り」監査ログと、想定外の読み出し・IAM変更・SAキー作成のログアラートは、GCP側で設定します。アプリコードのデプロイだけでは設定されません。

### Stripe の見覚えのない返金・送金

通常 Webhook（`/webhooks/stripe`）で `charge.refunded`・`refund.created`・`transfer.created`・`transfer.reversed`・`payout.created` を追加で受け、DB に記録のない返金・送金を `OPS_ALERT_EMAIL` へ知らせます。コーチングを止めている間も検知だけは続けます。取りこぼし対策として、上の見回りが1日1回、過去25時間の返金・送金も Stripe から一覧して照合します。

送信先を設定・変更するときは、Stripe ダッシュボード → 開発者 → Webhook → 本番の通常エンドポイントで上の5イベントを追加します（署名シークレットは変わりません）。そのあとテストモードで少額の返金をして、メールが届くか確認してください。

`payout.created`（運営の銀行への入金）は、`OPS_PAYOUT_EXPECTED_LAST4` に期待する入金先口座の末尾4桁を設定したときだけ宛先を照合し、違う口座への入金なら知らせます。未設定ならログだけです。値はSecret Managerに保存し、Cloud Runの `OPS_PAYOUT_EXPECTED_LAST4` は承認した数値バージョンへの参照として設定します。実行サービスアカウントには対象の秘密だけのSecret Accessor権限を付けます。実値を平文の環境変数・コマンド引数・文書・設定例に書かず、確認は参照先名・バージョン・設定状態だけで行います。メール本文にも末尾4桁を含めません。

## 月1回の記録のお知らせ

通知をオンにした全員に、毎月1日の19時(日本時間)に「大会の記録を残せていますか？」をプッシュ通知で送ります。Cloud Scheduler が API を呼びます。`VAPID_PUBLIC_KEY`・`VAPID_PRIVATE_KEY`・`VAPID_SUBJECT` が入っていることが前提です。setup.sh のあと、Cloud Shell で一度だけ流します。

```bash
PROJECT_ID=<プロジェクトID> bash deploy/cloud-run/monthly-reminder.sh
```

秘密の値を作って API の `REMINDER_SECRET` に入れ、同じ値をヘッダーに付けて `/internal/monthly-reminder` を呼ぶジョブ `track-talk-monthly-reminder` を作ります。送った月は `monthly_reminders` に残り、同じ月に呼び直しても二度は送りません。止めるときは `gcloud scheduler jobs pause track-talk-monthly-reminder --location asia-northeast1`。

## DB とメディアのバックアップ

Cloud Run Job `track-talk-db-backup` が毎日3時30分（日本時間、Cloud Scheduler）に DB を読み取り専用でダンプし、バックアップ用バケットの `database/` に置きます。日曜だけ、続けて Cloudinary の全メディアを `media/` に写します。バケットは30日で古いものを消します。コードは `deploy/cloud-run/db-backup/` にあります。

使う設定は、`DATABASE_URL`（秘密 `track-talk-db-backup-url` への参照）、`BACKUP_BUCKET`、`CLOUDINARY_BACKUP`（秘密 `track-talk-cloudinary-backup` への参照。`cloud`・`key`・`secret` を持つ JSON で、Admin API の一覧と非公開メディアのダウンロードを使います。認証は Admin API の宛先にだけ付け、公開メディアの配信 URL は認証なしで取得します）です。Cloudinary のキーを作り直したら、この秘密にも新しいバージョンを足してください。古いキーのままだとメディアだけが `http_status: 401` で失敗します。

失敗ログ（`database_backup_failed`／`media_backup_failed`）には、エラーの種類・HTTP ステータス・宛先ホスト・どの段階か（`list`／`download`／`upload`／`manifest`）だけを残します。URL のクエリ・署名・応答本文・認証情報は残しません。

イメージを更新するときは次のとおりです。

```bash
docker build --platform linux/amd64 -t asia-northeast1-docker.pkg.dev/<プロジェクトID>/track-talk/db-backup:<日付-版> deploy/cloud-run/db-backup
docker push asia-northeast1-docker.pkg.dev/<プロジェクトID>/track-talk/db-backup:<日付-版>
gcloud run jobs update track-talk-db-backup --region asia-northeast1 --image asia-northeast1-docker.pkg.dev/<プロジェクトID>/track-talk/db-backup:<日付-版>
```

日曜以外にメディアまで確かめるときは、その実行だけ `MEDIA_BACKUP_FORCE=1` を付けて手動で流します。

```bash
gcloud run jobs execute track-talk-db-backup --region asia-northeast1 --update-env-vars MEDIA_BACKUP_FORCE=1 --wait
```

`database_backup_completed` と `media_backup_completed` の両方が出れば成功です。手元のテストは `cd deploy/cloud-run/db-backup && python3 -m unittest test_backup` です。

## 金庫 DB の輸出

Cloud Run Job `track-talk-identity-export` が毎日3時45分（日本時間、Cloud Scheduler）に、身元専用サービスの金庫 DB（`identity_vault` スキーマ）を読み取り専用でダンプし、本体バックアップと同じバケットの `identity-vault/YYYY-MM-DD/<時刻>-<uuid>/` に `database.dump`・`roles.sql`・`manifest.json` の順で置きます。manifest が完了の目印で、置き場・照合（テーブルごとの件数と行ダイジェスト、アップロード後の md5）は本体バックアップと同じです。コードは `deploy/identity-vault/export/` にあり、本体の `backup.py` を共用します。

輸出は保存されたままの値を写すだけで、氏名・住所は暗号文のまま出ます。復号はせず、鍵は持ちません（`COACH_IDENTITY_KEY` がある環境では起動を拒みます）。接続は `export-role.sql` の読み取り専用ロールの URL を、秘密 `track-talk-identity-export-url` への参照で `DATABASE_URL` に渡します。本体 DB を指していると止まります。失敗ログ（`identity_export_failed`）は本体と同じく種類と理由の記号だけです。

```bash
docker build --platform linux/amd64 -f deploy/identity-vault/export/Dockerfile -t asia-northeast1-docker.pkg.dev/<プロジェクトID>/track-talk/identity-export:<日付-版> deploy
```

ジョブとスケジュールは `deploy/identity-vault/export-schedule.sh` で作ります（スケジュールは一時停止で作成）。取り寄せたセットの照合は、空のローカル DB に戻して manifest と比べます。

```bash
VERIFY_DATABASE_URL=postgres://<ユーザー>@localhost:<ポート>/<空の DB> python3 deploy/identity-vault/export/export.py verify <取り寄せたディレクトリ>
```

`identity_export_verified` なら一致、`identity_export_verify_failed` なら終了コード1です。手元のテストは `cd deploy/identity-vault/export && TEST_ADMIN_DATABASE_URL=<ローカルの管理接続> python3 -m unittest test_export` です。

## 本体 DB の migration（コマンドで適用）

公開 API は `DATABASE_MIGRATIONS_EXTERNAL=true` で起動するため、起動時に migration を流しません。本番の migration は Cloud Run Job `track-talk-db-migrate` だけが当てます。ダッシュボードや SQL Editor の手作業は要りません。

- イメージは API と同じもの（Deploy API が作る `api:<コミット>`）で、`/app/dbmigrate` を `--command` で起動します。新しい migration をマージしたら、Deploy API で作られたイメージにジョブを差し替えてから実行します
- 接続は公開 API の旧接続と同じ秘密（migration を流してきた所有者ロール）の版を `DATABASE_URL` として参照します。値はジョブの秘密参照にだけあり、起動する人の手元・引数・ログには出ません。サービスアカウントはこのジョブ専用で、その秘密1つの読み取りだけを持ちます
- 出力は1行の JSON（`event: db_migrate`）で、適用前後の版・dirty・適用件数・適用後の確認（真偽と件数）だけです。接続先・SQL・エラーの原文は出しません
- 冪等です。適用済みなら `applied: 0` で終わります。実行前が dirty なら何も当てずに終了コード3（`dirty_before_apply`）、本番がイメージより新しいときも終了コード3（`database_ahead_of_image`）で止まります。dirty の解消は人の判断が要るので、ジョブは自動で行いません
- 引数 `check` を付けると、何も当てずに現在の版・dirty・未適用の件数だけを出します

```bash
gcloud iam service-accounts create track-talk-db-migrate --display-name "TrackTalk DB migrate job"
gcloud secrets add-iam-policy-binding <所有者ロールの接続の秘密> \
  --member serviceAccount:track-talk-db-migrate@<プロジェクトID>.iam.gserviceaccount.com --role roles/secretmanager.secretAccessor
gcloud run jobs create track-talk-db-migrate --region asia-northeast1 \
  --image asia-northeast1-docker.pkg.dev/<プロジェクトID>/track-talk/api:<コミット> --command /app/dbmigrate \
  --service-account track-talk-db-migrate@<プロジェクトID>.iam.gserviceaccount.com \
  --set-secrets DATABASE_URL=<所有者ロールの接続の秘密>:<版> \
  --tasks 1 --max-retries 0 --task-timeout 600
# 読み取りだけ（版・未適用の件数）
gcloud run jobs execute track-talk-db-migrate --region asia-northeast1 --args check --wait
# 適用（未適用分を当てて、適用後の確認まで）
gcloud run jobs execute track-talk-db-migrate --region asia-northeast1 --wait
# 新しい migration のときはイメージを差し替えてから実行する
gcloud run jobs update track-talk-db-migrate --region asia-northeast1 \
  --image asia-northeast1-docker.pkg.dev/<プロジェクトID>/track-talk/api:<コミット>
```

結果はジョブのログ（`textPayload` の1行 JSON）で読みます。`result` が `ok` で、`after_version` が `expected_version` と同じ、`after_dirty` が `false` なら成功です。000154 以降は `checks` に、000154 の2つの関数（メール一致・合成コーチ ID）（`identity_boundary.coach_matches_email`）が SECURITY DEFINER・search_path 固定で、公開 API 用ロールと PUBLIC から実行できず、身元専用サービスの本体読み取りロール（`identity_boundary.mark_retired` の実行権を持つロール）だけが実行できることが出ます。手元のテストは `cd apps/api && DATABASE_URL=<ローカル DB> go test ./cmd/dbmigrate` です。

## 金庫 DB の migration（コマンドで適用）

Cloud Run Job `track-talk-identity-vault-migrate` が、`deploy/identity-vault/migrations/*.sql` を名前順に1ファイルずつのトランザクションで金庫 DB に当て、適用後の確認を1行の JSON（`event: identity_vault_migrate`）で出します。SQL Editor の手作業は要りません。migration はどれも何度流しても同じ結果になるように書きます（このジョブは毎回すべてのファイルを流します）。コードは `deploy/identity-vault/migrate/` にあります。

- 接続は金庫の migration 所有者の URL（sslmode=require 以上）を、秘密 `track-talk-identity-vault-migrate-url` への参照で `VAULT_MIGRATE_DATABASE_URL` に渡します。この秘密の値の登録だけは運営者が行います（金庫の接続文字列は運営者しか持たないため）。サービスアカウントはこのジョブ専用で、その秘密1つの読み取りだけを持ちます
- 出力は適用したファイル数と、確認（合成用の関数がそろい SECURITY DEFINER・search_path 固定であること、PUBLIC・API 用ロール・輸出用ロールが実行できないこと、身元専用サービスのアプリ用ロールだけが実行できること、管理者行の件数）の真偽と件数だけです。SQL・値・エラーの原文は出しません

```bash
docker build --platform linux/amd64 -f deploy/identity-vault/migrate/Dockerfile -t asia-northeast1-docker.pkg.dev/<プロジェクトID>/track-talk/identity-vault-migrate:<日付-版> deploy/identity-vault
gcloud iam service-accounts create tt-vault-migrate --display-name "TrackTalk identity vault migrate job"
gcloud secrets add-iam-policy-binding track-talk-identity-vault-migrate-url \
  --member serviceAccount:tt-vault-migrate@<プロジェクトID>.iam.gserviceaccount.com --role roles/secretmanager.secretAccessor
gcloud run jobs create track-talk-identity-vault-migrate --region asia-northeast1 \
  --image asia-northeast1-docker.pkg.dev/<プロジェクトID>/track-talk/identity-vault-migrate:<日付-版> \
  --service-account tt-vault-migrate@<プロジェクトID>.iam.gserviceaccount.com \
  --set-secrets VAULT_MIGRATE_DATABASE_URL=track-talk-identity-vault-migrate-url:<版> \
  --tasks 1 --max-retries 0 --task-timeout 600
gcloud run jobs execute track-talk-identity-vault-migrate --region asia-northeast1 --wait
```

手元のテストは `cd deploy/identity-vault/migrate && TEST_VAULT_ADMIN_DATABASE_URL=<ローカルの金庫相当の管理接続> python3 -m unittest test_migrate` です（接続先が 127.0.0.1 以外なら TLS が要ります）。

## 合成データ専用のプロビジョニング（provision／verify／cleanup／walkthrough）

Cloud Run Job `track-talk-identity-provision` は、本番経路の通し確認に使う**合成**コーチの金庫側の用意と後片付けを、コマンドだけで行います。ダッシュボードでのユーザー作成・SQL Editor での INSERT／UPDATE・画面ログインは要りません。イメージは身元専用サービスと同じもので、`/usr/local/bin/identityprovision` を `--command` で起動します。

```bash
gcloud run jobs execute track-talk-identity-provision --region asia-northeast1 --args provision,<本体コーチ ID>,<タグ> --wait
gcloud run jobs execute track-talk-identity-provision --region asia-northeast1 --args verify,<本体コーチ ID>,<タグ> --wait
gcloud run jobs execute track-talk-identity-provision --region asia-northeast1 --args walkthrough,<本体コーチ ID>,<タグ> --wait
gcloud run jobs execute track-talk-identity-provision --region asia-northeast1 --args cleanup,<本体コーチ ID>,<タグ> --wait
# 合成コーチの本体 ID を引く（承認済みで、合成メールが一致するときだけ）
gcloud run jobs execute track-talk-identity-provision --region asia-northeast1 --args lookup,<タグ> --wait
# 合成コーチの退会（本体の画面操作）の後、終了経路を確かめる
gcloud run jobs execute track-talk-identity-provision --region asia-northeast1 --args endcheck,<本体コーチ ID>,<タグ> --wait
```

- lookup：本体の `identity_boundary.synthetic_coach_id` で、合成メールのアカウントが承認済みコーチならその ID を出します（合成の標識が無いアカウントの ID は返しません）
- endcheck：本体で承認済みでなくなったこと・身元の保持（`identity_boundary.retention`）に載ったこと・身元専用サービスの身元画面が 403 になることを確かめます。退会そのもの（`/coach/me/withdraw`）は本体のセッションが要るため、本体の画面操作で行います
- legacy-cleanup（一回限り）：運営者決定（2026-11-10）で削除を許可された旧 aaa の割り当て（本体コーチ ID が `2877c3f1-` で始まる、管理者でない1件）と、その金庫 Auth ユーザーだけを、合成の標識を問わずに消します。1件に定まらなければ止まります

- 合成メールは、ジョブの設定値 `SYNTHETIC_EMAIL_BASE`（運営者が管理する受信箱のアドレス。公開リポジトリには書かない）の @ の前に `+synthetic-<タグ>` を足した形だけです。タグは英小文字と数字の32文字以内です。起動の引数にメールは渡しません
- provision：本体の `identity_boundary.coach_matches_email` で「そのコーチが承認済みで、アカウントのメールが合成メールと一致する」ことを確かめます。確かめられたら、金庫 Auth の合成ユーザーを作り（既にあれば使い回し）、その ID を subject とする members 行（`is_admin=false`、coach_id＝入力）を作り、自己検証（subject 一致・合成・管理者行0）まで行います。冪等です
- verify：members 行の件数・subject 一致・合成かどうか・管理者行の件数・身元行の件数を、真偽と件数だけで出します。subject の値・メールは出しません
- cleanup：割り当ての subject が合成で管理者でないときだけ、members 行・そのコーチの身元行（identities・commerce・disclosures）・合成の金庫 Auth ユーザーを消し、件数を出します。access_log は監査のため残します
- walkthrough：合成ユーザーのパスワードを乱数で付け直し、TOTP を登録し直して、二段階認証（aal2）までをジョブの中で済ませます。そのうえで身元専用サービスの読み取り→架空の身元の登録→読み戻しの一致→公開 API に氏名・住所・電話が出ないこと→管理者行と管理者の二段階認証の件数、を確かめます。パスワード・TOTP の設定キー・コード・架空の値・トークンは、ジョブのメモリにだけあり、出力・ログには出しません
- ガード（ジョブ側と金庫側の関数の両方で強制）：合成パターン外のメールは何も作らない／本体側の一致が確かめられなければ作らない／`is_admin=true` の行は作れない／管理者行・合成でない subject の行は作らない・変えない・消さない／自動の付け替えはしない（ずれていたら cleanup→provision）
- 秘密（参照のみ）：`IDENTITY_DATABASE_URL`・`IDENTITY_MAIN_READ_DATABASE_URL`・`IDENTITY_AUTH_PUBLIC_KEY`（身元専用サービスと同じ版）と、金庫 Auth の管理用キー `track-talk-identity-auth-admin-key`（`IDENTITY_AUTH_ADMIN_KEY`。値の登録だけは運営者）。設定値：`IDENTITY_AUTH_URL`・`SYNTHETIC_EMAIL_BASE`・`IDENTITY_SERVICE_URL`・`PUBLIC_API_URL`
- 出力は1行の JSON（`event: identity_synthetic`、操作名・成否・件数・真偽・HTTP ステータス・時刻）だけです

手元の統合テスト（公式の Supabase Auth と Postgres を外に出られないネットワークで動かす）は `internal/identityprovision/provision_integration_test.go` の冒頭の説明のとおりで、環境変数が無いときは skip します。

## 決済の件数確認（読み取り専用）

Cloud Run Job `track-talk-payment-counts` は、決済まわりのテーブルの状態別件数と孤立決済の件数だけを、1行の JSON（`event: payment_counts`）でログに出して終わります。行の中身・ID・金額・Stripe の ID は取得も出力もしません。SQL は `SELECT count(*)` と `GROUP BY` だけで、読み取り専用トランザクションで実行してロールバックします。スケジュールは無く、手動で流すときだけ動きます。コードは `deploy/cloud-run/payment-counts/` にあります。

接続はバックアップと同じ秘密 `track-talk-db-backup-url` を `DATABASE_URL` として参照し、同じサービスアカウントで動かします。新しい秘密は作りません。

```bash
docker build --platform linux/amd64 -t asia-northeast1-docker.pkg.dev/<プロジェクトID>/track-talk/payment-counts:<日付-版> deploy/cloud-run/payment-counts
docker push asia-northeast1-docker.pkg.dev/<プロジェクトID>/track-talk/payment-counts:<日付-版>
gcloud run jobs create track-talk-payment-counts --region asia-northeast1 \
  --image asia-northeast1-docker.pkg.dev/<プロジェクトID>/track-talk/payment-counts:<日付-版> \
  --service-account <バックアップと同じサービスアカウント> \
  --set-secrets DATABASE_URL=track-talk-db-backup-url:<バックアップと同じ版> \
  --tasks 1 --max-retries 0 --task-timeout 300
gcloud run jobs execute track-talk-payment-counts --region asia-northeast1 --wait
```

新しい migration を本番に当てたあとは、`counts.py` の `EXPECTED_MIGRATION` を同じ版に上げ、イメージを作り直してジョブのイメージを差し替えます。差し替えるまでは終了コード2で止まります。

```bash
gcloud run jobs update track-talk-payment-counts --region asia-northeast1 \
  --image asia-northeast1-docker.pkg.dev/<プロジェクトID>/track-talk/payment-counts:<日付-版>
```

`result` が `clear` なら孤立・想定外の状態は0件、`attention` なら `orphans`・`flagged_states`・`unexpected_values` のどれかが0でありません。本番のスキーマが想定（`counts.py` の `EXPECTED_MIGRATION`）と違うときは件数を取らず `payment_counts_schema_mismatch` を出して終了コード2で止まります。手元のテストは `cd deploy/cloud-run/payment-counts && python3 -m unittest test_counts` です。

## トラブル

- **画面が「読み込みに失敗しました」「通信に失敗しました」になるのに、API のログには `200` が出ている**: API の `CORS_ORIGINS` に、サイトの URL が入っていない。ブラウザが API の返事を捨てている。次で直す（URL は `https://` から始め、最後に `/` を付けない。複数あるときは、カンマで区切って、コマンドの引数を `"^@^CORS_ORIGINS=https://a.example,https://b.example"` のように書く。先頭の `^@^` は、値の中のカンマを許すための指定）

  ```bash
  gcloud run services update track-talk-api --region asia-northeast1 \
    --update-env-vars CORS_ORIGINS=https://<サイトの URL>
  ```

  いまの値を見るには（ほかの値は表示されない）:

  ```bash
  gcloud run services describe track-talk-api --region asia-northeast1 --format=json \
    | jq -r '.spec.template.spec.containers[0].env[] | select(.name=="CORS_ORIGINS") | .value'
  ```

### 返金・相殺台帳を使い始めた後のロールバック

migration 000127以降の返金・調整・適用・除外が存在するとき、downは拒否します。実際の返金や送金はDBスキーマの巻き戻しでは取り消せません。取引発生後は前進修正を基本とし、旧APIリビジョンへ戻すだけでは安全と扱いません（旧コードは返金済み報酬の除外台帳を参照しません）。戻す必要がある場合は、先に送金・返金と関連する定期処理を止め、Stripeとアプリの操作・残高・処理中状態を照合し、台帳を保全した復旧手順を個別に確定してください。

遅延返金失敗の継続照合は、Stripeが銀行からの返却に最大30日かかると説明する期間に7日の余裕を加え、成功記録後37日間を対象とします（[Stripe公式](https://docs.stripe.com/refunds#handle-failed-refunds)）。`refund.updated`・`refund.failed`の受信追加も投入時に設定確認してください。tick停止中は定期照合は動かず、再開は機能開放時の別判断です。
