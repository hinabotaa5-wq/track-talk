# Cloud Run Job track-talk-identity-export。金庫 DB を暗号文のまま輸出する（復号しない・鍵を持たない）。
# 本体バックアップの backup.py を共用するため、ビルドの起点は deploy/ にする:
#   docker build --platform linux/amd64 -f deploy/identity-vault/export/Dockerfile -t <イメージ> deploy
FROM postgres:17.11-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends python3 ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY cloud-run/db-backup/backup.py /app/backup.py
COPY identity-vault/export/export.py /app/export.py
RUN chmod 0444 /app/backup.py /app/export.py
USER postgres
ENTRYPOINT ["/usr/bin/python3", "/app/export.py"]
