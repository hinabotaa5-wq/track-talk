#!/usr/bin/env bash
# 日次の保存期限処理を設定する。API の新しい版を公開してから実行する。
# PROJECT_ID=refined-bolt-493709-a8 bash deploy/cloud-run/storage-cleanup.sh
# 秘密の値は表示せず、既存設定があるときは同じ値を使う。
set -euo pipefail
: "${PROJECT_ID:?PROJECT_ID を指定してください}"
export PROJECT_ID
python3 - <<'PY'
import json, os, secrets, subprocess

project = os.environ['PROJECT_ID']
region = 'asia-northeast1'
job = 'track-talk-storage-cleanup'
def run(*args):
    return subprocess.run(['gcloud', *args], check=True, text=True, stdout=subprocess.PIPE).stdout

service = json.loads(run('run', 'services', 'describe', 'track-talk-api', '--project', project, '--region', region, '--format=json'))
values = {item['name']: item.get('value', '') for item in service['spec']['template']['spec']['containers'][0].get('env', [])}
secret = values.get('STORAGE_CLEANUP_SECRET') or secrets.token_urlsafe(32)
if not values.get('STORAGE_CLEANUP_SECRET'):
    run('run', 'services', 'update', 'track-talk-api', '--project', project, '--region', region,
        '--update-env-vars', 'STORAGE_CLEANUP_SECRET='+secret, '--quiet')
run('services', 'enable', 'cloudscheduler.googleapis.com', '--project', project)
existing = subprocess.run(['gcloud', 'scheduler', 'jobs', 'describe', job, '--project', project, '--location', region], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0
action = 'update' if existing else 'create'
headers_flag = '--update-headers' if existing else '--headers'
run('scheduler', 'jobs', action, 'http', job, '--project', project, '--location', region,
    '--schedule', '0 4 * * *', '--time-zone', 'Asia/Tokyo',
    '--uri', service['status']['url']+'/internal/storage/cleanup', '--http-method', 'POST',
    headers_flag, 'X-Storage-Cleanup-Secret='+secret, '--attempt-deadline', '180s', '--quiet')
print('日本時間の毎日4時に保存期限の処理を実行する設定が完了しました。')
PY
