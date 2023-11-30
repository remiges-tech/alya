// Package service provides a way to create a web service using other packages in Alya framework.
//
// It provides a way to inject dependencies into the service.
//
// It can can be used to build a RESTful API server where each resource can be developed as a service.
// To help with that it supports creation of route groups and sub-groups. Each group can have its own
// routes and middleware. It allows grouping of routes by functionality.
package service

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/config"
	"github.com/remiges-tech/logharbour/logharbour"
)

// Dependencies is a map to hold arbitrary dependencies.
type Dependencies map[string]any

// Service is the core struct for a web service, holding essential components and optional dependencies.
// It also provides a map to hold arbitrary dependencies. It allows injecttion of any
// additional components that a service might need.
// Note: Assert the type of the dependency before using it because the value is of type any.
//
// Example:
//
//	  redisClient := // create Redis client
//	  s := NewService(cfg, router).WithDependency("redis", redisClient)
//	  value, ok := s.Dependencies["redis"]
//		 if !ok {
//			 Handle missing Redis client
//		 }
//
// The Service struct also provides a set of With... methods to inject specific dependencies.
//
// Example:
//
//	s := NewService(router).WithLogger(logger).WithDatabase(db)
type Service struct {
	Config       config.Config
	Router       *gin.Engine
	Logger       *logharbour.Logger
	Database     any
	Dependencies Dependencies
}

// NewService constructs a new Service with the given configuration, router, and options.
func NewService(r *gin.Engine) *Service {
	s := &Service{
		Router: r,
	}
	return s
}

// WithDependency is a method to inject an arbitrary dependency into the Service.
func (s *Service) WithDependency(key string, value any) *Service {
	if s.Dependencies == nil {
		s.Dependencies = make(Dependencies)
	}
	s.Dependencies[key] = value
	return s
}

// WithLogger is a method to inject a logger dependency into the Service.
func (s *Service) WithLogger(l *logharbour.Logger) *Service {
	s.Logger = l
	return s
}

// WithDatabase is a method to inject a database dependency into the Service.
func (s *Service) WithDatabase(db any) *Service {
	s.Database = db
	return s
}

// HandlerFunc is a function that handles a request.
// It takes a *gin.Context and a *Service as parameters.
type HandlerFunc func(*gin.Context, *Service)

// RegisterRoute allows for the registration of a single route directly on the service's engine.
func (s *Service) RegisterRoute(method, path string, handler HandlerFunc) {
	wrappedHandler := func(c *gin.Context) {
		handler(c, s)
	}
	switch method {
	case http.MethodGet:
		s.Router.GET(path, wrappedHandler)
	case http.MethodPost:
		s.Router.POST(path, wrappedHandler)
	case http.MethodPut:
		s.Router.PUT(path, wrappedHandler)
	case http.MethodDelete:
		s.Router.DELETE(path, wrappedHandler)
	default:
		// Handle unsupported methods
		log.Printf("Unsupported method: %s", method)
	}
}

// RouteGroup represents a group of routes.
type RouteGroup struct {
	Group *gin.RouterGroup
}

// CreateGroup creates a new route group with the given path.
func (s *Service) CreateGroup(path string) *RouteGroup {
	return &RouteGroup{
		Group: s.Router.Group(path),
	}
}

// RegisterRoute allows for the registration of a single route to the route group.
func (g *RouteGroup) RegisterRoute(method, path string, handler gin.HandlerFunc) {
	switch method {
	case http.MethodGet:
		g.Group.GET(path, handler)
	case http.MethodPost:
		g.Group.POST(path, handler)
	case http.MethodPut:
		g.Group.PUT(path, handler)
	case http.MethodDelete:
		g.Group.DELETE(path, handler)
	default:
		// Handle unsupported methods
		log.Printf("Unsupported method: %s", method)
	}
}

// CreateSubGroup creates a new sub-group within the current group.
func (g *RouteGroup) CreateSubGroup(path string) *RouteGroup {
	return &RouteGroup{
		Group: g.Group.Group(path),
	}
}
