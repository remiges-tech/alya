package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/remiges-tech/alya/logger"
	"github.com/remiges-tech/alya/wscutils"
)

// TokenCache is an interface for caching tokens.
type TokenCache interface {
	Get(token string) (bool, error)
	Set(token string) error
}

// RedisTokenCache is a Redis implementation of TokenCache.
type RedisTokenCache struct {
	Client     *redis.Client
	Ctx        context.Context
	Expiration time.Duration
}

const DefaultExpiration = 30 * time.Second

func NewRedisTokenCache(addr string, password string, db int, expiration time.Duration) TokenCache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()

	// If no expiration time is provided, set a default value
	if expiration == 0 {
		expiration = DefaultExpiration
	}

	return &RedisTokenCache{
		Client:     rdb,
		Ctx:        ctx,
		Expiration: expiration,
	}
}

// Set sets a token in the cache.
func (r *RedisTokenCache) Set(token string) error {
	err := r.Client.Set(r.Ctx, token, true, r.Expiration).Err()
	return err
}

// Get gets a token from the cache.
func (r *RedisTokenCache) Get(token string) (bool, error) {
	val, err := r.Client.Exists(r.Ctx, token).Result()
	if err != nil {
		return false, err
	}
	return val > 0, nil
}

type AuthMiddleware struct {
	Verifier             *oidc.IDTokenVerifier
	Cache                TokenCache
	Logger               logger.Logger
	SecurityMode         SecurityMode            // NEW: Security mode (Strict/Compatible)
	AllowedAlgorithms    []string                // NEW: Allowed signing algorithms
	RequiredClaims       []string                // NEW: Required claims to validate
	IssuerURL            string                  // NEW: Expected issuer URL
	ValidateClaimsFunc   ClaimsValidatorFunc     // NEW: Custom claims validator
	StoreClaimsInContext bool                    // NEW: Whether to store claims in Gin context
}

// SecurityMode defines the authentication security mode
type SecurityMode int

const (
	// CompatibilityMode maintains backward compatibility (legacy behavior)
	CompatibilityMode SecurityMode = iota
	// StrictMode enforces VAPT-compliant security (recommended for production)
	StrictMode
)

// ClaimsValidatorFunc is a function type for custom claims validation
type ClaimsValidatorFunc func(claims jwt.MapClaims) error

// NewAuthMiddleware creates an auth middleware with default settings (backward compatible)
//
// Deprecated: Use NewAuthMiddlewareWithConfig for VAPT-compliant security
func NewAuthMiddleware(clientID string, provider *oidc.Provider, cache TokenCache, logger logger.Logger) (*AuthMiddleware, error) {
	oidcConfig := &oidc.Config{
		ClientID: clientID,
	}
	verifier := provider.Verifier(oidcConfig)

	return &AuthMiddleware{
		Verifier:             verifier,
		Cache:                cache,
		Logger:               logger,
		SecurityMode:         CompatibilityMode, // Maintain backward compatibility
		StoreClaimsInContext: false,
	}, nil
}

// AuthMiddlewareConfig holds configuration for secure auth middleware
type AuthMiddlewareConfig struct {
	ClientID              string              // OIDC client ID (required)
	Provider              *oidc.Provider      // OIDC provider (required)
	Cache                 TokenCache          // Token cache (required)
	Logger                logger.Logger       // Logger (required)
	IssuerURL             string              // Expected token issuer URL (required for strict mode)
	SecurityMode          SecurityMode        // Security mode (default: StrictMode)
	AllowedAlgorithms     []string            // Allowed signing algorithms (default: RS256, RS384, RS512)
	RequiredClaims        []string            // Required claims to validate (default: exp, iss, sub)
	ValidateClaimsFunc    ClaimsValidatorFunc // Custom claims validator (optional)
	StoreClaimsInContext  bool                // Store claims in Gin context (default: true)
	SkipClientIDCheck     bool                // Skip client ID validation (default: false, not recommended)
	SkipExpiryCheck       bool                // Skip expiry validation (default: false, not recommended)
	SkipIssuerCheck       bool                // Skip issuer validation (default: false, not recommended)
}

// NewAuthMiddlewareWithConfig creates a VAPT-compliant auth middleware
//
// This is the recommended method for production use as it enforces:
// - Explicit algorithm whitelisting (prevents algorithm confusion attacks)
// - Comprehensive claims validation (prevents privilege escalation)
// - Strict security checks (fail-closed behavior)
//
// Example:
//
//	authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
//	    ClientID:  "my-client",
//	    Provider:  oidcProvider,
//	    Cache:     tokenCache,
//	    Logger:    logger,
//	    IssuerURL: "https://keycloak.example.com/realms/myrealm",
//	    // SecurityMode defaults to StrictMode
//	    // AllowedAlgorithms defaults to ["RS256", "RS384", "RS512"]
//	})
func NewAuthMiddlewareWithConfig(config AuthMiddlewareConfig) (*AuthMiddleware, error) {
	// Validate required fields
	if config.ClientID == "" {
		return nil, fmt.Errorf("ClientID is required")
	}
	if config.Provider == nil {
		return nil, fmt.Errorf("Provider is required")
	}
	if config.Cache == nil {
		return nil, fmt.Errorf("Cache is required")
	}

	// Set defaults
	securityMode := config.SecurityMode
	if securityMode == 0 {
		securityMode = StrictMode // Default to strict mode for security
	}

	allowedAlgorithms := config.AllowedAlgorithms
	if len(allowedAlgorithms) == 0 {
		// VAPT FIX: Default to secure RSA algorithms only
		allowedAlgorithms = []string{"RS256", "RS384", "RS512"}
	}

	requiredClaims := config.RequiredClaims
	if len(requiredClaims) == 0 {
		requiredClaims = []string{"exp", "iss", "sub"}
	}

	storeClaimsInContext := config.StoreClaimsInContext
	if !storeClaimsInContext {
		storeClaimsInContext = true // Default to storing claims
	}

	// Strict mode requires issuer URL
	if securityMode == StrictMode && config.IssuerURL == "" {
		return nil, fmt.Errorf("IssuerURL is required in StrictMode")
	}

	// Create OIDC config with security settings
	oidcConfig := &oidc.Config{
		ClientID:             config.ClientID,
		SupportedSigningAlgs: allowedAlgorithms, // ✅ VAPT FIX: Explicit algorithm whitelist
		SkipClientIDCheck:    config.SkipClientIDCheck,
		SkipExpiryCheck:      config.SkipExpiryCheck,
		SkipIssuerCheck:      config.SkipIssuerCheck,
	}

	// In strict mode, enforce all checks
	if securityMode == StrictMode {
		oidcConfig.SkipClientIDCheck = false  // ✅ VAPT FIX: Always verify client ID
		oidcConfig.SkipExpiryCheck = false    // ✅ VAPT FIX: Always verify expiration
		oidcConfig.SkipIssuerCheck = false    // ✅ VAPT FIX: Always verify issuer
	}

	verifier := config.Provider.Verifier(oidcConfig)

	if config.Logger != nil {
		config.Logger.Log(fmt.Sprintf(
			"Auth middleware initialized in %s mode with algorithms: %v",
			map[SecurityMode]string{CompatibilityMode: "Compatibility", StrictMode: "Strict"}[securityMode],
			allowedAlgorithms,
		))
	}

	return &AuthMiddleware{
		Verifier:             verifier,
		Cache:                config.Cache,
		Logger:               config.Logger,
		SecurityMode:         securityMode,
		AllowedAlgorithms:    allowedAlgorithms,
		RequiredClaims:       requiredClaims,
		IssuerURL:            config.IssuerURL,
		ValidateClaimsFunc:   config.ValidateClaimsFunc,
		StoreClaimsInContext: storeClaimsInContext,
	}, nil
}

// AuthErrorScenario defines a set of constants representing different error scenarios
// that can occur within the AuthMiddleware. These scenarios are used to map specific
// error conditions to message IDs and error codes.
type AuthErrorScenario string

const (
	// TokenMissing indicates an error scenario where the expected authentication token is missing from the request.
	TokenMissing AuthErrorScenario = "TokenMissing"
	// TokenCacheFailed indicates an error scenario where an operation related to caching the token fails.
	TokenCacheFailed AuthErrorScenario = "TokenCacheFailed"
	// TokenVerificationFailed indicates an error scenario where the authentication token fails verification.
	TokenVerificationFailed AuthErrorScenario = "TokenVerificationFailed"
)

// scenarioToMsgID maps specific AuthErrorScenarios to message IDs.
var scenarioToMsgID = make(map[AuthErrorScenario]int)

// scenarioToErrCode maps specific AuthErrorScenarios to error codes.
var scenarioToErrCode = make(map[AuthErrorScenario]string)

// RegisterAuthMsgID allows the registration of a message ID for a specific AuthErrorScenario.
func RegisterAuthMsgID(scenario AuthErrorScenario, msgID int) {
	scenarioToMsgID[scenario] = msgID
}

// RegisterAuthErrCode allows the registration of an error code for a specific AuthErrorScenario.
func RegisterAuthErrCode(scenario AuthErrorScenario, errCode string) {
	scenarioToErrCode[scenario] = errCode
}

// defaultMsgID holds the default message ID to be used in error responses when an error scenario
// does not have a specifically registered message ID. This provides a fallback mechanism to ensure
// that error responses always have a message ID.
var defaultMsgID int

// defaultErrCode holds the default error code to be used in error responses when an error scenario
// does not have a specifically registered error code. This default code serves as a generic indicator
// of an error in the absence of a more specific code.
var defaultErrCode string = "ROUTER_ERROR"

// SetDefaultMsgID allows external code to set a custom default message ID.
func SetDefaultMsgID(msgID int) {
	defaultMsgID = msgID
}

// SetDefaultErrCode allows external code to set a custom default error code.
func SetDefaultErrCode(errCode string) {
	defaultErrCode = errCode
}

// MiddlewareFunc returns a gin.HandlerFunc (middleware) that checks for a valid token.
func (a *AuthMiddleware) MiddlewareFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawIDToken, err := ExtractToken(c.Request.Header.Get("Authorization"))
		if err != nil {
			msgID, ok := scenarioToMsgID[TokenMissing]
			if !ok {
				msgID = defaultMsgID // Fallback to a default message ID if not registered
			}
			errCode, ok := scenarioToErrCode[TokenMissing]
			if !ok {
				errCode = defaultErrCode
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, wscutils.NewErrorResponse(msgID, errCode))
			return
		}

		// In strict mode, perform enhanced validation
		if a.SecurityMode == StrictMode {
			if err := a.verifyTokenStrict(c, rawIDToken); err != nil {
				if a.Logger != nil {
					a.Logger.LogDebug(fmt.Sprintf("Strict mode token verification failed: %v", err))
				}
				msgID, ok := scenarioToMsgID[TokenVerificationFailed]
				if !ok {
					msgID = defaultMsgID
				}
				errCode, ok := scenarioToErrCode[TokenVerificationFailed]
				if !ok {
					errCode = "DEFAULT_ERROR_CODE"
				}
				c.AbortWithStatusJSON(http.StatusUnauthorized, wscutils.NewErrorResponse(msgID, errCode))
				return
			}
			c.Next()
			return
		}

		// Compatibility mode: Original behavior
		isCached, err := a.Cache.Get(rawIDToken)
		if err != nil {
			msgID, ok := scenarioToMsgID[TokenCacheFailed]
			if !ok {
				msgID = defaultMsgID
			}
			errCode, ok := scenarioToErrCode[TokenCacheFailed]
			if !ok {
				errCode = "DEFAULT_ERROR_CODE"
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, wscutils.NewErrorResponse(msgID, errCode))
			return
		}

		if !isCached {
			_, err := a.Verifier.Verify(context.Background(), rawIDToken)
			if err != nil {
				msgID, ok := scenarioToMsgID[TokenVerificationFailed]
				if !ok {
					msgID = defaultMsgID
				}
				errCode, ok := scenarioToErrCode[TokenVerificationFailed]
				if !ok {
					errCode = "DEFAULT_ERROR_CODE"
				}
				c.AbortWithStatusJSON(http.StatusUnauthorized, wscutils.NewErrorResponse(msgID, errCode))
				return
			}

			err = a.Cache.Set(rawIDToken)
			if err != nil {
				// Assuming there's a scenario for cache set failure, or reuse TokenCacheFailed
				msgID, ok := scenarioToMsgID[TokenCacheFailed]
				if !ok {
					msgID = defaultMsgID
				}
				errCode, ok := scenarioToErrCode[TokenCacheFailed]
				if !ok {
					errCode = "DEFAULT_ERROR_CODE"
				}
				c.AbortWithStatusJSON(http.StatusInternalServerError, wscutils.NewErrorResponse(msgID, errCode))
				return
			}
		}

		c.Next()
	}
}

// verifyTokenStrict performs VAPT-compliant token verification (Strict Mode)
//
// This method provides comprehensive security validation including:
// - Algorithm whitelisting
// - Claims validation (exp, iss, nbf, iat, required claims)
// - Custom claims validation
// - Context storage of verified claims
func (a *AuthMiddleware) verifyTokenStrict(c *gin.Context, tokenString string) error {
	// Verify token with OIDC provider
	idToken, err := a.Verifier.Verify(c.Request.Context(), tokenString)
	if err != nil {
		return fmt.Errorf("OIDC verification failed: %w", err)
	}

	// Extract all claims
	var claims jwt.MapClaims
	if err := idToken.Claims(&claims); err != nil {
		return fmt.Errorf("failed to extract claims: %w", err)
	}

	// ✅ VAPT FIX: Validate algorithm (explicit check even after OIDC verification)
	if err := a.validateAlgorithm(tokenString); err != nil {
		return err
	}

	// ✅ VAPT FIX: Comprehensive claims validation
	if err := a.validateClaims(claims); err != nil {
		return err
	}

	// ✅ VAPT FIX: Custom claims validation if provided
	if a.ValidateClaimsFunc != nil {
		if err := a.ValidateClaimsFunc(claims); err != nil {
			return fmt.Errorf("custom claims validation failed: %w", err)
		}
	}

	// Store claims in context if enabled
	if a.StoreClaimsInContext {
		c.Set("jwt_claims", claims)
		c.Set("jwt_verified", true)

		// Store common claims for convenience
		if sub, ok := claims["sub"].(string); ok {
			c.Set("user_id", sub)
		}
		if username, ok := claims["preferred_username"].(string); ok {
			c.Set("username", username)
		}
		if email, ok := claims["email"].(string); ok {
			c.Set("email", email)
		}
	}

	return nil
}

// validateAlgorithm validates the token's signing algorithm
func (a *AuthMiddleware) validateAlgorithm(tokenString string) error {
	// Parse token to extract algorithm from header (without verification)
	parser := jwt.Parser{}
	token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return fmt.Errorf("failed to parse token header: %w", err)
	}

	alg := token.Method.Alg()

	// Check if algorithm is in allowed list
	for _, allowedAlg := range a.AllowedAlgorithms {
		if alg == allowedAlg {
			return nil
		}
	}

	return fmt.Errorf("algorithm %s not allowed (expected one of: %v)", alg, a.AllowedAlgorithms)
}

// validateClaims performs comprehensive JWT claims validation
func (a *AuthMiddleware) validateClaims(claims jwt.MapClaims) error {
	now := time.Now()

	// Validate expiration (exp)
	if exp, ok := claims["exp"].(float64); ok {
		expTime := time.Unix(int64(exp), 0)
		if now.After(expTime) {
			return fmt.Errorf("token expired at %s", expTime.Format(time.RFC3339))
		}
	} else {
		return fmt.Errorf("token missing required claim: exp")
	}

	// Validate not before (nbf) if present
	if nbf, ok := claims["nbf"].(float64); ok {
		nbfTime := time.Unix(int64(nbf), 0)
		if now.Before(nbfTime) {
			return fmt.Errorf("token not valid before %s", nbfTime.Format(time.RFC3339))
		}
	}

	// Validate issued at (iat) if present
	if iat, ok := claims["iat"].(float64); ok {
		iatTime := time.Unix(int64(iat), 0)
		// Reject tokens issued in the future (with 5 second grace period for clock skew)
		if now.Add(-5 * time.Second).Before(iatTime) {
			return fmt.Errorf("token issued in the future at %s", iatTime.Format(time.RFC3339))
		}
	}

	// Validate issuer (iss) if configured
	if a.IssuerURL != "" {
		if iss, ok := claims["iss"].(string); ok {
			if iss != a.IssuerURL {
				return fmt.Errorf("invalid issuer: expected %s, got %s", a.IssuerURL, iss)
			}
		} else {
			return fmt.Errorf("token missing required claim: iss")
		}
	}

	// Validate all required claims are present
	for _, requiredClaim := range a.RequiredClaims {
		if _, exists := claims[requiredClaim]; !exists {
			return fmt.Errorf("token missing required claim: %s", requiredClaim)
		}
	}

	return nil
}

// ExtractToken extracts the token from the Authorization header.
func ExtractToken(headerValue string) (string, error) {
	const prefix = "Bearer "

	if !strings.HasPrefix(headerValue, prefix) {
		return "", fmt.Errorf("missing or incorrect Authorization header format")
	}

	token := strings.TrimPrefix(headerValue, prefix)
	if token == "" {
		return "", fmt.Errorf("missing token in Authorization header")
	}

	return token, nil
}
