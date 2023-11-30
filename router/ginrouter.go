package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GinRouter implements the Router interface for Gin framework.
type GinRouter struct {
	engine *gin.Engine
}

// NewGinRouter is a constructor for GinRouter.
func NewGinRouter() *GinRouter {
	r := gin.New()

	// Attach the logging middleware provided by Gin.
	r.Use(gin.Logger())

	// Attach the recovery middleware provided by Gin.
	r.Use(gin.Recovery())

	// Assume authMiddleware and loggingMiddleware are defined elsewhere in the package.
	r.Use(authMiddleware)
	r.Use(loggingMiddleware)

	return &GinRouter{engine: r}
}

// authMiddleware is the default auth middleware used in Alya
var authMiddleware gin.HandlerFunc = func(c *gin.Context) {
	// Authentication logic would be implemented here
	c.Next()
}

// loggingMiddleware is the default request logging middleware used in Alya
var loggingMiddleware gin.HandlerFunc = func(c *gin.Context) {
	// Logging logic would be implemented here
	c.Next()
}

// Serve starts the HTTP server at the specified address.
func (gr *GinRouter) Serve(address string) error {
	return gr.engine.Run(address)
}

// GinContext is an adapter that implements the Context interface for Gin.
type GinContext struct {
	ginContext *gin.Context
}

// JSON sends a JSON response.
func (gc *GinContext) JSON(code int, obj any) {
	gc.ginContext.JSON(code, obj)
}

// Bind binds the request body to obj.
func (gc *GinContext) Bind(obj any) error {
	return gc.ginContext.ShouldBind(obj)
}

// BindJSON binds the JSON request body into obj.
func (gc *GinContext) BindJSON(obj any) error {
	return gc.ginContext.ShouldBindJSON(obj)
}

// Request returns the underlying http.Request.
func (gc *GinContext) Request() *http.Request {
	return gc.ginContext.Request
}
