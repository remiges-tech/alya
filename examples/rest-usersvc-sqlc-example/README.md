# REST users and orders SQLC example

This example shows a REST-style Alya service with two resources:

- `users`
- `orders`

It keeps:

- HTTP handling in `transport/`
- business logic in `app/`
- repository contracts and SQLC-backed implementation in `repository/`
- schema, SQL queries, SQLC config, generated code, and DB provider in `pg/`

## Layers

```text
HTTP request
    |
    v
transport/*_handler.go
    |
    v
app/*_service.go
    |
    v
repository/sqlc_repository.go
    |
    v
pg/sqlc-gen
    |
    v
PostgreSQL
```

## Resource layout

```text
/users   -> transport/user_handler.go  -> app/user_service.go  -> repository user methods
/orders  -> transport/order_handler.go -> app/order_service.go -> repository order methods
```

## Request flow

```text
client
  -> Gin route
  -> restutils.BindBody + validation
  -> app service
  -> repository interface
  -> SQLC Queries
  -> PostgreSQL
  -> SQLC result
  -> repository model
  -> API response
  -> HTTP response
```

## Project layout

```text
rest-usersvc-sqlc-example/
  api/         request and response types
  app/         use cases and business rules
  pg/          migrations, queries, SQLC config, generated code, provider
  repository/  repository interfaces and SQLC implementation
  transport/   Gin handlers and route registration
  main.go      wiring
  config.json  startup configuration file
  docker-compose.yaml
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
    "port": 8083
  }
}
```

Use a custom file path with `CONFIG_FILE`:

```bash
CONFIG_FILE=/etc/rest-usersvc/config.json go run .
```

### Environment configuration

Set `CONFIG_SOURCE=env` to load startup config from environment variables.

This example uses the prefix `ALYA_REST_USERSVC`.

Variables:
- `ALYA_REST_USERSVC_DATABASE_HOST`
- `ALYA_REST_USERSVC_DATABASE_PORT`
- `ALYA_REST_USERSVC_DATABASE_USER`
- `ALYA_REST_USERSVC_DATABASE_PASSWORD`
- `ALYA_REST_USERSVC_DATABASE_DBNAME`
- `ALYA_REST_USERSVC_SERVER_PORT`

Example:

```bash
CONFIG_SOURCE=env \
ALYA_REST_USERSVC_DATABASE_HOST=localhost \
ALYA_REST_USERSVC_DATABASE_PORT=5432 \
ALYA_REST_USERSVC_DATABASE_USER=alyatest \
ALYA_REST_USERSVC_DATABASE_PASSWORD=alyatest \
ALYA_REST_USERSVC_DATABASE_DBNAME=alyatest \
ALYA_REST_USERSVC_SERVER_PORT=8083 \
go run .
```

### Other environment variables

- `CONFIG_SOURCE` -> `file` or `env`
- `CONFIG_FILE` -> path to custom config file when `CONFIG_SOURCE=file`

## Start PostgreSQL

```bash
cd examples/rest-usersvc-sqlc-example
docker compose up -d
```

Check status:

```bash
docker compose ps
```

PostgreSQL listens on `localhost:5432`.

## Run migrations

Install `tern` if needed:

```bash
go install github.com/jackc/tern/v2@latest
```

Run migrations:

```bash
cd examples/rest-usersvc-sqlc-example/pg/migrations
tern migrate
```

This creates:

- `users`
- `orders`

## Regenerate SQLC code

Install `sqlc` if needed:

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

Regenerate from the local `pg/` directory:

```bash
cd examples/rest-usersvc-sqlc-example/pg
sqlc generate
```

The SQLC config enables `emit_interface: true`, so the generated package exposes `sqlc.Querier`.

## Run the service

Run with file-based startup config, which is the default:

```bash
cd examples/rest-usersvc-sqlc-example
go run .
```

Run with environment-based startup config:

```bash
cd examples/rest-usersvc-sqlc-example
CONFIG_SOURCE=env \
ALYA_REST_USERSVC_DATABASE_HOST=localhost \
ALYA_REST_USERSVC_DATABASE_PORT=5432 \
ALYA_REST_USERSVC_DATABASE_USER=alyatest \
ALYA_REST_USERSVC_DATABASE_PASSWORD=alyatest \
ALYA_REST_USERSVC_DATABASE_DBNAME=alyatest \
ALYA_REST_USERSVC_SERVER_PORT=8083 \
go run .
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
curl -i -X POST http://localhost:8083/users \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Sachin",
    "email": "sachin@example.com",
    "username": "sachin"
  }'
```

Trigger a custom validation mapping for `username` length:

```bash
curl -i -X POST http://localhost:8083/users \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Sachin",
    "email": "sachin@example.com",
    "username": "abcdefghijklmnopqrstuvwxyz12345"
  }'
```

The response will be a problem document. The `errors` array will include Alya fields:

```json
{
  "type": "https://alya.dev/problems/validation",
  "title": "Validation failed",
  "status": 422,
  "errors": [
    {
      "msgid": 7,
      "errcode": "toobig",
      "field": "username",
      "vals": ["30"],
      "message": "must be at most 30 characters",
      "params": {
        "max": "30"
      }
    }
  ]
}
```

List users:

```bash
curl -i http://localhost:8083/users
```

Create an order for user `1`:

```bash
curl -i -X POST http://localhost:8083/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "number": "ORD1001",
    "status": "pending",
    "total_amount": 2500
  }'
```

List orders:

```bash
curl -i http://localhost:8083/orders
```

Update a user:

```bash
curl -i -X PATCH http://localhost:8083/users/1 \
  -H "Content-Type: application/json" \
  -d '{
    "email": "sachin+new@example.com"
  }'
```

Update an order:

```bash
curl -i -X PATCH http://localhost:8083/orders/1 \
  -H "Content-Type: application/json" \
  -d '{
    "status": "paid",
    "total_amount": 3000
  }'
```

Delete a user:

```bash
curl -i -X DELETE http://localhost:8083/users/1
```

Delete an order:

```bash
curl -i -X DELETE http://localhost:8083/orders/1
```

## Notes

- `main.go` uses Alya's `config.NewFile`, `config.NewEnv`, and `config.LoadWith` for startup config.
- `app/user_service.go` depends on `repository.UserRepository`, not on SQLC.
- `app/order_service.go` depends on `repository.OrderRepository` and uses the user repository to validate `user_id`.
- `repository/sqlc_repository.go` maps SQLC models to repository models.
- `repository/sqlc_repository.go` depends on the generated `sqlc.Querier` interface, not on `*sqlc.Queries` directly.
- The transport layer does not know about SQLC or SQL.
- The example keeps its own schema and generated SQLC code under `pg/`.
