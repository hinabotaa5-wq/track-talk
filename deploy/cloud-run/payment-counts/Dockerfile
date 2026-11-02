# Cloud Run Job track-talk-payment-counts。counts.py の説明と実行手順は docs/cloud-run-deploy.md を参照。
FROM postgres:17.11-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends python3 ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY counts.py /app/counts.py
RUN chmod 0444 /app/counts.py
USER postgres
ENTRYPOINT ["/usr/bin/python3", "/app/counts.py"]
