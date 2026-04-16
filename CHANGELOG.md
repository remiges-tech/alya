# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)

## [Unreleased]

## [0.36.0] - 2026-04-16

### Added
- Compile-time dependency injection support via the `di` package and the `alya-di` generator
- Config loader, provider, and watch support in `config`
- Environment-backed startup config loading via `config.NewEnv(...)`
- REST helper APIs in `restutils` for JSON binding, validation, problem responses, and standard responses
- Additive helper APIs in `wscutils` for `{"data": ...}` binding, path param parsing, and validator instances
- PATCH route registration in `service.Service`
- `examples/rest-usersvc-sqlc-example` with users and orders, SQLC, migrations, config loading, and compile-time DI wiring
- `examples/wsc-usersvc-sqlc-example` with users and orders, SQLC, optional GORM repository, migrations, and config loading

## [0.35.0] - 2026-02-20

### Fixed
- Batch summarization failing when multiple workers finish the last row of a batch simultaneously

### Added
- Crash recovery: workers detect dead peers via heartbeat expiry and reprocess abandoned rows
- Periodic sweep for batches that miss summarization due to race conditions

### Upgrade
Run migration 003 to add the required index:
```sql
CREATE INDEX IF NOT EXISTS idx_batches_status ON batches(status);
```

## [0.34.0] - 2025-01-15

### Fixed
- Race condition in TimeoutMiddleware that caused pod crashes under load
- Handler goroutine panics now recovered (previously crashed the process)

### Changed
- TimeoutMiddleware uses handler response when available after timeout instead of always returning 504
- Redis key format changed from `ALYA_BATCHSTATUS_{id}` to `ALYA_{id}_STATUS` for cluster compatibility

### Added
- BatchStatus API with Redis-cached summary (`jobs.BatchStatus()`)
- Client disconnect tracking in TimeoutMiddleware (`CtxKeyClientDisconnected`)
- Timeout and panic info integration with LogRequest middleware
- Benchmarks for TimeoutMiddleware stress testing

## [0.33.0] and earlier

See [git history](https://github.com/remiges-tech/alya/commits/main) for changes prior to v0.34.0.
