package main

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/logharbour/logharbour"

	"github.com/remiges-tech/alya/router"
)

func main() {
	// Create a logger context with the default priority
	lctx := logharbour.NewLoggerContext(logharbour.Info)

	// Initialize the logger with stdout as the writer
	logger := logharbour.NewLogger(lctx, "GinExample", os.Stdout)

	// Create the LogHarbour adapter that implements RequestLogger interface
	logAdapter := router.NewLogHarbourAdapter(logger)

	// Create a new Gin router
	r := gin.New()

	// Use our custom logging middleware with the adapter
	r.Use(router.LogRequest(logAdapter))

	// Recovery middleware recovers from any panics and writes a 500 if there was one
	r.Use(gin.Recovery())

	// Define a simple endpoint
	r.GET("/ping", func(c *gin.Context) {
		// Simulate some processing time
		time.Sleep(50 * time.Millisecond)
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	// Define an endpoint that returns an error
	r.GET("/error", func(c *gin.Context) {
		// Simulate some processing time
		time.Sleep(100 * time.Millisecond)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong",
		})
	})

	// Define a POST endpoint to demonstrate request with body
	r.POST("/users", func(c *gin.Context) {
		// Simulate some processing time
		time.Sleep(150 * time.Millisecond)
		c.JSON(http.StatusCreated, gin.H{
			"message": "User created successfully",
		})
	})

	// Run the server
	r.Run(":8090") // Listen and serve on 0.0.0.0:8090
}
