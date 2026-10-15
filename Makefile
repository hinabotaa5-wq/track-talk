.PHONY: db api web migrate sqlc test-api test-web lint

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

sqlc:
	cd apps/api && sqlc generate

test-api:
	cd apps/api && go test ./...

test-web:
	cd apps/web && npm test
