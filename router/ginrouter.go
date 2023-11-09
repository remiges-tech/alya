package router

import (
	"github.com/gin-gonic/gin"
	"net/http"
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

// convertHandlerFunc adapts a generic HandlerFunc to a gin.HandlerFunc.
func convertHandlerFunc(handler HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler(&GinContext{ginContext: c})
	}
}

// GET defines a route for GET requests.
func (gr *GinRouter) GET(path string, handler HandlerFunc) {
	gr.engine.GET(path, convertHandlerFunc(handler))
}

// POST defines a route for POST requests.
func (gr *GinRouter) POST(path string, handler HandlerFunc) {
	gr.engine.POST(path, convertHandlerFunc(handler))
}

// PUT defines a route for PUT requests.
func (gr *GinRouter) PUT(path string, handler HandlerFunc) {
	gr.engine.PUT(path, convertHandlerFunc(handler))
}

// DELETE defines a route for DELETE requests.
func (gr *GinRouter) DELETE(path string, handler HandlerFunc) {
	gr.engine.DELETE(path, convertHandlerFunc(handler))
}

// Use applies middleware to the router.
func (gr *GinRouter) Use(middleware MiddlewareFunc) {
	gr.engine.Use(func(c *gin.Context) {
		// Wrap the request in our generic Context interface
		ctx := &GinContext{ginContext: c}

		// following is equivalent to:
		// middleware(ctx, func(ctx Context) { c.Next() })(ctx)
		// basically, it passes func(ctx Context) { c.Next() } as the next parameter to the middleware
		// which then returns its hanlderfunc which is then called immediately with ctx as the parameter
		next := middleware(ctx, func(ctx Context) {
			// it will be called when next() is called in the middleware
			c.Next()
		})
		// Call the next middleware/handler in the chain
		next(ctx)
	})
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
