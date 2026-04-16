# WSC users and orders SQLC example

This example shows Alya envelope style web services with two resources and interchangeable SQLC or GORM repository adapters:

- `users`
- `orders`

Theme: keep the transport contract in Alya envelope format, and organize the example with the production `cmd/` and `internal/` layout.

## Transport contract

- Request bodies use Alya's `{"data": ...}` envelope.
- Responses use Alya's `status`, `data`, and `messages` envelope.
- Handlers use typed dependencies and constructor injection.
- The example uses LogHarbour for basic request and startup logs.

## Layers

```text
HTTP request
    |
    v
internal/http/*_handler.go
    |
    v
internal/service/*_service.go
    |
    v
internal/repository/*_repository.go
    |
    v
internal/db/{sqlc,gorm}
    |
    v
PostgreSQL
```

## Project layout

```text
wsc-usersvc-sqlc-example/
  cmd/
    service/
      main.go

  internal/
    api/         request and response types
    http/        Gin handlers and route registration
    service/     use cases and business rules
    repository/  repository interfaces and SQLC/GORM implementations
    validation/  validator setup and Alya msgid/errcode mapping
    db/          migrations, queries, SQLC config, generated code, providers

  config.json      configuration file
  docker-compose.yaml
  README.md
```

## Configuration

This example uses Alya's `config` package for startup configuration.

It supports two startup sources:
- `file` -> load from `config.json`
- `env` -> load from environment variables

`CONFIG_SOURCE` selects the startup source.
If it is empty, the example uses `file`.

### File configuration

The default configuration file is `config.json`:

```json
{
  "database": {
    "host": "localhost",
    "port": 5432,
    "user": "alyatest",
    "password": "alyatest",
    "dbname": "alyatest"
  },
  "server": {
    "port": 8084
  }
}
```

Use a custom file path with `CONFIG_FILE`:

```bash
CONFIG_FILE=/etc/wsc-usersvc/config.json go run ./cmd/service
```

### Environment configuration

Set `CONFIG_SOURCE=env` to load startup config from environment variables.

This example uses the prefix `ALYA_WSC_USERSVC`.

Variables:
- `ALYA_WSC_USERSVC_DATABASE_HOST`
- `ALYA_WSC_USERSVC_DATABASE_PORT`
- `ALYA_WSC_USERSVC_DATABASE_USER`
- `ALYA_WSC_USERSVC_DATABASE_PASSWORD`
- `ALYA_WSC_USERSVC_DATABASE_DBNAME`
- `ALYA_WSC_USERSVC_SERVER_PORT`

Example:

```bash
CONFIG_SOURCE=env \
ALYA_WSC_USERSVC_DATABASE_HOST=localhost \
ALYA_WSC_USERSVC_DATABASE_PORT=5432 \
ALYA_WSC_USERSVC_DATABASE_USER=alyatest \
ALYA_WSC_USERSVC_DATABASE_PASSWORD=alyatest \
ALYA_WSC_USERSVC_DATABASE_DBNAME=alyatest \
ALYA_WSC_USERSVC_SERVER_PORT=8084 \
go run ./cmd/service
```

### Other environment variables

- `ALYA_REPOSITORY_BACKEND` -> set to `gorm` to use GORM instead of SQLC
- `CONFIG_SOURCE` -> `file` or `env`
- `CONFIG_FILE` -> path to custom config file when `CONFIG_SOURCE=file`

## Start PostgreSQL

```bash
cd examples/wsc-usersvc-sqlc-example
docker compose up -d
```

Check status:

```bash
docker compose ps
```

## Run migrations

Install `tern` if needed:

```bash
go install github.com/jackc/tern/v2@latest
```

Run migrations:

```bash
cd examples/wsc-usersvc-sqlc-example/internal/db/migrations
tern migrate
```

## Regenerate SQLC code

Install `sqlc` if needed:

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

Regenerate from the local DB directory:

```bash
cd examples/wsc-usersvc-sqlc-example/internal/db
sqlc generate
```

## Run the service

Run with file-based startup config and SQLC, which are both the defaults:

```bash
cd examples/wsc-usersvc-sqlc-example
go run ./cmd/service
```

Run with file-based startup config and GORM:

```bash
cd examples/wsc-usersvc-sqlc-example
ALYA_REPOSITORY_BACKEND=gorm go run ./cmd/service
```

Run with environment-based startup config and SQLC:

```bash
cd examples/wsc-usersvc-sqlc-example
CONFIG_SOURCE=env \
ALYA_WSC_USERSVC_DATABASE_HOST=localhost \
ALYA_WSC_USERSVC_DATABASE_PORT=5432 \
ALYA_WSC_USERSVC_DATABASE_USER=alyatest \
ALYA_WSC_USERSVC_DATABASE_PASSWORD=alyatest \
ALYA_WSC_USERSVC_DATABASE_DBNAME=alyatest \
ALYA_WSC_USERSVC_SERVER_PORT=8084 \
go run ./cmd/service
```

The service listens on the configured server port.

## Routes

Users:
- `POST /users`
- `GET /users`
- `GET /users/{id}`
- `PATCH /users/{id}`
- `DELETE /users/{id}`

Orders:
- `POST /orders`
- `GET /orders`
- `GET /orders/{id}`
- `PATCH /orders/{id}`
- `DELETE /orders/{id}`

## Example requests

Create a user:

```bash
curl -i -X POST http://localhost:8084/users \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "name": "Sachin",
      "email": "sachin@example.com",
      "username": "sachin"
    }
  }'
```

Create an order for user `1`:

```bash
curl -i -X POST http://localhost:8084/orders \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "user_id": 1,
      "number": "ORD1001",
      "status": "pending",
      "total_amount": 2500
    }
  }'
```

List users:

```bash
curl -i http://localhost:8084/users
```

List orders:

```bash
curl -i http://localhost:8084/orders
```

Update a user:

```bash
curl -i -X PATCH http://localhost:8084/users/1 \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "email": "sachin+new@example.com"
    }
  }'
```

Update an order:

```bash
curl -i -X PATCH http://localhost:8084/orders/1 \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "status": "paid",
      "total_amount": 3000
    }
  }'
```

Delete a user:

```bash
curl -i -X DELETE http://localhost:8084/users/1
```

Delete an order:

```bash
curl -i -X DELETE http://localhost:8084/orders/1
```

## Notes

- This example uses `wscutils.BindData` for request binding.
- Validator setup lives in `internal/validation/validator.go`.
- `internal/validation.NewUserValidator()` and `internal/validation.NewOrderValidator()` build additive `wscutils.Validator` instances.
- The handlers use `wscutils.NewSuccessResponse`, `wscutils.NewResponse`, and `wscutils.SendSuccessResponse` for Alya envelope responses.
- `cmd/service/main.go` initializes LogHarbour and wires the dependencies.
- `cmd/service/main.go` uses Alya's `config.NewFile`, `config.NewEnv`, and `config.LoadWith` for startup config.
- Only the backend selection in `cmd/service/main.go` changes between SQLC and GORM; the service and handler layers stay the same.
- Set `ALYA_REPOSITORY_BACKEND=gorm` to use the GORM repository adapter.
- The database providers, migrations, queries, SQLC config, and generated code live under `internal/db/`.
