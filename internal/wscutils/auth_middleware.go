package wscutils

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type TokenVerifier interface {
	Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error)
}

type TokenCache interface {
	Get(token string) (bool, error)
	Set(token string) error
}

type RedisTokenCache struct {
	Client *redis.Client
	Ctx    context.Context
}

func (r *RedisTokenCache) Set(token string) error {
	err := r.Client.Set(r.Ctx, token, true, 24*time.Hour).Err()
	return err
}

func (r *RedisTokenCache) Get(token string) (bool, error) {
	val, err := r.Client.Exists(r.Ctx, token).Result()
	if err != nil {
		return false, err
	}
	return val > 0, nil
}

type AuthMiddleware struct {
	ClientID     string
	ClientSecret string
	KeycloakURL  string
	Verifier     TokenVerifier
	Cache        TokenCache
}

func NewAuthMiddleware(clientID string, clientSecret string, keycloakURL string, verifier TokenVerifier, cache TokenCache) *AuthMiddleware {
	return &AuthMiddleware{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		KeycloakURL:  keycloakURL,
		Verifier:     verifier,
		Cache:        cache,
	}
}

func (a *AuthMiddleware) MiddlewareFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawIDToken, err := ExtractToken(c.Request.Header.Get("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		isCached, err := a.Cache.Get(rawIDToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if !isCached {
			_, err := a.Verifier.Verify(context.Background(), rawIDToken)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Failed to verify ID Token: %v", err)})
				return
			}

			err = a.Cache.Set(rawIDToken)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to cache ID Token: %v", err)})
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
