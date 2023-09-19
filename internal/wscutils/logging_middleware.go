package wscutils

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"time"
)

type CustomLogger struct{}

func (l *CustomLogger) Log(message string) {
	// Implement your logging logic here
	fmt.Println(message)
}

func CustomLoggerMiddleware(logger *CustomLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Collect log data
		method := c.Request.Method
		path := c.Request.URL.Path
		status := c.Writer.Status()

		// Log the data using your custom logger
		logger.Log(fmt.Sprintf("Method: %s, Path: %s, Status: %d, Latency: %v", method, path, status, latency))
	}
}
