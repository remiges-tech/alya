package router

import "net/http"

// HandlerFunc defines the handler signature for our router.
type HandlerFunc func(Context)

// MiddlewareFunc defines the middleware signature.
// The HandlerFunc parameter represents the next function to be executed in the request handling chain.
// This could be another middleware function or the final request handler.
// By passing the next function as a parameter, the MiddlewareFunc has the ability to control the execution flow.
// It can choose to all the next function immediately, delay its execution, or even not call it at all.
// This is useful for implementing functionalities like error handling, request filtering.
type MiddlewareFunc func(Context, HandlerFunc) HandlerFunc

// Router is a generic interface that can be implemented by different web frameworks.
type Router interface {
	GET(path string, handler HandlerFunc)
	POST(path string, handler HandlerFunc)
	PUT(path string, handler HandlerFunc)
	DELETE(path string, handler HandlerFunc)
	Use(middleware MiddlewareFunc)
	Serve(address string) error
}

// Context is a generic interface that abstracts the context provided by web frameworks.
// It allows us to interact with the request and response in a generic way.
type Context interface {
	// JSON sends a JSON response with the given status code and object.
	// The object can be any data structure, and it will be serialized to JSON.
	JSON(code int, obj any)

	// BindJSON decodes the json payload into the struct specified as a pointer.
	BindJSON(obj any) error

	// Request returns the underlying http.Request.
	// This can be used to access details about the request, such as headers, query parameters, and the request method.
	Request() *http.Request
}
