# API を Cloud Run（東京）で動かす

Go API を Google Cloud Run の東京リージョン（`asia-northeast1`）で動かす手順です。
アクセスがないときは 0 台まで縮むので、今の利用規模なら費用はほぼ通信料だけです。料金は変わるため、作成前に [Cloud Run の料金](https://cloud.google.com/run/pricing) を確認してください。

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

値は Render の Environment からそのまま写せます。`DATABASE_URL` も Render と同じ値で動きます（Cloud Run は IPv4 で外に出るので、Supabase の接続プーラーの URL が必要。Render で動いている値ならプーラーのはず）。
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
gcloud run services describe track-talk-api --region asia-northeast1               # 状態
```

- **環境変数を変える**: `gcloud run services update track-talk-api --region asia-northeast1 --update-env-vars KEY=VALUE`
- **予算超過で止まったとき**: 原因を確かめてから `gcloud run services update track-talk-api --region asia-northeast1 --ingress all` で再開する。予算は月ごとにリセットされるが、再開は自動では行わない
- **予算を変える**: コンソールの「お支払い → 予算とアラート」で `track-talk` を編集する
- **ロールバック**: コンソールの Cloud Run → `track-talk-api` → リビジョンで、前のリビジョンにトラフィックを戻す


## フォーム添削の tick

フォーム添削の打診（30分）と返却目安の期限切れは、Cloud Scheduler が1分ごとに API を呼んで片付けます。API は使われないときに0台まで縮むので、自分で時計を持てないためです。setup.sh のあと、Cloud Shell で一度だけ流します。

```bash
PROJECT_ID=<プロジェクトID> bash deploy/cloud-run/coaching-tick.sh
```

秘密の値を作って API の `COACHING_TICK_SECRET` に入れ、同じ値をヘッダーに付けて `/internal/coaching/tick` を呼ぶジョブ `track-talk-coaching-tick` を作ります。Cloud Scheduler は月3ジョブまで無料です。止めるときは `gcloud scheduler jobs pause track-talk-coaching-tick --location asia-northeast1`。

tick が止まっていても、依頼した人が依頼を開いたときに期限切れは片付き、期限の過ぎた打診は引き受けられません。ただ、時間切れの打診が次のコーチに回るのは tick だけです。

## 見慣れない DB 接続の見回り

API・imports・移行の DB 接続には `application_name` の名札（`track-talk-api` / `track-talk-import` / `track-talk-migrate`）が付きます。Cloud Scheduler が15分ごとに `/internal/security/check` を呼び、同じ DB ユーザーの接続のうち名札が許可リスト（自前の名札と Supabase 内部）にないものがあれば、`OPS_ALERT_EMAIL` へ「名札・接続開始時刻・状態」だけをメールします。クエリ本文や IP は送りません。setup.sh のあと、Cloud Shell で一度だけ流します。

```bash
PROJECT_ID=<プロジェクトID> bash deploy/cloud-run/security-check.sh
```

秘密の値を作って API の `SECURITY_CHECK_SECRET` に入れ、同じ値をヘッダーに付けて呼ぶジョブ `track-talk-security-check` を作ります。無料枠（月3ジョブ）を超える分は1ジョブ約$0.10/月です。止めるときは `gcloud scheduler jobs pause track-talk-security-check --location asia-northeast1`。Supabase 内部の接続で誤検知したら、その `application_name` を `security_check.go` の許可リストに足してください。15分ごとの見回りのため短い接続は見逃すことがあり、名札の偽装は DB ユーザーの用途別分離（未実施）で塞ぎます。

Secret Manager の「データ読み取り」監査ログと、想定外の読み出し・IAM 変更・SA キー作成のログアラートは、GCP コンソールで一度だけ作る設定です（手順は漏えい検知の実装計画 P1 4-A / 4-B）。


## 月1回の記録のお知らせ

通知をオンにした全員に、毎月1日の19時(日本時間)に「大会の記録、残せていますか？」をプッシュ通知で送ります。Cloud Scheduler が API を呼びます。`VAPID_PUBLIC_KEY`・`VAPID_PRIVATE_KEY`・`VAPID_SUBJECT` が入っていることが前提です。setup.sh のあと、Cloud Shell で一度だけ流します。

```bash
PROJECT_ID=<プロジェクトID> bash deploy/cloud-run/monthly-reminder.sh
```

秘密の値を作って API の `REMINDER_SECRET` に入れ、同じ値をヘッダーに付けて `/internal/monthly-reminder` を呼ぶジョブ `track-talk-monthly-reminder` を作ります。送った月は `monthly_reminders` に残り、同じ月に呼び直しても二度は送りません。止めるときは `gcloud scheduler jobs pause track-talk-monthly-reminder --location asia-northeast1`。

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
