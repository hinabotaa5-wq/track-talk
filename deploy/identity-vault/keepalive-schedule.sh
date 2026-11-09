#!/usr/bin/env bash
# 金庫サービスの生存確認（GET /health）を Cloud Scheduler から6時間ごとに呼ぶ設定。
# 役割はサービス（Cloud Run）の生存確認だけ。/health は DB に触れないため、PREMISES 第4節の
# vault DB への読み取り keep-alive（Supabase Free の自動停止対策）の代わりにはならない。
# DB 読み取りの keep-alive（H3 の ping 専用ロール）は別件で、このスクリプトでは作らない。
# 認証情報は不要・個人情報に触れないため、収集開始前から有効のままにする。
# 対象コード: apps/api/internal/identityvault/service.go の GET /health（204）。
#   PROJECT_ID=<プロジェクトID> bash deploy/identity-vault/keepalive-schedule.sh
set -euo pipefail

: "${PROJECT_ID:?PROJECT_ID を指定してください}"
REGION=asia-northeast1
SERVICE=track-talk-identity
JOB=track-talk-identity-keepalive

gcloud services enable cloudscheduler.googleapis.com --project "$PROJECT_ID" >/dev/null

SERVICE_URL=$(gcloud run services describe "$SERVICE" --project "$PROJECT_ID" --region "$REGION" --format='value(status.url)')
if [[ -z "$SERVICE_URL" ]]; then
  echo "サービス $SERVICE の URL を取得できなかったため中断" >&2
  exit 1
fi

args=(--project "$PROJECT_ID" --location "$REGION" --schedule "0 */6 * * *" --time-zone Asia/Tokyo
  --uri "$SERVICE_URL/health" --http-method GET
  --attempt-deadline 60s)
# 設定を変えるときは作り直す。
if gcloud scheduler jobs describe "$JOB" --project "$PROJECT_ID" --location "$REGION" >/dev/null 2>&1; then
  gcloud scheduler jobs delete "$JOB" --project "$PROJECT_ID" --location "$REGION" --quiet
fi
gcloud scheduler jobs create http "$JOB" "${args[@]}"
# 生存確認は収集開始前も有効のまま（停止しない）。
gcloud scheduler jobs resume "$JOB" --project "$PROJECT_ID" --location "$REGION" --quiet

echo "完了しました。6時間ごとに金庫サービスの /health を呼びます（生存確認のみ。DB 読み取りの keep-alive ではありません）。止めるときは: gcloud scheduler jobs pause $JOB --project $PROJECT_ID --location $REGION"
