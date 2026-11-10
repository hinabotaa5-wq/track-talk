# Cloud Run Job track-talk-identity-vault-migrate。金庫 DB に deploy/identity-vault/migrations/*.sql を
# 名前順に当て、適用後の確認を1行の JSON で出す（ops #63）。ビルドの起点は deploy/identity-vault にする:
#   docker build --platform linux/amd64 -f deploy/identity-vault/migrate/Dockerfile -t <イメージ> deploy/identity-vault
FROM postgres:17.11-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends python3 ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY migrate/migrate.py /app/migrate.py
COPY migrations/ /app/migrations/
RUN chmod 0444 /app/migrate.py /app/migrations/*.sql
USER postgres
ENTRYPOINT ["/usr/bin/python3", "/app/migrate.py"]
