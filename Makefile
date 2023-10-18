.PHONY: migrate generate

pg-migrate:
	cd internal/pg/migrations; tern migrate

sqlc-generate:
	cd internal/pg; sqlc generate