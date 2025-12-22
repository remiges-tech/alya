# Security Enhancements for Auth Middleware

This document describes the VAPT-compliant security enhancements made to the Alya authentication middleware.

## Overview

The enhanced `AuthMiddleware` addresses critical security vulnerabilities identified in penetration testing:

1. **Algorithm Confusion Attacks** (CVE-type vulnerability)
2. **Privilege Escalation via Token Manipulation**
3. **Insufficient Claims Validation**

## VAPT Issues Addressed

### Issue #1: JWT Algorithm Substitution Attack

**Problem:** The middleware accepted JWT tokens signed with any algorithm, including weak or no algorithms (HS256, HS512, "none").

**Fix:**
- ✅ Explicit algorithm whitelisting via `SupportedSigningAlgs`
- ✅ Default whitelist: `["RS256", "RS384", "RS512"]` (RSA only)
- ✅ Algorithm validation enforced before verification

**Code:**
```go
oidcConfig := &oidc.Config{
    ClientID:             config.ClientID,
    SupportedSigningAlgs: []string{"RS256", "RS384", "RS512"}, // ✅ SECURITY FIX
    SkipClientIDCheck:    false,
    SkipExpiryCheck:      false,
    SkipIssuerCheck:      false,
}
```

###Issue #2: Privilege Escalation via JWT Manipulation

**Problem:** Claims were not validated server-side, allowing attackers to modify token claims (e.g., `caps`, `roles`) to gain unauthorized access.

**Fix:**
- ✅ Comprehensive server-side claims validation
- ✅ Expiration (exp), Not Before (nbf), Issued At (iat) validation
- ✅ Issuer (iss) validation
- ✅ Required claims enforcement
- ✅ Custom claims validator support

**Code:**
```go
func (a *AuthMiddleware) validateClaims(claims jwt.MapClaims) error {
    // Validate exp, nbf, iat, iss
    // Validate required claims
    // Run custom validator
    return nil
}
```

### Issue #3: Token Caching Security Risk

**Problem:** Caching entire tokens in Redis could lead to cache poisoning attacks.

**Note:** While this implementation still supports token caching for backward compatibility, we recommend migrating to JWKS-based key caching as demonstrated in the example backend implementation.

## Security Modes

The middleware supports two modes:

### 1. Compatibility Mode (Default for `NewAuthMiddleware`)

Maintains backward compatibility with existing code:
- Uses original token caching behavior
- Basic OIDC verification only
- **⚠️ Not recommended for production**

```go
// Legacy usage (compatibility mode)
authMW, err := router.NewAuthMiddleware(clientID, provider, cache, logger)
```

### 2. Strict Mode (Default for `NewAuthMiddlewareWithConfig`)

Enforces VAPT-compliant security:
- Explicit algorithm whitelisting
- Comprehensive claims validation
- Fail-closed behavior
- **✅ Recommended for production**

```go
// Secure usage (strict mode)
authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    ClientID:  "my-client",
    Provider:  oidcProvider,
    Cache:     tokenCache,
    Logger:    logger,
    IssuerURL: "https://keycloak.example.com/realms/myrealm",
    // SecurityMode defaults to StrictMode
})
```

## Migration Guide

### Step 1: Update to Secure Configuration

**Before (Insecure):**
```go
authMW, err := router.NewAuthMiddleware(
    "my-client",
    oidcProvider,
    redisCache,
    logger,
)
```

**After (Secure):**
```go
authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    ClientID:  "my-client",
    Provider:  oidcProvider,
    Cache:     redisCache,
    Logger:    logger,
    IssuerURL: "https://keycloak.example.com/realms/myrealm", // Required
    // Optional: customize security
    AllowedAlgorithms: []string{"RS256"}, // Even stricter
    RequiredClaims:    []string{"exp", "iss", "sub", "email"},
})
```

### Step 2: Add Custom Claims Validation (Optional)

```go
authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    // ... other config ...
    ValidateClaimsFunc: func(claims jwt.MapClaims) error {
        // Example: Validate user roles
        roles, ok := claims["roles"].([]interface{})
        if !ok || len(roles) == 0 {
            return fmt.Errorf("user has no roles")
        }
        return nil
    },
})
```

### Step 3: Access Claims in Handlers

```go
func MyHandler(c *gin.Context) {
    // Get verified claims from context
    claims, exists := c.Get("jwt_claims")
    if !exists {
        c.JSON(401, gin.H{"error": "unauthorized"})
        return
    }

    jwtClaims := claims.(jwt.MapClaims)

    // Access specific claims
    userID, _ := c.Get("user_id")
    username, _ := c.Get("username")
    email, _ := c.Get("email")

    // Use claims...
}
```

## Configuration Options

### AuthMiddlewareConfig Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `ClientID` | string | Yes | - | OIDC client ID |
| `Provider` | *oidc.Provider | Yes | - | OIDC provider instance |
| `Cache` | TokenCache | Yes | - | Token cache (Redis/custom) |
| `Logger` | logger.Logger | Yes | - | Logger instance |
| `IssuerURL` | string | Yes (strict) | - | Expected token issuer |
| `SecurityMode` | SecurityMode | No | StrictMode | Security mode |
| `AllowedAlgorithms` | []string | No | RS256/384/512 | Allowed signing algorithms |
| `RequiredClaims` | []string | No | exp, iss, sub | Required JWT claims |
| `ValidateClaimsFunc` | func | No | nil | Custom claims validator |
| `StoreClaimsInContext` | bool | No | true | Store claims in Gin context |

### Security Mode Values

| Mode | Description | Use Case |
|------|-------------|----------|
| `CompatibilityMode` | Legacy behavior | Migration only |
| `StrictMode` | VAPT-compliant | **Production (recommended)** |

## Security Best Practices

### ✅ DO:

1. **Use Strict Mode in production**
   ```go
   SecurityMode: router.StrictMode
   ```

2. **Specify issuer URL**
   ```go
   IssuerURL: "https://your-keycloak.com/realms/your-realm"
   ```

3. **Use RSA algorithms only**
   ```go
   AllowedAlgorithms: []string{"RS256", "RS384", "RS512"}
   ```

4. **Validate required claims**
   ```go
   RequiredClaims: []string{"exp", "iss", "sub", "email"}
   ```

5. **Implement custom validators for business logic**
   ```go
   ValidateClaimsFunc: yourCustomValidator
   ```

6. **Store claims in context for access control**
   ```go
   StoreClaimsInContext: true // Default
   ```

### ❌ DON'T:

1. **Don't use Compatibility Mode in production**
   ```go
   SecurityMode: router.CompatibilityMode // ⚠️ INSECURE
   ```

2. **Don't allow weak algorithms**
   ```go
   AllowedAlgorithms: []string{"HS256", "none"} // ❌ VULNERABLE
   ```

3. **Don't skip security checks**
   ```go
   SkipClientIDCheck: true  // ❌ INSECURE
   SkipExpiryCheck: true    // ❌ INSECURE
   SkipIssuerCheck: true    // ❌ INSECURE
   ```

4. **Don't trust client-side token modifications**
   - Always validate on server
   - Never trust claims without verification

## Testing

### Security Test Cases

The middleware includes comprehensive security tests:

1. **Algorithm Validation Tests**
   - RS256/384/512 accepted ✅
   - HS256/512 rejected ✅
   - "none" algorithm rejected ✅

2. **Token Manipulation Tests**
   - Modified claims rejected ✅
   - Invalid signatures rejected ✅

3. **Claims Validation Tests**
   - Expired tokens rejected ✅
   - Future-dated tokens rejected ✅
   - Invalid issuer rejected ✅
   - Missing required claims rejected ✅

### Running Tests

```bash
cd router
go test -v -run TestAuthMiddleware
```

## Compliance

This implementation addresses:

- ✅ **OWASP Top 10 2021**
  - A01:2021 – Broken Access Control
  - A02:2021 – Cryptographic Failures
  - A07:2021 – Identification and Authentication Failures

- ✅ **CWE-​347**: Improper Verification of Cryptographic Signature
- ✅ **CWE-​345**: Insufficient Verification of Data Authenticity
- ✅ **CWE-​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​290**: Authentication Bypass by Spoofing

## Support & Contributing

For questions or issues:
1. Check existing issues on GitHub
2. Review this security documentation
3. Consult the Alya framework documentation

### Reporting Security Vulnerabilities

Please report security vulnerabilities responsibly:
- **DO NOT** open public GitHub issues for security bugs
- Email security concerns to the maintainers
- Allow time for patch before public disclosure

## Changelog

### v0.33.0 (Proposed)
- ✅ Added `NewAuthMiddlewareWithConfig` for VAPT-compliant auth
- ✅ Added explicit algorithm whitelisting
- ✅ Added comprehensive claims validation
- ✅ Added `SecurityMode` (Strict/Compatibility)
- ✅ Added custom claims validator support
- ✅ Deprecated `NewAuthMiddleware` (use `NewAuthMiddlewareWithConfig`)
- ✅ Maintained backward compatibility

## License

Same as Alya framework (check main repository LICENSE file)

---

**Security Note:** This enhancement is designed to address real-world penetration testing findings. Always keep dependencies updated and monitor security advisories.
