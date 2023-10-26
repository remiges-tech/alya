.PHONY: migrate generate

pg-migrate:
	cd internal/pg/migrations; tern migrate

sqlc-generate:
	cd internal/pg; sqlc generate

pg-drop-all:
	cd internal/pg/migrations; tern migrate --destination 0

pg-reset-and-migrate: pg-drop-all pg-migrate

run-server-for-dev:
	docker compose up -d; go run cmd/server/main.go