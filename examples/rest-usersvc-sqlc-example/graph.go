package main

import (
	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/di"
	"github.com/remiges-tech/alya/examples/rest-usersvc-sqlc-example/app"
	pg "github.com/remiges-tech/alya/examples/rest-usersvc-sqlc-example/pg"
	"github.com/remiges-tech/alya/examples/rest-usersvc-sqlc-example/repository"
	"github.com/remiges-tech/alya/examples/rest-usersvc-sqlc-example/transport"
	"github.com/remiges-tech/alya/service"
)

//go:generate go run ../../cmd/alya-di gen -graph Graph -out zz_generated_di.go

// Graph declares the startup dependency graph for the REST SQLC example.
//
// The alya-di generator reads this declaration and emits a Build(...) function in
// zz_generated_di.go. The generated build function performs plain constructor
// calls in dependency order. There is no runtime container and no runtime
// dependency lookup.
var Graph = di.New(
	di.Inputs(
		di.Type[*gin.Engine](),
		di.Type[AppConfig](),
	),
	di.Provide(
		newPGConfig,
		newPGProvider,
		newSQLCRepository,
		newValidator,
		service.NewService,
		app.NewUserService,
		app.NewOrderService,
		transport.NewUserHandler,
		transport.NewOrderHandler,
	),
	di.Bind[repository.UserRepository, *repository.SQLCRepository](),
	di.Bind[repository.OrderRepository, *repository.SQLCRepository](),
	di.Invoke(transport.RegisterRoutes),
	di.Outputs(
		di.Type[*service.Service](),
		di.Type[*transport.UserHandler](),
		di.Type[*transport.OrderHandler](),
	),
)

// newPGConfig adapts the example's startup config into the pg provider config.
func newPGConfig(cfg AppConfig) pg.Config {
	return pg.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
	}
}

// newPGProvider adapts pg.NewProvider to the cleanup-aware provider shape that
// the generator understands.
func newPGProvider(cfg pg.Config) (*pg.Provider, func(), error) {
	provider, err := pg.NewProvider(cfg)
	if err != nil {
		return nil, nil, err
	}
	return provider, func() {
		_ = provider.Close()
	}, nil
}

// newSQLCRepository adapts the provider's Queries() method into a constructor the
// graph can reason about.
func newSQLCRepository(provider *pg.Provider) *repository.SQLCRepository {
	return repository.NewSQLCRepository(provider.Queries())
}
