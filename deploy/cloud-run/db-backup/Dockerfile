# Cloud Run Job track-talk-db-backup。backup.py の説明と更新手順は docs/cloud-run-deploy.md を参照。
FROM postgres:17.11-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends python3 ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY backup.py /app/backup.py
RUN chmod 0444 /app/backup.py
USER postgres
ENTRYPOINT ["/usr/bin/python3", "/app/backup.py"]
