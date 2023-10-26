package infra

import (
	"context"
	"go-framework/internal/pg"
	"go-framework/internal/pg/sqlc-gen"
	middleware "go-framework/internal/wscutils"
	"go-framework/logharbour"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

var (
	clientID      = "account"
	clientSecret  = "OFyNKbP1..."
	keycloakURL   = "https://localhost/auth/realms/myrealm"
	redisAddr     = "localhost:6379"
	redisPassword = ""
	redisDB       = 0
)

// initInfraServices sets up required infrastructure services. All the database connections,
// logger, etc. are initialized here.
func InitInfraServices() (*sqlc.Queries, *logharbour.LogHarbour, *redis.Client) {
	// Establish Env -- connection connections, logger, etc.
	pgConfig := pg.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "erptest",
		Password: "erptest",
		DBName:   "erptest",
	}
	pgProvider := pg.NewProvider(pgConfig)
	sqlq := sqlc.New(pgProvider.DB())
	lh := logharbour.New()

	// Set up Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	return sqlq, lh, rdb
}

func setupMiddleware(rdb *redis.Client) gin.HandlerFunc {
	// Set up Redis cache and Verifier
	redisCache := &middleware.RedisTokenCache{
		Client: rdb,
		Ctx:    context.Background(),
	}

	provider, _ := oidc.NewProvider(context.Background(), keycloakURL)
	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})
	authMiddleware := middleware.NewAuthMiddleware(clientID, clientSecret, keycloakURL, verifier, redisCache)

	return authMiddleware.MiddlewareFunc()
}

// SetupRouter sets up the Gin router with middleware.
func SetupRouter(lh *logharbour.LogHarbour, rdb *redis.Client) *gin.Engine {
	authMiddleware := setupMiddleware(rdb)
	r := gin.Default()
	r.Use(authMiddleware)

	logger := &middleware.CustomLogger{}
	r.Use(middleware.CustomLoggerMiddleware(logger))

	return r
}
