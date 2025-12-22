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

// OIDCProvider is an interface that wraps oidc.Provider for testability
type OIDCProvider interface {
	Verifier(config *oidc.Config) *oidc.IDTokenVerifier
}

// oidcProviderWrapper wraps *oidc.Provider to implement OIDCProvider interface
type oidcProviderWrapper struct {
	provider *oidc.Provider
}

func (w *oidcProviderWrapper) Verifier(config *oidc.Config) *oidc.IDTokenVerifier {
	return w.provider.Verifier(config)
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
	Provider             OIDCProvider        // OIDC provider interface
	AllowedAlgorithms    []string            // Allowed signing algorithms
	RequiredClaims       []string            // Required claims to validate
	IssuerURL            string              // Expected issuer URL
	ValidateClaimsFunc   ClaimsValidatorFunc // Custom claims validator
	StoreClaimsInContext bool                // Whether to store claims in Gin context
}

// ClaimsValidatorFunc is a function type for custom claims validation
type ClaimsValidatorFunc func(claims jwt.MapClaims) error

// NewAuthMiddleware creates an auth middleware with default settings
func NewAuthMiddleware(clientID string, provider *oidc.Provider, cache TokenCache, logger logger.Logger) (*AuthMiddleware, error) {
	oidcConfig := &oidc.Config{
		ClientID: clientID,
	}

	// Wrap the provider to make it compatible with OIDCProvider interface
	wrappedProvider := &oidcProviderWrapper{provider: provider}
	verifier := wrappedProvider.Verifier(oidcConfig)

	return &AuthMiddleware{
		Verifier:             verifier,
		Cache:                cache,
		Logger:               logger,
		Provider:             wrappedProvider,
		AllowedAlgorithms:    []string{"RS256", "RS384", "RS512"},
		RequiredClaims:       []string{"exp", "iss", "sub"},
		StoreClaimsInContext: true,
	}, nil
}

// AuthMiddlewareConfig holds configuration for Auth middleware
type AuthMiddlewareConfig struct {
	ClientID             string              // OIDC client ID (required)
	Provider             OIDCProvider        // OIDC provider interface (required)
	Cache                TokenCache          // Token cache (required for performance)
	Logger               logger.Logger       // Logger (optional)
	IssuerURL            string              // Expected token issuer URL (required)
	AllowedAlgorithms    []string            // Allowed signing algorithms (default: RS256, RS384, RS512)
	RequiredClaims       []string            // Required claims to validate (default: exp, iss, sub)
	ValidateClaimsFunc   ClaimsValidatorFunc // Custom claims validator (optional)
	StoreClaimsInContext bool                // Store claims in Gin context (default: true)
	SkipClientIDCheck    bool                // Skip client ID validation (default: false, not recommended)
	SkipExpiryCheck      bool                // Skip expiry validation (default: false, not recommended)
	SkipIssuerCheck      bool                // Skip issuer validation (default: false, not recommended)
}

// WrapOIDCProvider wraps a *oidc.Provider to implement the OIDCProvider interface
// Use this helper when constructing AuthMiddlewareConfig with a real OIDC provider
func WrapOIDCProvider(provider *oidc.Provider) OIDCProvider {
	return &oidcProviderWrapper{provider: provider}
}

// NewAuthMiddlewareWithConfig creates a Auth middleware
//
// This method enforces production-grade security:
// - Explicit algorithm whitelisting (prevents algorithm confusion attacks)
// - Comprehensive claims validation (prevents privilege escalation)
// - Token caching for performance
// - Strict security checks (fail-closed behavior)
//
// Example:
//
//	authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
//	    ClientID:  "my-client",
//	    Provider:  router.WrapOIDCProvider(oidcProvider),
//	    Cache:     tokenCache,
//	    Logger:    logger,
//	    IssuerURL: "https://keycloak.example.com/realms/myrealm",
//	    // AllowedAlgorithms defaults to ["RS256", "RS384", "RS512"]
//	    // StoreClaimsInContext defaults to true
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
	if config.IssuerURL == "" {
		return nil, fmt.Errorf("IssuerURL is required")
	}

	// Set defaults for allowed algorithms
	allowedAlgorithms := config.AllowedAlgorithms
	if len(allowedAlgorithms) == 0 {
		// Default to secure RSA algorithms only
		allowedAlgorithms = []string{"RS256", "RS384", "RS512"}
	}

	// Set defaults for required claims
	requiredClaims := config.RequiredClaims
	if len(requiredClaims) == 0 {
		requiredClaims = []string{"exp", "iss", "sub"}
	}

	// Always store claims in context for auditability and access control
	// This is required for VAPT compliance
	storeClaimsInContext := true

	// Create OIDC config with security settings
	oidcConfig := &oidc.Config{
		ClientID:             config.ClientID,
		SupportedSigningAlgs: allowedAlgorithms,
		SkipClientIDCheck:    false, // Always verify client ID
		SkipExpiryCheck:      false, // Always verify expiration
		SkipIssuerCheck:      false, // Always verify issuer
	}

	// Allow overrides only if explicitly set (not recommended for production)
	if config.SkipClientIDCheck {
		oidcConfig.SkipClientIDCheck = true
	}
	if config.SkipExpiryCheck {
		oidcConfig.SkipExpiryCheck = true
	}
	if config.SkipIssuerCheck {
		oidcConfig.SkipIssuerCheck = true
	}

	verifier := config.Provider.Verifier(oidcConfig)

	if config.Logger != nil {
		config.Logger.Log(fmt.Sprintf(
			" Auth middleware initialized with algorithms: %v",
			allowedAlgorithms,
		))
	}

	return &AuthMiddleware{
		Verifier:             verifier,
		Cache:                config.Cache,
		Logger:               config.Logger,
		Provider:             config.Provider,
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

// MiddlewareFunc returns a gin.HandlerFunc (middleware) that performs  token validation
func (a *AuthMiddleware) MiddlewareFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawIDToken, err := ExtractToken(c.Request.Header.Get("Authorization"))
		if err != nil {
			msgID, ok := scenarioToMsgID[TokenMissing]
			if !ok {
				msgID = defaultMsgID
			}
			errCode, ok := scenarioToErrCode[TokenMissing]
			if !ok {
				errCode = defaultErrCode
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, wscutils.NewErrorResponse(msgID, errCode))
			return
		}

		// Perform VAPT-compliant validation with caching
		if err := a.verifyToken(c, rawIDToken); err != nil {
			if a.Logger != nil {
				a.Logger.LogDebug(fmt.Sprintf("Token verification failed: %v", err))
			}
			msgID, ok := scenarioToMsgID[TokenVerificationFailed]
			if !ok {
				msgID = defaultMsgID
			}
			errCode, ok := scenarioToErrCode[TokenVerificationFailed]
			if !ok {
				errCode = defaultErrCode
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, wscutils.NewErrorResponse(msgID, errCode))
			return
		}

		c.Next()
	}
}

// verifyToken performs VAPT-compliant token verification with caching
//
// This method provides comprehensive security validation including:
// - Token caching to reduce OIDC provider calls
// - Algorithm whitelisting
// - Claims validation (exp, iss, nbf, iat, required claims)
// - Custom claims validation
// - Context storage of verified claims
func (a *AuthMiddleware) verifyToken(c *gin.Context, tokenString string) error {
	// Check cache first to reduce OIDC provider calls
	isCached, err := a.Cache.Get(tokenString)
	if err != nil {
		if a.Logger != nil {
			a.Logger.LogDebug(fmt.Sprintf("Cache check failed: %v", err))
		}
		// Continue with verification even if cache fails
		isCached = false
	}

	var claims jwt.MapClaims

	if !isCached {
		// Token not in cache - perform full OIDC verification
		idToken, err := a.Verifier.Verify(c.Request.Context(), tokenString)
		if err != nil {
			return fmt.Errorf("OIDC verification failed: %w", err)
		}

		// Extract all claims
		if err := idToken.Claims(&claims); err != nil {
			return fmt.Errorf("failed to extract claims: %w", err)
		}

		// Cache the verified token to reduce future OIDC calls
		if err := a.Cache.Set(tokenString); err != nil {
			if a.Logger != nil {
				a.Logger.LogDebug(fmt.Sprintf("Failed to cache token: %v", err))
			}
			// Continue even if caching fails
		}
	} else {
		// Token is cached - skip OIDC verification, but still extract claims for validation
		// We need to parse the token to extract claims (without signature verification)
		parser := jwt.Parser{}
		token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
		if err != nil {
			return fmt.Errorf("failed to parse cached token: %w", err)
		}
		var ok bool
		claims, ok = token.Claims.(jwt.MapClaims)
		if !ok {
			return fmt.Errorf("failed to extract claims from cached token")
		}
	}

	//  Validate algorithm (always check, even for cached tokens)
	if err := a.validateAlgorithm(tokenString); err != nil {
		return err
	}

	//  Comprehensive claims validation (always check, even for cached tokens)
	if err := a.validateClaims(claims); err != nil {
		return err
	}

	// Custom claims validation if provided
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
		// Reject tokens issued more than 5 seconds in the future (clock skew grace period)
		if iatTime.After(now.Add(5 * time.Second)) {
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
