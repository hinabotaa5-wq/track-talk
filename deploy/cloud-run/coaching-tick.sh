#!/usr/bin/env bash
# フォーム添削の tick（打診の30分切れ・返却目安切れの処理）を Cloud Scheduler から1分ごとに呼ぶ設定。
# setup.sh のあと、Cloud Shell で一度だけ流す。流し直すと秘密の値を作り直して上書きする。
#   PROJECT_ID=<プロジェクトID> bash deploy/cloud-run/coaching-tick.sh
set -euo pipefail

: "${PROJECT_ID:?PROJECT_ID を指定してください}"
REGION=asia-northeast1
API=track-talk-api
JOB=track-talk-coaching-tick

gcloud config set project "$PROJECT_ID"
gcloud services enable cloudscheduler.googleapis.com

SECRET=$(openssl rand -hex 32)
API_URL=$(gcloud run services describe "$API" --region "$REGION" --format='value(status.url)')

echo "== API に秘密の値を入れる"
gcloud run services update "$API" --region "$REGION" --update-env-vars "COACHING_TICK_SECRET=$SECRET" --quiet

echo "== 1分ごとに呼ぶジョブ"
# Cloud Scheduler は月3ジョブまで無料。
args=(--location "$REGION" --schedule "* * * * *" --time-zone Asia/Tokyo
  --uri "$API_URL/internal/coaching/tick" --http-method POST
  --headers "X-Coaching-Tick-Secret=$SECRET" --attempt-deadline 60s)
# 設定を変えるときは作り直す。
if gcloud scheduler jobs describe "$JOB" --location "$REGION" >/dev/null 2>&1; then
  gcloud scheduler jobs delete "$JOB" --location "$REGION" --quiet
fi
gcloud scheduler jobs create http "$JOB" "${args[@]}"

echo "完了しました。止めるときは: gcloud scheduler jobs pause $JOB --location $REGION"
