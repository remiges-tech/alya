# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)

## [Unreleased]

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
