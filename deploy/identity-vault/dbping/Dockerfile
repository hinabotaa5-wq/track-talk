# Cloud Run Job track-talk-identity-dbping。金庫 DB の keep-alive（H3）用の件数だけの読み取り。
# 値も暗号文も出さない・書かない・輸出しない。復号の鍵は持たない。鍵素材がある環境では起動しない。
# 本体バックアップの backup.py を共用するため、ビルドの起点は deploy/ にする:
#   docker build --platform linux/amd64 -f deploy/identity-vault/dbping/Dockerfile -t <イメージ> deploy
FROM postgres:17.11-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends python3 ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY cloud-run/db-backup/backup.py /app/backup.py
COPY identity-vault/dbping/dbping.py /app/dbping.py
RUN chmod 0444 /app/backup.py /app/dbping.py
USER postgres
ENTRYPOINT ["/usr/bin/python3", "/app/dbping.py"]
