# restutils

This package provides Gin helpers for REST-style Alya endpoints.

Use it when you want:

- direct JSON request bodies
- plain JSON success responses
- problem-style error responses
- request validation with JSON field names

## What the package covers

The package has four parts:

- `BindBody` for request decoding
- `Validator` for request validation
- `Problem` helpers for error responses
- response helpers such as `WriteOK` and `WriteCreated`

## Request binding

Use `BindBody` to decode one JSON request body into a struct.

```go
var req CreateUserRequest
if err := restutils.BindBody(c, &req); err != nil {
    restutils.WriteProblem(c, restutils.ProblemFromBindError(err))
    return
}
```

`BindBody` enforces these rules:

- request body must be present
- `Content-Type` must be `application/json`
- unknown JSON fields are rejected
- the body must contain a single JSON object

### Example request type

```go
type CreateUserRequest struct {
    Name     string `json:"name" validate:"required,min=2,max=50"`
    Email    string `json:"email" validate:"required,email,max=100"`
    Username string `json:"username" validate:"required,min=3,max=30,alphanum"`
}
```

### Binding failure mapping

Convert binding errors to problem responses with `ProblemFromBindError(...)`.

Current mapping:

- invalid content type -> `415 Unsupported Media Type`
- empty body -> `400 Bad Request`
- malformed JSON -> `400 Bad Request`
- invalid value type -> `400 Bad Request`
- unknown field -> `400 Bad Request`

## Validation

Create a validator once and reuse it.

```go
validator := restutils.NewValidator()
```

Then validate a request value:

```go
if errs := validator.Validate(req); len(errs) > 0 {
    restutils.WriteProblem(c, restutils.ValidationProblem(errs))
    return
}
```

The validator uses JSON tag names in returned field errors.

### Built-in mappings

The built-in mappings cover these validator tags:

- `required`
- `email`
- `uuid`
- `e164`
- `alphanum`
- `oneof`
- `min`
- `max`
- `gte`
- `lte`

Each validation error includes Alya-style fields from `wscutils.ErrorMessage` plus REST-specific detail fields:

```json
{
  "msgid": 103,
  "errcode": "toobig",
  "field": "username",
  "vals": ["30"],
  "message": "must be at most 30 characters",
  "params": {
    "max": "30"
  }
}
```

### Customize validation mapping

Use `NewValidatorWithConfig(...)` to override mappings by validator tag or by field.

```go
validator := restutils.NewValidatorWithConfig(restutils.ValidatorConfig{
    FieldRules: map[string]map[string]restutils.ValidationRule{
        "username": {
            "max": {
                MsgID:   7,
                ErrCode: "toobig",
            },
        },
    },
})
```

Use this when one field needs Alya-specific `msgid` or `errcode` values that differ from the default mapping.

## Problem responses

Use `Problem` helpers for non-success responses.

### Validation problem

```go
if errs := validator.Validate(req); len(errs) > 0 {
    restutils.WriteProblem(c, restutils.ValidationProblem(errs))
    return
}
```

This writes a `422 Unprocessable Entity` response.

### Custom problem

```go
problem := restutils.NewProblem(
    http.StatusConflict,
    "https://alya.dev/problems/conflict",
    "Conflict",
    "username already exists",
)
restutils.WriteProblem(c, problem)
```

### Internal server error

```go
restutils.WriteProblem(c, restutils.InternalServerError())
```

### What `WriteProblem` adds

`WriteProblem(...)` also fills these fields when possible:

- `instance` from the current request path
- `trace_id` from `c.Get("trace_id")`

It sets:

- `Content-Type: application/problem+json`

## Success responses

Use the response helpers for common status codes.

```go
restutils.WriteOK(c, data)
restutils.WriteCreated(c, "/users/1", data)
restutils.WriteAccepted(c, data)
restutils.WriteNoContent(c)
```

## Minimal handler example

```go
func (h *UserHandler) CreateUser(c *gin.Context, _ *service.Service) {
    var req CreateUserRequest
    if err := restutils.BindBody(c, &req); err != nil {
        restutils.WriteProblem(c, restutils.ProblemFromBindError(err))
        return
    }

    if errs := h.validator.Validate(req); len(errs) > 0 {
        restutils.WriteProblem(c, restutils.ValidationProblem(errs))
        return
    }

    user, err := h.app.CreateUser(c.Request.Context(), req)
    if err != nil {
        restutils.WriteProblem(c, restutils.NewProblem(
            http.StatusConflict,
            "https://alya.dev/problems/conflict",
            "Conflict",
            "username already exists",
        ))
        return
    }

    restutils.WriteCreated(c, "/users/1", user)
}
```

## Related example

See:

- `examples/rest-usersvc-sqlc-example/transport/user_handler.go`
- `examples/rest-usersvc-sqlc-example/transport/order_handler.go`

Those handlers show request binding, validation, problem mapping, and success responses in a running service.
