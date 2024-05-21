.PHONY: migrate generate

pg-migrate:
	cd pg/migrations; tern migrate

sqlc-generate:
	cd pg; sqlc generate

pg-drop-all:
	cd pg/migrations; tern migrate --destination 0

pg-reset-and-migrate: pg-drop-all pg-migrate

run-server-for-dev:
	docker compose up -d; go run main.go