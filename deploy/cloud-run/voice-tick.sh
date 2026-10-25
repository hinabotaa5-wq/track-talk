#!/usr/bin/env bash
# 明示的に実行したときだけ、開発/本番プロジェクトに音声接続整理のジョブを作る。
# PROJECT_ID=... bash deploy/cloud-run/voice-tick.sh
set -euo pipefail
: "${PROJECT_ID:?PROJECT_ID を指定してください}"
VOICE_REGION=${VOICE_REGION:-asia-northeast1}
VOICE_SERVICE=${VOICE_SERVICE:-track-talk-api}
VOICE_JOB=${VOICE_JOB:-track-talk-voice-tick}
VOICE_TICK_SECRET=$(openssl rand -hex 32)
VOICE_API_URL=$(gcloud run services describe "$VOICE_SERVICE" --project "$PROJECT_ID" --region "$VOICE_REGION" --format='value(status.url)')
gcloud services enable cloudscheduler.googleapis.com --project "$PROJECT_ID"
gcloud run services update "$VOICE_SERVICE" --project "$PROJECT_ID" --region "$VOICE_REGION" --update-env-vars "MEET_VOICE_TICK_SECRET=$VOICE_TICK_SECRET" --quiet
args=(--project "$PROJECT_ID" --location "$VOICE_REGION" --schedule '* * * * *' --time-zone Asia/Tokyo --uri "$VOICE_API_URL/internal/voice/tick" --http-method POST --headers "X-Voice-Tick-Secret=$VOICE_TICK_SECRET" --attempt-deadline 60s)
if gcloud scheduler jobs describe "$VOICE_JOB" --project "$PROJECT_ID" --location "$VOICE_REGION" >/dev/null 2>&1; then
  gcloud scheduler jobs update http "$VOICE_JOB" "${args[@]}"
else
  gcloud scheduler jobs create http "$VOICE_JOB" "${args[@]}"
fi
gcloud scheduler jobs run "$VOICE_JOB" --project "$PROJECT_ID" --location "$VOICE_REGION"
echo '設定しました。管理画面で定期確認の実行時刻を確認してください。'
