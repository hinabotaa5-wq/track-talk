#!/usr/bin/env bash
# Cloud Run（東京）に API を初めて立てるときに一度だけ流す。手順は docs/cloud-run-deploy.md。
# Cloud Shell で、このリポジトリを clone したディレクトリから実行する:
#   PROJECT_ID=<プロジェクトID> bash deploy/cloud-run/setup.sh deploy/cloud-run/env.yaml
# 途中で失敗しても、直してから流し直せば作成済みのものは飛ばす。
set -euo pipefail

: "${PROJECT_ID:?PROJECT_ID を指定してください}"
ENV_FILE="${1:?環境変数のファイル（env.example.yaml をコピーしたもの）を指定してください}"
REGION=asia-northeast1
GITHUB_REPO="${GITHUB_REPO:-hinabotaa5-wq/track-talk}"
# 請求先アカウントの通貨で書く（日本円なら JPY）。
BUDGET_AMOUNT="${BUDGET_AMOUNT:-100JPY}"

API=track-talk-api
STOPPER=track-talk-budget-stop
REPO=track-talk
IMAGE="$REGION-docker.pkg.dev/$PROJECT_ID/$REPO/api:initial"

gcloud config set project "$PROJECT_ID"
PROJECT_NUMBER=$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')
BILLING_ACCOUNT=$(gcloud billing projects describe "$PROJECT_ID" --format='value(billingAccountName)')
BILLING_ACCOUNT=${BILLING_ACCOUNT#billingAccounts/}
if [ -z "$BILLING_ACCOUNT" ]; then
  echo "このプロジェクトに請求先アカウントがつながっていません" >&2
  exit 1
fi

sa() { echo "$1@$PROJECT_ID.iam.gserviceaccount.com"; }
ensure_sa() {
  gcloud iam service-accounts describe "$(sa "$1")" >/dev/null 2>&1 ||
    gcloud iam service-accounts create "$1" --display-name "$2"
}

echo "== API を有効にする"
gcloud services enable \
  run.googleapis.com artifactregistry.googleapis.com iam.googleapis.com \
  iamcredentials.googleapis.com sts.googleapis.com pubsub.googleapis.com \
  cloudbilling.googleapis.com billingbudgets.googleapis.com

echo "== イメージの置き場所（古いものは自動で消す）"
gcloud artifacts repositories describe "$REPO" --location "$REGION" >/dev/null 2>&1 ||
  gcloud artifacts repositories create "$REPO" --repository-format docker --location "$REGION"
POLICY=$(mktemp)
cat >"$POLICY" <<'JSON'
[
  {"name": "keep-recent", "action": {"type": "Keep"}, "mostRecentVersions": {"keepCount": 3}},
  {"name": "delete-old", "action": {"type": "Delete"}, "condition": {"tagState": "any", "olderThan": "7d"}}
]
JSON
gcloud artifacts repositories set-cleanup-policies "$REPO" --location "$REGION" --policy "$POLICY" --no-dry-run
rm -f "$POLICY"

echo "== サービスアカウント"
ensure_sa track-talk-api "TrackTalk API (runtime)"
ensure_sa budget-stop "TrackTalk budget stop (runtime)"
ensure_sa github-deploy "TrackTalk deploy from GitHub Actions"

echo "== 初回のイメージを作って送る"
gcloud auth configure-docker "$REGION-docker.pkg.dev" --quiet
if gcloud artifacts docker images describe "$IMAGE" >/dev/null 2>&1; then
  echo "初回のイメージは作成済みなので飛ばします"
# Cloud Shell の docker は、ビルド中に Go のモジュール取得が "connection refused" で失敗することがある。
# ホストのネットワークで試し、それでも駄目なら Cloud Build に任せる。
elif docker build --network host -t "$IMAGE" apps/api; then
  docker push "$IMAGE"
else
  echo "docker でのビルドに失敗したので、Cloud Build で作り直します"
  gcloud services enable cloudbuild.googleapis.com
  # 新しいプロジェクトでは、ビルドを動かすサービスアカウントに権限が無く、
  # "storage.objects.get" の 403 で止まることがある。その場合は docs/cloud-run-deploy.md の「トラブル」を見る。
  gcloud builds submit apps/api --config deploy/cloud-run/cloudbuild.yaml \
    --substitutions "_IMAGE=$IMAGE" --region "$REGION"
fi

echo "== API を立てる"
# 最大1台: 回数制限がメモリ上にあるのと、急なアクセスで費用が膨らまないようにするため。
gcloud run deploy "$API" \
  --image "$IMAGE" --region "$REGION" \
  --service-account "$(sa track-talk-api)" \
  --env-vars-file "$ENV_FILE" \
  --port 8080 --cpu 1 --memory 512Mi \
  --min-instances 0 --max-instances 1 \
  --cpu-boost --allow-unauthenticated --quiet
API_URL=$(gcloud run services describe "$API" --region "$REGION" --format='value(status.url)')

echo "== 予算超過で API を止める budget-stop を立てる"
gcloud run deploy "$STOPPER" \
  --image "$IMAGE" --region "$REGION" \
  --service-account "$(sa budget-stop)" \
  --command /app/budgetstop \
  --set-env-vars "TARGET_SERVICE=projects/$PROJECT_ID/locations/$REGION/services/$API" \
  --cpu 1 --memory 128Mi --min-instances 0 --max-instances 1 \
  --no-allow-unauthenticated --quiet
STOPPER_URL=$(gcloud run services describe "$STOPPER" --region "$REGION" --format='value(status.url)')
# budget-stop が API の設定を変えられるようにする。
gcloud run services add-iam-policy-binding "$API" --region "$REGION" \
  --member "serviceAccount:$(sa budget-stop)" --role roles/run.developer --quiet >/dev/null
gcloud iam service-accounts add-iam-policy-binding "$(sa track-talk-api)" \
  --member "serviceAccount:$(sa budget-stop)" --role roles/iam.serviceAccountUser --quiet >/dev/null
# Pub/Sub が budget-stop を呼べるようにする。
gcloud run services add-iam-policy-binding "$STOPPER" --region "$REGION" \
  --member "serviceAccount:$(sa budget-stop)" --role roles/run.invoker --quiet >/dev/null

echo "== 予算と通知"
gcloud pubsub topics describe budget-alerts >/dev/null 2>&1 || gcloud pubsub topics create budget-alerts
gcloud pubsub subscriptions describe budget-stop >/dev/null 2>&1 ||
  gcloud pubsub subscriptions create budget-stop --topic budget-alerts \
    --push-endpoint "$STOPPER_URL" --push-auth-service-account "$(sa budget-stop)"
if [ -z "$(gcloud billing budgets list --billing-account "$BILLING_ACCOUNT" --filter 'displayName=track-talk' --format='value(name)')" ]; then
  # 50%・90%・100% で請求先の管理者にメールが届き、100% に達すると budget-stop が API を止める。
  gcloud billing budgets create --billing-account "$BILLING_ACCOUNT" \
    --display-name track-talk --budget-amount "$BUDGET_AMOUNT" \
    --filter-projects "projects/$PROJECT_ID" \
    --threshold-rule percent=0.5 --threshold-rule percent=0.9 --threshold-rule percent=1.0 \
    --notifications-rule-pubsub-topic "projects/$PROJECT_ID/topics/budget-alerts"
fi

echo "== GitHub Actions から送れるようにする"
gcloud iam workload-identity-pools describe github --location global >/dev/null 2>&1 ||
  gcloud iam workload-identity-pools create github --location global --display-name GitHub
gcloud iam workload-identity-pools providers describe github --location global --workload-identity-pool github >/dev/null 2>&1 ||
  gcloud iam workload-identity-pools providers create-oidc github \
    --location global --workload-identity-pool github \
    --issuer-uri https://token.actions.githubusercontent.com \
    --attribute-mapping 'google.subject=assertion.sub,attribute.repository=assertion.repository' \
    --attribute-condition "assertion.repository=='$GITHUB_REPO'"
gcloud iam service-accounts add-iam-policy-binding "$(sa github-deploy)" \
  --role roles/iam.workloadIdentityUser \
  --member "principalSet://iam.googleapis.com/projects/$PROJECT_NUMBER/locations/global/workloadIdentityPools/github/attribute.repository/$GITHUB_REPO" \
  --quiet >/dev/null
gcloud artifacts repositories add-iam-policy-binding "$REPO" --location "$REGION" \
  --member "serviceAccount:$(sa github-deploy)" --role roles/artifactregistry.writer --quiet >/dev/null
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member "serviceAccount:$(sa github-deploy)" --role roles/run.developer --condition None --quiet >/dev/null
for runtime in track-talk-api budget-stop; do
  gcloud iam service-accounts add-iam-policy-binding "$(sa "$runtime")" \
    --member "serviceAccount:$(sa github-deploy)" --role roles/iam.serviceAccountUser --quiet >/dev/null
done

cat <<EOF

完了しました。

API の URL: $API_URL
  確認: curl $API_URL/health

GitHub の Settings → Secrets and variables → Actions → Variables に登録する値:
  GCP_PROJECT_ID                  $PROJECT_ID
  GCP_WORKLOAD_IDENTITY_PROVIDER  projects/$PROJECT_NUMBER/locations/global/workloadIdentityPools/github/providers/github
  GCP_SERVICE_ACCOUNT             $(sa github-deploy)
EOF
