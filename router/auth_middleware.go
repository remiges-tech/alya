package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
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
	Verifier *oidc.IDTokenVerifier
	Cache    TokenCache
	Logger   logger.Logger
}

func NewAuthMiddleware(clientID string, provider *oidc.Provider, cache TokenCache, logger logger.Logger) (*AuthMiddleware, error) {
	oidcConfig := &oidc.Config{
		ClientID: clientID,
	}
	verifier := provider.Verifier(oidcConfig)

	return &AuthMiddleware{
		Verifier: verifier,
		Cache:    cache,
		Logger:   logger,
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
