package service_test

import (
	"context"
	"testing"

	"github.com/remiges-tech/alya/config"
	"github.com/remiges-tech/alya/service"
)

type MockConfig struct{}

func (mc *MockConfig) LoadConfig(c any) error {
	return nil
}

func (mc *MockConfig) Check() error {
	return nil
}

func (mc *MockConfig) Get(key string) (string, error) {
	return "dummy", nil
}

func (mc *MockConfig) Watch(ctx context.Context, key string, events chan<- config.Event) error {
	return nil
}

func TestWithConfig(t *testing.T) {
	cfg := &MockConfig{} // Create a mock config

	s := service.NewService(nil) // Create a new service with nil router

	s.WithConfig(cfg) // Call WithConfig method

	if s.Config != cfg { // Check if Config field is correctly set
		t.Errorf("WithConfig() = %v, want %v", s.Config, cfg)
	}
}

// Example demonstrates how to create a new service and register routes.
func Example() {
	// // Shared router
	// router := gin.Default()

	// // UserService with authentication middleware
	// userService := NewService(router).WithLogger(logger).WithDatabase(db)

	// // Version 1 of UserService
	// userGroupV1 := userService.CreateGroup("/v1/user")
	// userGroupV1.Group.Use(authMiddleware) // Apply auth middleware to userGroupV1

	// // Register routes for version 1 of UserService
	// userGroupV1.RegisterRoute(http.MethodGet, "/profile", userProfileHandlerV1)         // Endpoint: GET /v1/user/profile
	// userGroupV1.RegisterRoute(http.MethodPut, "/profile", userUpdateProfileHandlerV1)   // Endpoint: PUT /v1/user/profile
	// userGroupV1.RegisterRoute(http.MethodGet, "/settings", userSettingsHandlerV1)       // Endpoint: GET /v1/user/settings
	// userGroupV1.RegisterRoute(http.MethodPut, "/settings", userUpdateSettingsHandlerV1) // Endpoint: PUT /v1/user/settings

	// // Version 2 of UserService
	// userGroupV2 := userService.CreateGroup("/v2/user")
	// userGroupV2.Group.Use(authMiddleware) // Apply auth middleware to userGroupV2

	// // Register routes for version 2 of UserService
	// userGroupV2.RegisterRoute(http.MethodGet, "/profile", userProfileHandlerV2)         // Endpoint: GET /v2/user/profile
	// userGroupV2.RegisterRoute(http.MethodPut, "/profile", userUpdateProfileHandlerV2)   // Endpoint: PUT /v2/user/profile
	// userGroupV2.RegisterRoute(http.MethodGet, "/settings", userSettingsHandlerV2)       // Endpoint: GET /v2/user/settings
	// userGroupV2.RegisterRoute(http.MethodPut, "/settings", userUpdateSettingsHandlerV2) // Endpoint: PUT /v2/user/settings

	// // BlogService without authentication middleware
	// blogService := NewService(router).WithLogger(logger).WithDatabase(db)

	// // BlogService group
	// blogGroup := blogService.CreateGroup("/blog")

	// // Register routes for BlogService
	// blogGroup.RegisterRoute(http.MethodGet, "/posts", blogPostsHandler)       // Endpoint: GET /blog/posts
	// blogGroup.RegisterRoute(http.MethodPost, "/posts", blogCreatePostHandler) // Endpoint: POST /blog/posts

	// // Run the server on port 8080
	// router.Run(":8080")
}
