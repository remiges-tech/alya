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

// MiddlewareFunc returns a gin.HandlerFunc (middleware) that checks for a valid token.
func (a *AuthMiddleware) MiddlewareFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawIDToken, err := ExtractToken(c.Request.Header.Get("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, wscutils.NewErrorResponse(wscutils.ErrcodeTokenMissing))
			return
		}

		isCached, err := a.Cache.Get(rawIDToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, wscutils.NewErrorResponse(wscutils.ErrcodeTokenCacheFailed))
			return
		}

		if !isCached {
			_, err := a.Verifier.Verify(context.Background(), rawIDToken)
			if err != nil {
				a.Logger.Log(fmt.Sprintf("Auth error: %v", err)) // Use the logger for logging
				c.Set("auth_error", err)                         // Set the error in the context
				c.AbortWithStatusJSON(http.StatusUnauthorized, wscutils.NewErrorResponse(wscutils.ErrcodeTokenVerificationFailed))
				return
			}

			err = a.Cache.Set(rawIDToken)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, wscutils.NewErrorResponse(wscutils.ErrcodeTokenCacheFailed))
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
