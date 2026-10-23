.PHONY: db api web migrate seed-injury unseed-injury sqlc test-api test-web lint local local-down

# 開発用 DB(docker)。.env の DATABASE_URL が何であっても、これらのターゲットは必ずここに接続する。
# 本番に流すときは make を使わず、ALLOW_REMOTE_DB=1 を付けて直接実行すること。
LOCAL_DB ?= postgres://tracktalk:tracktalk@127.0.0.1:55432/tracktalk?sslmode=disable
export DATABASE_URL := $(LOCAL_DB)

db:
	docker compose up -d --wait postgres

api:
	cd apps/api && go run ./cmd/api

web:
	cd apps/web && npm run dev

migrate:
	cd apps/api && go run ./cmd/migrate up

# ケガ部のデモデータ(ローカルの docker の DB にだけ流す。何度流しても同じ結果になる)
seed-injury:
	docker compose exec -T postgres psql -U tracktalk -d tracktalk -v ON_ERROR_STOP=1 < apps/api/seeds/injury_demo.sql

# ケガ部のデモデータを消す(デモ用プロフィールだけを消す)
unseed-injury:
	docker compose exec -T postgres psql -U tracktalk -d tracktalk -v ON_ERROR_STOP=1 < apps/api/seeds/injury_demo_remove.sql

sqlc:
	cd apps/api && sqlc generate

test-api:
	cd apps/api && go test ./...

test-web:
	cd apps/web && npm test

# ローカル専用の認証・DB・API・Web を起動する。
local:
	docker compose -f docker-compose.preview.yml up -d --build --wait

local-down:
	docker compose -f docker-compose.preview.yml down
