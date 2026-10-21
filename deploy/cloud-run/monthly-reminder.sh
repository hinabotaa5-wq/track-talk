#!/usr/bin/env bash
# 月1回の記録のお知らせ(プッシュ通知)を、Cloud Scheduler から毎月1日の19時(日本時間)に呼ぶ設定。
# setup.sh のあと、Cloud Shell で一度だけ流す。流し直すと秘密の値を作り直して上書きする。
#   PROJECT_ID=<プロジェクトID> bash deploy/cloud-run/monthly-reminder.sh
set -euo pipefail

: "${PROJECT_ID:?PROJECT_ID を指定してください}"
REGION=asia-northeast1
API=track-talk-api
JOB=track-talk-monthly-reminder

gcloud config set project "$PROJECT_ID"
gcloud services enable cloudscheduler.googleapis.com

SECRET=$(openssl rand -hex 32)
API_URL=$(gcloud run services describe "$API" --region "$REGION" --format='value(status.url)')

echo "== API に秘密の値を入れる"
gcloud run services update "$API" --region "$REGION" --update-env-vars "REMINDER_SECRET=$SECRET" --quiet

echo "== 毎月1日の19時に呼ぶジョブ"
# Cloud Scheduler は月3ジョブまで無料(coaching-tick と合わせて2つ)。
# 同じ月に呼び直しても API が二度は送らないので、失敗したときの再試行はそのままでよい。
args=(--location "$REGION" --schedule "0 19 1 * *" --time-zone Asia/Tokyo
  --uri "$API_URL/internal/monthly-reminder" --http-method POST
  --headers "X-Reminder-Secret=$SECRET" --attempt-deadline 300s)
# 設定を変えるときは作り直す。
if gcloud scheduler jobs describe "$JOB" --location "$REGION" >/dev/null 2>&1; then
  gcloud scheduler jobs delete "$JOB" --location "$REGION" --quiet
fi
gcloud scheduler jobs create http "$JOB" "${args[@]}"

echo "完了しました。止めるときは: gcloud scheduler jobs pause $JOB --location $REGION"
