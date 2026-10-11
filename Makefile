.PHONY: api web migrate sqlc test-api test-web lint

DATABASE_URL ?= postgres://tracktalk:tracktalk@localhost:5432/tracktalk?sslmode=disable

api:
	cd apps/api && DATABASE_URL=$(DATABASE_URL) go run ./cmd/api

web:
	cd apps/web && npm run dev

migrate:
	cd apps/api && DATABASE_URL=$(DATABASE_URL) go run ./cmd/migrate up

sqlc:
	cd apps/api && sqlc generate

test-api:
	cd apps/api && go test ./...

test-web:
	cd apps/web && npm test
