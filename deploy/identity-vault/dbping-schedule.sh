#!/usr/bin/env bash
# 金庫 DB の keep-alive（H3）ジョブ（Cloud Run Job track-talk-identity-dbping）と、それを起動する
# Cloud Scheduler を作る／更新する。PREMISES 第4節の「1日数回」の読み取り用に 1 日 4 回
#（日本時間 2:15・8:15・14:15・20:15。3:30 の本体バックアップ・3:45 の輸出・5:00 の掃除を避ける）。
# 既存の /health の keep-alive（サービスの生存確認、DB に触れない）とは別物で、置き換えない。
# スケジュールは作成時に一時停止で作る。初回の手動実行で identity_ping_completed を確かめてから
# `gcloud scheduler jobs resume track-talk-identity-dbping --location asia-northeast1` で動かす。
#
#   PROJECT_ID=<プロジェクトID> IMAGE=<イメージ> \
#   JOB_SERVICE_ACCOUNT=<keep-alive 用 SA> SCHEDULER_SERVICE_ACCOUNT=<起動用 SA> SECRET_VERSION=<版> \
#   bash deploy/identity-vault/dbping-schedule.sh
#
# 接続 URL は秘密 track-talk-identity-dbping-url（dbping-role.sql のロール）への参照だけで渡す。
# このスクリプトは秘密の値・サービスの環境変数を読まず、表示もしない。鍵（COACH_IDENTITY_KEY）は渡さない。
# ロールと秘密の用意は運営者の参加点（export と同じく、移行オーナーでの SQL 実行と秘密の登録）。
set -euo pipefail
: "${PROJECT_ID:?PROJECT_ID を指定してください}"
: "${IMAGE:?IMAGE を指定してください}"
: "${JOB_SERVICE_ACCOUNT:?JOB_SERVICE_ACCOUNT を指定してください}"
: "${SCHEDULER_SERVICE_ACCOUNT:?SCHEDULER_SERVICE_ACCOUNT を指定してください}"
: "${SECRET_VERSION:?SECRET_VERSION を指定してください}"

REGION=asia-northeast1
JOB=track-talk-identity-dbping
SECRET=track-talk-identity-dbping-url

job_args=(--project "$PROJECT_ID" --region "$REGION" --image "$IMAGE"
  --service-account "$JOB_SERVICE_ACCOUNT"
  --set-secrets "DATABASE_URL=${SECRET}:${SECRET_VERSION}"
  --tasks 1 --max-retries 0 --task-timeout 300 --quiet)
if gcloud run jobs describe "$JOB" --project "$PROJECT_ID" --region "$REGION" --format='value(metadata.name)' >/dev/null 2>&1; then
  gcloud run jobs update "$JOB" "${job_args[@]}" >/dev/null
  echo "ジョブ $JOB を更新しました。"
else
  gcloud run jobs create "$JOB" "${job_args[@]}" >/dev/null
  echo "ジョブ $JOB を作成しました。"
fi

uri="https://run.googleapis.com/v2/projects/${PROJECT_ID}/locations/${REGION}/jobs/${JOB}:run"
schedule_args=(--project "$PROJECT_ID" --location "$REGION"
  --schedule '15 2,8,14,20 * * *' --time-zone 'Asia/Tokyo'
  --uri "$uri" --http-method POST
  --oauth-service-account-email "$SCHEDULER_SERVICE_ACCOUNT"
  --attempt-deadline 180s --quiet)
if gcloud scheduler jobs describe "$JOB" --project "$PROJECT_ID" --location "$REGION" --format='value(name)' >/dev/null 2>&1; then
  # 更新では一時停止・再開の状態を変えない。
  gcloud scheduler jobs update http "$JOB" "${schedule_args[@]}" >/dev/null
  echo "スケジュール $JOB を更新しました（状態は変えていません）。"
else
  gcloud scheduler jobs create http "$JOB" "${schedule_args[@]}" >/dev/null
  gcloud scheduler jobs pause "$JOB" --project "$PROJECT_ID" --location "$REGION" --quiet >/dev/null
  echo "スケジュール $JOB を一時停止の状態で作成しました。初回の手動実行で確かめてから resume してください。"
fi
