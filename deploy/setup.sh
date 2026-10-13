#!/usr/bin/env bash
# Ubuntu (Oracle Cloud の VM) の初期設定。何度実行しても同じ結果になる。
#   scp deploy/setup.sh ubuntu@<IP>:  →  ssh して  bash setup.sh
set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  exec sudo -E bash "$0" "$@"
fi

APP_DIR=/opt/track-talk
DEPLOY_USER="${DEPLOY_USER:-${SUDO_USER:-ubuntu}}"
export DEBIAN_FRONTEND=noninteractive

echo "==> パッケージ更新と基本ツール"
apt-get update -y
apt-get install -y ca-certificates curl unattended-upgrades

echo "==> Docker"
if ! command -v docker >/dev/null 2>&1; then
  curl -fsSL https://get.docker.com | sh
fi
systemctl enable --now docker
usermod -aG docker "${DEPLOY_USER}"

echo "==> セキュリティ更新の自動適用（再起動は日本時間 04:00）"
cat >/etc/apt/apt.conf.d/20auto-upgrades <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
EOF
cat >/etc/apt/apt.conf.d/52track-talk-unattended <<'EOF'
Unattended-Upgrade::Automatic-Reboot "true";
Unattended-Upgrade::Automatic-Reboot-WithUsers "true";
Unattended-Upgrade::Automatic-Reboot-Time "19:00";
EOF

echo "==> スワップ（メモリ 2GB 未満のときだけ 2GB 作る）"
mem_kb="$(awk '/MemTotal/ {print $2}' /proc/meminfo)"
if [ "${mem_kb}" -lt 2000000 ] && ! swapon --show | grep -q .; then
  fallocate -l 2G /swapfile
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile
  grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >>/etc/fstab
fi

echo "==> アプリ用ディレクトリ"
install -d -o "${DEPLOY_USER}" -g "${DEPLOY_USER}" -m 755 "${APP_DIR}"

echo "==> 主要大会ホストの取り込み（4月・10月の1日 00:00 UTC）"
cat >/etc/cron.d/track-talk-importmajors <<EOF
0 0 1 4,10 * root cd ${APP_DIR} && docker compose --profile jobs run --rm importmajors >> /var/log/track-talk-importmajors.log 2>&1
EOF
chmod 644 /etc/cron.d/track-talk-importmajors

cat <<EOF

完了しました。次の作業:
  1. ${APP_DIR}/.env を作る（deploy/.env.example をコピーして値を入れる）
  2. GitHub の Actions から「Deploy API」を実行する
  ※ ${DEPLOY_USER} が docker を使うには再ログインが必要です
EOF
