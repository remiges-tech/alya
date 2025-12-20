# Auth Middleware Demo Application

A complete example showing how to use Alya's VAPT-compliant authentication middleware with proper error code configuration.

## Features

✅ **VAPT-Compliant Security** - Uses StrictMode with comprehensive validation
✅ **Standardized Error Codes** - Proper error messages for all auth scenarios
✅ **Claims Storage** - JWT claims automatically stored in Gin context
✅ **Custom Validation** - Extensible business logic validation
✅ **Redis Caching** - Token verification caching for performance
✅ **Production-Ready** - Following security best practices

## Quick Start

### 1. Prerequisites

- Go 1.21+
- Redis server running
- OIDC provider (Keycloak, Auth0, etc.)

### 2. Installation

```bash
cd examples/auth-middleware-demo
go mod init auth-demo
go get github.com/remiges-tech/alya@latest
go get github.com/gin-gonic/gin
go get github.com/coreos/go-oidc/v3/oidc
go get github.com/golang-jwt/jwt/v5
```

### 3. Configuration

Create a `.env` file:

```bash
cp .env.example .env
# Edit .env with your actual values
```

### 4. Start Redis

```bash
redis-server
```

### 5. Run the Application

```bash
go run main.go
```

Server will start on `http://localhost:8080`

## API Endpoints

### Public Endpoints (No Auth Required)

#### Health Check
```bash
curl http://localhost:8080/health
```

**Response:**
```json
{
    "status": "ok",
    "service": "auth-middleware-demo"
}
```

#### Homepage
```bash
curl http://localhost:8080/
```

### Protected Endpoints (Auth Required)

#### Get User Info
```bash
curl http://localhost:8080/api/user \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

**Success Response (200):**
```json
{
    "user_id": "user-123",
    "email": "user@example.com",
    "username": "john.doe",
    "claims": {
        "exp": 1735123456,
        "iss": "https://keycloak.example.com/realms/myrealm",
        "sub": "user-123",
        "email": "user@example.com"
    }
}
```

#### List Users
```bash
curl http://localhost:8080/api/users \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

#### Create User
```bash
curl -X POST http://localhost:8080/api/users \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Alice",
    "email": "alice@example.com"
  }'
```

## Error Handling Examples

### Error 1: Missing Token

**Request:**
```bash
curl http://localhost:8080/api/user
```

**Response (401):**
```json
{
    "status": "error",
    "data": null,
    "messages": [
        {
            "msgid": 1001,
            "errcode": "AUTH_TOKEN_MISSING"
        }
    ]
}
```

### Error 2: Invalid Token

**Request:**
```bash
curl http://localhost:8080/api/user \
  -H "Authorization: Bearer invalid-token"
```

**Response (401):**
```json
{
    "status": "error",
    "data": null,
    "messages": [
        {
            "msgid": 1002,
            "errcode": "AUTH_TOKEN_INVALID"
        }
    ]
}
```

### Error 3: Cache Error

**Response (500):**
```json
{
    "status": "error",
    "data": null,
    "messages": [
        {
            "msgid": 1003,
            "errcode": "AUTH_CACHE_ERROR"
        }
    ]
}
```

## Configuration Details

### Error Code Setup

The application configures three auth error scenarios:

```go
func setupAuthErrorCodes() {
    // Default fallback
    router.SetDefaultMsgID(9999)
    router.SetDefaultErrCode("AUTH_ERROR")

    // Scenario 1: Missing token
    router.RegisterAuthMsgID(router.TokenMissing, 1001)
    router.RegisterAuthErrCode(router.TokenMissing, "AUTH_TOKEN_MISSING")

    // Scenario 2: Invalid token
    router.RegisterAuthMsgID(router.TokenVerificationFailed, 1002)
    router.RegisterAuthErrCode(router.TokenVerificationFailed, "AUTH_TOKEN_INVALID")

    // Scenario 3: Cache failure
    router.RegisterAuthMsgID(router.TokenCacheFailed, 1003)
    router.RegisterAuthErrCode(router.TokenCacheFailed, "AUTH_CACHE_ERROR")
}
```

### Middleware Configuration

Using VAPT-compliant StrictMode:

```go
authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    ClientID:     clientID,
    Provider:     router.WrapOIDCProvider(provider),
    Cache:        cache,
    Logger:       l,
    IssuerURL:    oidcProviderURL,
    SecurityMode: router.StrictMode,  // Enforces all security checks
})
```

## Security Features

### StrictMode Enforces:

- ✅ **Algorithm Whitelisting** - Only RS256, RS384, RS512 allowed
- ✅ **Issuer Validation** - Prevents token substitution attacks
- ✅ **Claims Validation** - Validates exp, iss, sub, nbf, iat
- ✅ **Client ID Check** - Always enforced
- ✅ **Expiry Check** - Always enforced
- ✅ **Clock Skew Tolerance** - 5 second grace period
- ✅ **Claims Storage** - Stored in Gin context for audit trails

### Accessing Claims in Handlers

```go
func myHandler(c *gin.Context) {
    // Method 1: Get specific user info (extracted automatically)
    userID, _ := c.Get("user_id")
    email, _ := c.Get("email")
    username, _ := c.Get("username")

    // Method 2: Get all claims
    claims, exists := c.Get("jwt_claims")
    if exists {
        jwtClaims := claims.(jwt.MapClaims)
        // Access any claim: jwtClaims["role"]
    }
}
```

## Custom Validation

Add business-specific validation in `validateBusinessRules`:

```go
func validateBusinessRules(claims jwt.MapClaims) error {
    // Example: Require admin role
    role, ok := claims["role"].(string)
    if !ok || role != "admin" {
        return fmt.Errorf("admin role required")
    }

    // Example: Check organization
    org, ok := claims["org"].(string)
    if !ok || org != "acme-corp" {
        return fmt.Errorf("invalid organization")
    }

    return nil
}
```

## Testing

### Generate a Test Token

If using Keycloak:

```bash
# Get access token
TOKEN=$(curl -X POST 'https://keycloak.example.com/realms/myrealm/protocol/openid-connect/token' \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  -d 'client_id=your-client-id' \
  -d 'username=testuser' \
  -d 'password=testpass' \
  -d 'grant_type=password' \
  | jq -r '.access_token')

# Use it
curl http://localhost:8080/api/user \
  -H "Authorization: Bearer $TOKEN"
```

## Troubleshooting

### "AUTH_CACHE_ERROR" responses

**Cause:** Redis is not running or unreachable

**Solution:**
```bash
# Start Redis
redis-server

# Or update REDIS_ADDR in .env
```

### "AUTH_TOKEN_INVALID" for valid tokens

**Causes:**
1. IssuerURL mismatch
2. Wrong ClientID
3. Token algorithm not in AllowedAlgorithms
4. Token expired

**Solutions:**
```bash
# Verify issuer matches
echo $OIDC_PROVIDER_URL

# Check token claims
echo $TOKEN | cut -d'.' -f2 | base64 -d | jq

# Verify not expired
date -u
```

### Generic "ROUTER_ERROR" responses

**Cause:** Error codes not configured before middleware creation

**Solution:** Ensure `setupAuthErrorCodes()` is called before creating middleware

## Production Deployment

### Environment Variables

```bash
export OIDC_PROVIDER_URL=https://your-keycloak.com/realms/prod
export CLIENT_ID=prod-client-id
export REDIS_ADDR=redis-cluster:6379
export PORT=8080
```

### Docker Deployment

Create `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o auth-demo main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/auth-demo .
EXPOSE 8080
CMD ["./auth-demo"]
```

Build and run:

```bash
docker build -t auth-demo .
docker run -p 8080:8080 \
  -e OIDC_PROVIDER_URL=https://your-keycloak.com/realms/prod \
  -e CLIENT_ID=prod-client-id \
  -e REDIS_ADDR=redis:6379 \
  auth-demo
```

## Advanced Configuration

### Custom Algorithm List

```go
authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    // ... other config
    AllowedAlgorithms: []string{"RS384", "RS512"}, // Only these algorithms
})
```

### Additional Required Claims

```go
authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    // ... other config
    RequiredClaims: []string{"exp", "iss", "sub", "email", "role"},
})
```

### Disable Claims Storage (Not Recommended)

```go
authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    // ... other config
    StoreClaimsInContext: false,
})
```

## References

- [Alya Router Documentation](../../router/README.md)
- [Auth Middleware Source](../../router/auth_middleware.go)
- [Test Examples](../../router/auth_middleware_test.go)
- [OIDC Specification](https://openid.net/connect/)
- [JWT Best Practices](https://datatracker.ietf.org/doc/html/rfc8725)

## License

Same as Alya project license.
