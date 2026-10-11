.PHONY: api web migrate sqlc test-api test-web lint

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
