package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/router"
	"github.com/remiges-tech/alya/service"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/remiges-tech/logharbour/logharbour"
)

func main() {
	// Set up logger
	fallbackWriter := logharbour.NewFallbackWriter(os.Stdout, os.Stdout)
	lctx := logharbour.NewLoggerContext(logharbour.DefaultPriority)
	logger := logharbour.NewLogger(lctx, "TimeoutService", fallbackWriter)
	logger.WithPriority(logharbour.Debug2)

	// Create the LogHarbour adapter for request logging
	logAdapter := router.NewLogHarbourAdapter(logger)

	// Create Gin router
	r := gin.Default()

	// Add request logging middleware
	r.Use(router.LogRequest(logAdapter))

	// Register timeout error codes and messages
	router.RegisterMiddlewareMsgID(router.RequestTimeout, 5001)
	router.RegisterMiddlewareErrCode(router.RequestTimeout, "request_timeout")

	// Add timeout middleware (5 seconds)
	r.Use(router.TimeoutMiddleware(2 * time.Minute))

	// Create service
	timeoutService := service.NewService(r).WithLogHarbour(logger)

	// Register routes
	timeoutService.RegisterRoute(http.MethodGet, "/quick", handleQuickRequest)
	timeoutService.RegisterRoute(http.MethodGet, "/slow", handleSlowRequest)

	// Start server
	if err := r.Run(":8090"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// Quick request handler - returns immediately
func handleQuickRequest(c *gin.Context, s *service.Service) {
	startTime := time.Now()
	s.LogHarbour.LogActivity("Quick request started", map[string]any{
		"startTime": startTime.Format(time.RFC3339),
	})

	response := map[string]string{
		"message": "Quick response",
		"time":    time.Now().Format(time.RFC3339),
	}

	endTime := time.Now()
	s.LogHarbour.LogActivity("Quick request completed", map[string]any{
		"startTime": startTime.Format(time.RFC3339),
		"endTime":   endTime.Format(time.RFC3339),
		"duration":  endTime.Sub(startTime).String(),
	})

	wscutils.SendSuccessResponse(c, wscutils.NewSuccessResponse(response))
}

// Slow request handler - simulates a long-running operation
func handleSlowRequest(c *gin.Context, s *service.Service) {
	startTime := time.Now()
	s.LogHarbour.LogActivity("Slow request started", map[string]any{
		"startTime": startTime.Format(time.RFC3339),
	})

	s.LogHarbour.LogActivity("Starting long operation", map[string]any{
		"sleepDuration": "3 minutes",
		"currentTime":   time.Now().Format(time.RFC3339),
	})

	// Simulate long processing (3 minutes)
	time.Sleep(3 * time.Minute)

	// This part may not execute due to timeout
	endTime := time.Now()
	s.LogHarbour.LogActivity("Slow request completed", map[string]any{
		"startTime": startTime.Format(time.RFC3339),
		"endTime":   endTime.Format(time.RFC3339),
		"duration":  endTime.Sub(startTime).String(),
	})

	response := map[string]string{
		"message": "Slow response completed",
		"time":    time.Now().Format(time.RFC3339),
	}
	wscutils.SendSuccessResponse(c, wscutils.NewSuccessResponse(response))
}
