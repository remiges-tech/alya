# Auth Middleware Demo

Production-ready authentication middleware demo showcasing VAPT-compliant JWT validation with Keycloak 26.

## What is VAPT Compliance?

VAPT (Vulnerability Assessment and Penetration Testing) compliance ensures your authentication system follows security best practices:

- **Algorithm Whitelisting**: Only RS256, RS384, RS512 allowed (prevents "none" algorithm attacks)
- **Comprehensive Claims Validation**: Validates exp, iss, nbf, iat, sub, aud
- **Clock Skew Protection**: 5-second grace period for time-based claims
- **Issuer Verification**: Strict issuer URL matching
- **Audience Validation**: Ensures tokens are for your application
- **Token Caching**: Redis caching to reduce OIDC provider calls

## Quick Start

```bash
docker-compose up -d  # Wait 60 seconds for Keycloak to initialize
go run main.go        # Server runs on http://localhost:8083
```

## Testing

Import `postman-collection.json` into Postman and run the requests in order:

1. **Get Token** - Authenticates and saves token automatically
2. **Health Check (Public)** - Tests public endpoint
3. **Missing Token Error** - Validates `AUTH_TOKEN_MISSING` error (msgid: 1001)
4. **Invalid Token Error** - Validates `AUTH_TOKEN_INVALID` error (msgid: 1002)
5. **Get User Info (Protected)** - Returns JWT claims using saved token
6. **Get Protected Data** - Tests protected endpoint with saved token

All requests include automated test scripts that validate responses.

## Key Features Demonstrated

✅ **Error Code Standardization** - Custom error codes and message IDs
✅ **VAPT Compliance** - Production-grade token validation
✅ **Claims Extraction** - Access user_id, email, and custom claims
✅ **Redis Caching** - Token caching for performance
✅ **OIDC Integration** - Works with any OIDC provider (Keycloak, Auth0, Okta)

## Available Claims in Context

After successful authentication, these are available via `c.Get()`:

```go
user_id     string          // Subject (sub) claim
email       string          // Email claim
jwt_claims  map[string]any  // All JWT claims
```

## Credentials

- **Keycloak Admin**: http://localhost:8080 (admin/admin)
- **Test User**: testuser / testpass123

## Routes

- `GET /health` - Public health check
- `GET /api/user` - Protected, returns JWT claims
- `GET /api/data` - Protected, returns sample data

## Troubleshooting

### Error: "expected audience X got []"
Your OIDC provider isn't including the `aud` claim. In Keycloak:
1. Client Settings → Client Scopes → Dedicated scope
2. Add Mapper → Audience → Included Client Audience

### Error: "token missing required claim: sub"
Add a subject mapper in your OIDC provider configuration. See `keycloak-realm.json` for reference.

### Error: "Failed to create OIDC provider"
Ensure Keycloak is running and accessible at the configured URL. Check `docker-compose ps`.

### Error: "AUTH_CACHE_ERROR"
Redis connection failed. Verify Redis is running: `docker-compose ps redis`

## Architecture

```
Request → Auth Middleware → Token Validation → Claims Extraction → Handler
                ↓
          Redis Cache (optional)
                ↓
          OIDC Provider (Keycloak)
```

## Customization

**Use Different OIDC Provider:**
```go
oidcURL := "https://your-provider.com/realms/your-realm"
clientID := "your-client-id"
```

**Add Custom Claim Validators:**
```go
authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    // ... other config
    ClaimValidators: []router.ClaimValidator{
        func(claims map[string]interface{}) error {
            role, ok := claims["role"].(string)
            if !ok || role != "admin" {
                return errors.New("admin role required")
            }
            return nil
        },
    },
})
```

## Stop

```bash
docker-compose down
```
