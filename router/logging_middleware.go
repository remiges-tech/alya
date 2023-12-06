package router

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/logger"
)

func CustomLoggerMiddleware(logger logger.Logger) gin.HandlerFunc {
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
