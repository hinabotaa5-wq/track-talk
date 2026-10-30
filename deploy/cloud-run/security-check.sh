#!/usr/bin/env bash
# 見慣れない DB 接続の見回りを Cloud Scheduler から15分ごとに呼ぶ設定。
# setup.sh のあと、Cloud Shell で一度だけ流す。流し直すと秘密の値を作り直して上書きする。
#   PROJECT_ID=<プロジェクトID> bash deploy/cloud-run/security-check.sh
set -euo pipefail

: "${PROJECT_ID:?PROJECT_ID を指定してください}"
REGION=asia-northeast1
API=track-talk-api
JOB=track-talk-security-check

gcloud config set project "$PROJECT_ID"
gcloud services enable cloudscheduler.googleapis.com

SECRET=$(openssl rand -hex 32)
API_URL=$(gcloud run services describe "$API" --region "$REGION" --format='value(status.url)')

echo "== API に秘密の値を入れる"
gcloud run services update "$API" --region "$REGION" --update-env-vars "SECURITY_CHECK_SECRET=$SECRET" --quiet

echo "== 15分ごとに呼ぶジョブ"
# Cloud Scheduler は月3ジョブまで無料。既存のジョブと合わせて超える分は1ジョブ約$0.10/月。
args=(--location "$REGION" --schedule "*/15 * * * *" --time-zone Asia/Tokyo
  --uri "$API_URL/internal/security/check" --http-method POST
  --headers "X-Security-Check-Secret=$SECRET" --attempt-deadline 60s)
# 設定を変えるときは作り直す。
if gcloud scheduler jobs describe "$JOB" --location "$REGION" >/dev/null 2>&1; then
  gcloud scheduler jobs delete "$JOB" --location "$REGION" --quiet
fi
gcloud scheduler jobs create http "$JOB" "${args[@]}"

echo "完了しました。通知先は API の OPS_ALERT_EMAIL（未設定ならメールは届きません）。止めるときは: gcloud scheduler jobs pause $JOB --location $REGION"
