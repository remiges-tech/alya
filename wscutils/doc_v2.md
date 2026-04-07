# wscutils additive API

This file documents the additive API introduced for Alya envelope style services.

Theme: keep the existing Alya envelope contract, but give new code better request binding and instance-based validation.

## Existing API

These functions and behaviors are unchanged:

- `BindJSON`
- `WscValidate`
- `SetValidationTagToMsgIDMap`
- `SetValidationTagToErrCodeMap`
- package-level validation defaults and maps
- `NewSuccessResponse`
- `NewErrorResponse`
- `NewResponse`
- `SendSuccessResponse`
- `SendErrorResponse`

Existing users can keep using the current API without changes.

## New additive API

The additive API is small.

It adds:

- `BindData`
- `ParseInt64PathParam`
- `Validator`
- `ValidationRule`

Use it for new envelope-style services that want:

- `{"data": ...}` request binding
- JSON-tag-based field names in validation errors
- validator instances instead of package-level validation state

Response construction stays on the existing API.

## Request shape

The additive API keeps the same Alya request envelope.

```json
{
  "data": {
    "name": "Sachin",
    "email": "sachin@example.com",
    "username": "sachin"
  }
}
```

## Bind request data

Use `BindData` to decode the top-level `data` payload into a typed struct.

```go
type CreateUserRequest struct {
    Name     string `json:"name" validate:"required,min=2,max=50"`
    Email    string `json:"email" validate:"required,email,max=100"`
    Username string `json:"username" validate:"required,min=3,max=30,alphanum"`
}

var req CreateUserRequest
if err := wscutils.BindData(c, &req); err != nil {
    messages := []wscutils.ErrorMessage{
        wscutils.BuildErrorMessage(1001, "invalid_json", ""),
    }
    c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, messages))
    return
}
```

`BindData` enforces these rules:

- request body must be present
- `Content-Type` must be `application/json`
- unknown JSON fields are rejected
- the body must contain a single JSON object
- the payload is read from the top-level `data` field

## Parse path parameters

Use `ParseInt64PathParam` for resource-oriented routes.

```go
id, err := wscutils.ParseInt64PathParam(c, "id")
if err != nil {
    messages := []wscutils.ErrorMessage{
        wscutils.BuildErrorMessage(104, "invalid", "id"),
    }
    c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, messages))
    return
}
```

## Instance-based validation

The existing `WscValidate` function uses package-level validation maps.

The new `Validator` type keeps validation setup local to one service or module.

Create a validator like this:

```go
validator := wscutils.NewValidator(
    map[string]wscutils.ValidationRule{
        "required": {MsgID: 45, ErrCode: "missing"},
        "email":    {MsgID: 101, ErrCode: "datafmt"},
        "min": {
            MsgID:   102,
            ErrCode: "toosmall",
            GetVals: func(err validator.FieldError) []string {
                return []string{err.Param()}
            },
        },
        "max": {
            MsgID:   103,
            ErrCode: "toobig",
            GetVals: func(err validator.FieldError) []string {
                return []string{err.Param()}
            },
        },
    },
    wscutils.ValidationRule{MsgID: 104, ErrCode: "invalid"},
)
```

Validate a request value like this:

```go
if errs := validator.Validate(req); len(errs) > 0 {
    c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, errs))
    return
}
```

The additive validator:

- uses JSON tag names for fields
- maps validator tags to Alya `msgid` and `errcode`
- optionally adds `vals` through `GetVals`
- returns `[]ErrorMessage`
- avoids package-level validation state in new code

## Response construction

Response construction stays on the existing API.

Success response:

```go
wscutils.SendSuccessResponse(c, wscutils.NewSuccessResponse(user))
```

Error response with a non-400 status:

```go
messages := []wscutils.ErrorMessage{
    wscutils.BuildErrorMessage(105, "exists", "username"),
}
c.JSON(http.StatusConflict, wscutils.NewResponse(wscutils.ErrorStatus, nil, messages))
```

## Minimal handler example

```go
func (h *UserHandler) CreateUser(c *gin.Context) {
    var req api.CreateUserRequest
    if err := wscutils.BindData(c, &req); err != nil {
        messages := []wscutils.ErrorMessage{
            wscutils.BuildErrorMessage(1001, "invalid_json", ""),
        }
        c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, messages))
        return
    }

    if errs := h.validator.Validate(req); len(errs) > 0 {
        c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, errs))
        return
    }

    user, err := h.app.CreateUser(c.Request.Context(), req)
    if err != nil {
        messages := []wscutils.ErrorMessage{
            wscutils.BuildErrorMessage(105, "exists", "username"),
        }
        c.JSON(http.StatusConflict, wscutils.NewResponse(wscutils.ErrorStatus, nil, messages))
        return
    }

    wscutils.SendSuccessResponse(c, wscutils.NewSuccessResponse(user))
}
```

## Migration notes

If an existing service already uses:

- `BindJSON`
- `WscValidate`
- `SendSuccessResponse`
- `SendErrorResponse`
- package-level validation maps

it does not need to change.

A common migration path is:

1. keep the Alya envelope response format
2. switch one handler from `BindJSON` to `BindData`
3. replace package-level validation maps with a local `Validator`

## Related example

See:

- `examples/wsc-usersvc-sqlc-example/internal/http/user_handler.go`
- `examples/wsc-usersvc-sqlc-example/internal/http/order_handler.go`
- `examples/wsc-usersvc-sqlc-example/internal/validation/validator.go`
