// Package router provides routing capabilities and associated middlewares for the Alya framework.
// This file, logging_middleware.go, implements a request logging middleware
// for Alya-based applications. It captures detailed information about each incoming HTTP request
// and its corresponding response, and logs this information in a structured format using
// the Remiges LogHarbour library.
//
// Functionalities:
//   - Captures request details: HTTP method, path, query parameters, client IP, user agent,
//     referer, request size, trace ID, and span ID.
//   - Captures response details: Status code and response size.
//   - Calculates request processing duration.
//   - Logs a structured entry at the end of each request lifecycle.
//   - Uses an adapter pattern (RequestLogger interface and LogHarbourAdapter) to decouple
//     the logging middleware from specific logging implementations. This design provides Alya
//     the flexibility to support other logging libraries in the future by creating new adapters.
//
// Components:
//   - RequestInfo: A struct that holds all the captured information for a single request/response cycle.
//     The 'StartTime' field indicates when the request processing began and is logged in UTC.
//   - RequestLogger: An interface that defines the contract for logging RequestInfo. This allows
//     for different logging backends to be used if needed.
//   - LogHarbourAdapter: An implementation of RequestLogger that uses the Remiges LogHarbour
//     library to write the log entries.
//   - LogRequest: The Gin middleware handler function. It orchestrates the capturing of data,
//     calls the next handlers in the chain, and then triggers the logging via the RequestLogger.
//
// Usage:
// To use this middleware, create an instance of a RequestLogger (e.g., LogHarbourAdapter initialized
// with a LogHarbour logger) and then add the LogRequest middleware to your Gin router:
//
//	logger := logharbour.NewLogger(...)
//	logAdapter := router.NewLogHarbourAdapter(logger)
//	ginRouter.Use(router.LogRequest(logAdapter))
//
// This setup ensures that every request handled by the ginRouter will be logged with the
// configured details.
//
// Using a Custom Logger:
// The RequestLogger interface allows for integration of any custom logger.
// To use your own logger:
//  1. Define a struct for your adapter (e.g., MyCustomLoggerAdapter) -- this struct would typically
//     hold an instance of your custom logger.
//  2. Implement the `Log(info RequestInfo)` method for your adapter struct.
//     Inside this method, transform `RequestInfo` into your logger's desired format and log it.
//  4. Create an instance of your adapter and pass it to `router.LogRequest()`.
//
// Example of a custom adapter structure:
//
//	import (
//	    "fmt" // Or your custom logger package
//	    "github.com/remiges-tech/alya/router"
//	)
//
//	type MyCustomLoggerAdapter struct {
//	    // myLogger *yourcustomlogger.Logger // Example: if your logger has a struct
//	}
//
//	func NewMyCustomLoggerAdapter(/* custom logger instance */) *MyCustomLoggerAdapter {
//	    return &MyCustomLoggerAdapter{/* initialize with logger */}
//	}
//
//	func (a *MyCustomLoggerAdapter) Log(info router.RequestInfo) {
//	    // Example: simple print, replace with your logger's methods
//	    logLine := fmt.Sprintf("Method: %s, Path: %s, Status: %d, Duration: %s\n",
//	        info.Method, info.Path, info.StatusCode, info.Duration)
//	    fmt.Print(logLine) // Replace with: a.myLogger.Info(logLine) or similar
//	}
//
// Then, in your main setup:
//
//	customAdapter := NewMyCustomLoggerAdapter(/* ... */)
//	ginRouter.Use(router.LogRequest(customAdapter))
package router

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/logharbour/logharbour"
)

// RequestInfo contains all the information about a request to be logged
type RequestInfo struct {
	Method             string        `json:"method"`                       // HTTP method (e.g., "GET", "POST")
	Path               string        `json:"path"`                         // Request path (e.g., "/users/123")
	ClientIP           string        `json:"client_ip"`                    // Client's IP address
	StatusCode         int           `json:"status_code"`                  // HTTP status code of the response (e.g., 200, 404)
	StartTime          time.Time     `json:"start_time"`                   // Time when request processing started (UTC)
	Duration           time.Duration `json:"duration"`                     // Total duration of request processing
	RequestSize        int64         `json:"request_size"`                 // Size of the request body in bytes
	ResponseSize       int64         `json:"response_size"`                // Size of the response body in bytes
	Query              string        `json:"query,omitempty"`              // Raw query string (e.g., "id=1&name=test")
	UserAgent          string        `json:"user_agent,omitempty"`         // User-Agent header from the request
	Referer            string        `json:"referer,omitempty"`            // Referer header from the request
	TraceID            string        `json:"trace_id,omitempty"`           // Trace ID for distributed tracing
	SpanID             string        `json:"span_id,omitempty"`            // Span ID for distributed tracing
	TimedOut           bool          `json:"timed_out,omitempty"`          // True if request exceeded timeout
	ClientDisconnected bool          `json:"client_disconnected,omitempty"` // True if client closed connection
	PanicRecovered     bool          `json:"panic_recovered,omitempty"`    // True if handler panicked
	PanicValue         string        `json:"panic_value,omitempty"`        // Panic message if handler panicked
}

// RequestLogger defines the interface that a logger must implement to be used with LogRequest middleware
type RequestLogger interface {
	Log(info RequestInfo)
}

// LogRequest returns a Gin middleware that logs details about a request at the end of the request lifecycle.
// It logs a single entry at the end of the request, including:
// - HTTP method
// - Path
// - Status code
// - Client IP
// - Request timestamp
// - Request duration
// - Request size
// - Response size
// - Trace ID (if available)
// - Span ID (if available)
func LogRequest(logger RequestLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Record start time
		startTime := time.Now()

		// Store the content length before processing
		requestSize := c.Request.ContentLength

		// Get trace and span IDs if available
		traceID := c.GetHeader("X-Trace-ID")
		spanID := c.GetHeader("X-Span-ID")

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(startTime)

		// Check for timeout/disconnect/panic context keys set by TimeoutMiddleware.
		// TimeoutMiddleware sets these keys to communicate events to this logging middleware:
		//   - CtxKeyTimedOut: request exceeded configured timeout
		//   - CtxKeyClientDisconnected: client closed connection before response
		//   - CtxKeyPanicRecovered, CtxKeyPanicValue: handler panicked
		var timedOut, clientDisconnected, panicRecovered bool
		var panicValue string
		if v, exists := c.Get(CtxKeyTimedOut); exists {
			timedOut, _ = v.(bool)
		}
		if v, exists := c.Get(CtxKeyClientDisconnected); exists {
			clientDisconnected, _ = v.(bool)
		}
		if v, exists := c.Get(CtxKeyPanicRecovered); exists {
			panicRecovered, _ = v.(bool)
		}
		if v, exists := c.Get(CtxKeyPanicValue); exists {
			panicValue, _ = v.(string)
		}

		// Create request info
		info := RequestInfo{
			Method:             c.Request.Method,
			Path:               c.Request.URL.Path,
			ClientIP:           c.ClientIP(),
			StatusCode:         c.Writer.Status(),
			StartTime:          startTime.UTC(), // Convert to UTC
			Duration:           duration,
			RequestSize:        requestSize,
			ResponseSize:       int64(c.Writer.Size()),
			Query:              c.Request.URL.RawQuery,
			UserAgent:          c.Request.UserAgent(),
			Referer:            c.Request.Referer(),
			TraceID:            traceID,
			SpanID:             spanID,
			TimedOut:           timedOut,
			ClientDisconnected: clientDisconnected,
			PanicRecovered:     panicRecovered,
			PanicValue:         panicValue,
		}

		// Log request details using the provided logger
		logger.Log(info)
	}
}

// LogHarbourAdapter adapts a LogHarbour logger to implement the RequestLogger interface
type LogHarbourAdapter struct {
	logger *logharbour.Logger
}

// NewLogHarbourAdapter creates a new adapter for a LogHarbour logger
func NewLogHarbourAdapter(logger *logharbour.Logger) *LogHarbourAdapter {
	return &LogHarbourAdapter{
		logger: logger,
	}
}

// Log implements the RequestLogger interface by using LogHarbour's structured logging
func (a *LogHarbourAdapter) Log(info RequestInfo) {
	// Create a structured activity log entry
	logger := a.logger.WithModule("http").
		WithOp("request").
		WithRemoteIP(info.ClientIP).
		WithClass(info.Method).
		WithInstanceId(info.Path).
		WithStatus(getStatus(info.StatusCode))

	// Create the activity data map
	activityData := map[string]interface{}{
		"method":        info.Method,
		"path":          info.Path,
		"status":        info.StatusCode,
		"start_time":    info.StartTime.Format(time.RFC3339),
		"duration_ms":   info.Duration.Milliseconds(),
		"duration":      info.Duration.String(),
		"request_size":  info.RequestSize,
		"response_size": info.ResponseSize,
		"query":         info.Query,
		"user_agent":    info.UserAgent,
		"referer":       info.Referer,
	}

	// Add trace and span IDs if available
	if info.TraceID != "" {
		activityData["trace_id"] = info.TraceID
	}

	if info.SpanID != "" {
		activityData["span_id"] = info.SpanID
	}

	// Add timeout, client disconnect, and panic info if present
	if info.TimedOut {
		activityData["timed_out"] = true
	}
	if info.ClientDisconnected {
		activityData["client_disconnected"] = true
	}
	if info.PanicRecovered {
		activityData["panic_recovered"] = true
		activityData["panic_value"] = info.PanicValue
	}

	// Log the activity with the collected data
	logger.Info().LogActivity("HTTP request completed", activityData)
}

// getStatus converts an HTTP status code to a logharbour Status
func getStatus(statusCode int) logharbour.Status {
	if statusCode >= 200 && statusCode < 400 {
		return logharbour.Success
	}
	return logharbour.Failure
}
