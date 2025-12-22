# Auth Middleware Security

The `AuthMiddleware` provides JWT token validation with algorithm whitelisting and claims validation.

## Usage

```go
authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    ClientID:  "my-client",
    Provider:  router.WrapOIDCProvider(provider),
    Cache:     tokenCache,
    Logger:    logger,
    IssuerURL: "https://keycloak.example.com/realms/myrealm",
})

r := gin.Default()
api := r.Group("/api")
api.Use(authMW.MiddlewareFunc())
```

## Configuration

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| ClientID | Yes | - | OIDC client ID |
| Provider | Yes | - | OIDC provider (use `WrapOIDCProvider()`) |
| Cache | Yes | - | Token cache implementation |
| Logger | No | nil | Logger instance |
| IssuerURL | Yes | - | Expected token issuer |
| AllowedAlgorithms | No | RS256, RS384, RS512 | Allowed signing algorithms |
| RequiredClaims | No | exp, iss, sub | Claims that must be present |
| ValidateClaimsFunc | No | nil | Custom validation function |
| StoreClaimsInContext | No | true | Store claims in Gin context |

## Migration from Old API

Replace:
```go
authMW, _ := router.NewAuthMiddleware(clientID, provider, cache, logger)
```

With:
```go
authMW, _ := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    ClientID:  clientID,
    Provider:  router.WrapOIDCProvider(provider),
    Cache:     cache,
    Logger:    logger,
    IssuerURL: "https://your-issuer-url",
})
```

## Accessing Claims in Handlers

```go
func MyHandler(c *gin.Context) {
    claims, _ := c.Get("jwt_claims")  // All claims as jwt.MapClaims
    userID, _ := c.Get("user_id")     // Subject claim
    email, _ := c.Get("email")        // Email claim
}
```

## Custom Claims Validation

```go
authMW, _ := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    // ... required fields ...
    ValidateClaimsFunc: func(claims jwt.MapClaims) error {
        roles, ok := claims["roles"].([]interface{})
        if !ok || len(roles) == 0 {
            return fmt.Errorf("user has no roles")
        }
        return nil
    },
})
```

## Security Validations

The middleware validates:
- **Algorithm**: Only RS256, RS384, RS512 allowed by default
- **Expiration (exp)**: Token must not be expired
- **Not Before (nbf)**: Token must be valid for current time
- **Issued At (iat)**: Rejects tokens issued in the future (5s grace period)
- **Issuer (iss)**: Must match configured IssuerURL
- **Required claims**: All configured claims must be present
