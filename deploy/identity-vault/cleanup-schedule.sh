#!/usr/bin/env bash
# 金庫の終了後始末（POST /internal/cleanup）を Cloud Scheduler から毎日呼ぶ設定。
# 収集開始までは停止したままにする（paused で作成し、ops #42 の受け入れ後に resume）。
# 既にあるジョブに流し直した場合も paused に戻る（収集開始後に流し直したら resume し直す）。
# 対象コード: apps/api/internal/identityvault/retention.go の cleanup()/cleanupCoach()
# （終了から5年（JST）経過した身元・開示を削除。ハンドラの上限は3分）。
#
# 使い方（運営者が実行。AI は実行しない）:
#   PROJECT_ID=<プロジェクトID> IDENTITY_CLEANUP_TOKEN=<H5で配備した値> bash deploy/identity-vault/cleanup-schedule.sh
# 秘密の扱い（§5 準拠）: このスクリプトは配備済みサービスの環境変数を読み取らない。
# トークンは運営者が用意した環境変数 IDENTITY_CLEANUP_TOKEN からのみ受け取り、
# 表示・記録しない（set -x 禁止、値の echo なし）。未設定時は値に触れず中断する。
# gcloud の引数リスト（ps で見える）にも載せないよう、ヘッダーは本人だけが読める一時ファイル
# 経由の --flags-file で渡し、終了時に消す。ジョブ作成・更新の結果表示も出さない（--format=none）。
set -euo pipefail

: "${PROJECT_ID:?PROJECT_ID を指定してください}"
REGION=asia-northeast1
SERVICE=track-talk-identity
JOB=track-talk-identity-cleanup
# 頻度は暫定（本体 storage-cleanup の日次に合わせ、時刻だけずらした）。
# ops 決定があれば SCHEDULE を差し替えて流し直す。
SCHEDULE='0 5 * * *'
ZONE=Asia/Tokyo

if [[ -z "${IDENTITY_CLEANUP_TOKEN:-}" ]]; then
  echo 'IDENTITY_CLEANUP_TOKEN が未設定のため中断（値自体は表示・記録しない。配備 H5 の手順で発行した値を運営者が環境変数に設定して流し直す）' >&2
  exit 2
fi
# 一時ファイル（YAML）に安全に書ける文字だけを受け付ける。値は表示しない。
if [[ ! "$IDENTITY_CLEANUP_TOKEN" =~ ^[A-Za-z0-9._~+/=-]{32,}$ ]]; then
  echo 'IDENTITY_CLEANUP_TOKEN の形式が想定外のため中断（32文字以上の英数字と ._~+/=- のみ。値自体は表示・記録しない）' >&2
  exit 2
fi

gcloud services enable cloudscheduler.googleapis.com --project "$PROJECT_ID" >/dev/null

SERVICE_URL="$(gcloud run services describe "$SERVICE" --project "$PROJECT_ID" --region "$REGION" --format='value(status.url)')"
if [[ -z "$SERVICE_URL" ]]; then
  echo "サービス $SERVICE の URL を取得できなかったため中断" >&2
  exit 1
fi

args=(--project "$PROJECT_ID" --location "$REGION"
  --schedule "$SCHEDULE" --time-zone "$ZONE"
  --uri "$SERVICE_URL/internal/cleanup" --http-method POST
  --attempt-deadline 180s)

flags_file="$(umask 077 && mktemp)"
trap 'rm -f "$flags_file"' EXIT
# 設定を変えるときは作り直さず更新する（ヘッダーは更新用フラグで付け替える）。
if gcloud scheduler jobs describe "$JOB" --project "$PROJECT_ID" --location "$REGION" >/dev/null 2>&1; then
  printf -- '--update-headers: "Authorization=Bearer %s"\n' "$IDENTITY_CLEANUP_TOKEN" >"$flags_file"
  gcloud scheduler jobs update http "$JOB" "${args[@]}" --flags-file "$flags_file" --format=none --quiet
else
  printf -- '--headers: "Authorization=Bearer %s"\n' "$IDENTITY_CLEANUP_TOKEN" >"$flags_file"
  gcloud scheduler jobs create http "$JOB" "${args[@]}" --flags-file "$flags_file" --format=none --quiet
fi
rm -f "$flags_file"
# 収集開始までは停止のままにする。開始後に resume する。
gcloud scheduler jobs pause "$JOB" --project "$PROJECT_ID" --location "$REGION" --quiet

echo '金庫の終了後始末ジョブを作成し、収集開始まで停止（paused）にしました。開始後に resume してください。'
