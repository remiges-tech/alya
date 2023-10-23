.PHONY: migrate generate

pg-migrate:
	cd internal/pg/migrations; tern migrate

sqlc-generate:
	cd internal/pg; sqlc generate

pg-reset:
	cd internal/pg/migrations; tern migrate --destination -+1

pg-reset-and-migrate: pg-reset pg-migrate