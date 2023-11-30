.PHONY: migrate generate

pg-migrate:
	cd examples/pg/migrations; tern migrate

sqlc-generate:
	cd examples/pg; sqlc generate

pg-drop-all:
	cd examples/pg/migrations; tern migrate --destination 0

pg-reset-and-migrate: pg-drop-all pg-migrate

run-server-for-dev:
	cd examples; docker compose up -d; go run main.go