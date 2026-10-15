# API を Cloud Run（東京）で動かす

Go API を Google Cloud Run の東京リージョン（`asia-northeast1`）で動かす手順です。
アクセスがないときは 0 台まで縮むので、今の利用規模なら費用はほぼ通信料だけです。料金は変わるため、作成前に [Cloud Run の料金](https://cloud.google.com/run/pricing) を確認してください。

```
ブラウザ ─ 画面（Vercel / Netlify）
   └────── https://track-talk-api-xxxx.a.run.app ─ Cloud Run（東京、最大1台）─ Supabase（DB・Auth）
```

- **デプロイ**: `main` の `apps/api` が変わると、`.github/workflows/deploy-api.yml` がイメージを作って Cloud Run に送る
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
`CORS_ORIGINS` は画面の本番 URL です。Vercel と Netlify を並行で使う間は、カンマ区切りで両方書きます。

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

## 4. GitHub に登録する（手作業）

リポジトリの Settings → Secrets and variables → Actions → **Variables** に、表示された3つを登録します（Secret ではなく Variable）。

| Variable | 値 |
| --- | --- |
| `GCP_PROJECT_ID` | プロジェクト ID |
| `GCP_WORKLOAD_IDENTITY_PROVIDER` | `projects/<番号>/locations/global/workloadIdentityPools/github/providers/github` |
| `GCP_SERVICE_ACCOUNT` | `github-deploy@<プロジェクトID>.iam.gserviceaccount.com` |

登録後、Actions の「Deploy API」を Run workflow で一度流し、成功することを確認します。

## 5. 切り替え

1. `curl https://<API の URL>/health` が `{"status":"ok"}` を返すことを確認する
2. 画面（Vercel、のちに Netlify）の `NEXT_PUBLIC_API_BASE_URL` を Cloud Run の URL にして再デプロイする
3. 画面からログイン、投稿、画像のアップロードができることを確認する
4. 問題なければ Render のサービスを削除し、`render.yaml` を消す

## 運用

```bash
gcloud run services logs read track-talk-api --region asia-northeast1 --limit 100   # ログ
gcloud run services describe track-talk-api --region asia-northeast1               # 状態
```

- **環境変数を変える**: `gcloud run services update track-talk-api --region asia-northeast1 --update-env-vars KEY=VALUE`
- **予算超過で止まったとき**: 原因を確かめてから `gcloud run services update track-talk-api --region asia-northeast1 --ingress all` で再開する。予算は月ごとにリセットされるが、再開は自動では行わない
- **予算を変える**: コンソールの「お支払い → 予算とアラート」で `track-talk` を編集する
- **ロールバック**: コンソールの Cloud Run → `track-talk-api` → リビジョンで、前のリビジョンにトラフィックを戻す

## トラブル

- **docker のビルドで `go mod download` が `connection refused` になる**: Cloud Shell の docker のネットワークの問題。setup.sh は自動で Cloud Build に切り替える
- **Cloud Build が `storage.objects.get` の 403 で止まる**: 新しいプロジェクトでは、ビルドを動かすアカウント（`<プロジェクト番号>-compute@developer.gserviceaccount.com`）に権限が無い。次を実行する（このアカウントに、Cloud Build の作業に必要な権限を与える）

  ```bash
  gcloud projects add-iam-policy-binding <プロジェクトID> \
    --member="serviceAccount:<プロジェクト番号>-compute@developer.gserviceaccount.com" \
    --role="roles/cloudbuild.builds.builder"
  ```
