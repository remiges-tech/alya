package restutils

import "github.com/gin-gonic/gin"

// WriteJSON writes a JSON response and aborts the request.
func WriteJSON(c *gin.Context, status int, data any) {
	c.AbortWithStatusJSON(status, data)
}

// WriteOK writes a 200 JSON response.
func WriteOK(c *gin.Context, data any) {
	WriteJSON(c, 200, data)
}

// WriteCreated writes a 201 JSON response and optional Location header.
func WriteCreated(c *gin.Context, location string, data any) {
	if location != "" {
		c.Header("Location", location)
	}
	WriteJSON(c, 201, data)
}

// WriteAccepted writes a 202 JSON response.
func WriteAccepted(c *gin.Context, data any) {
	WriteJSON(c, 202, data)
}

// WriteNoContent writes a 204 response and aborts the request.
func WriteNoContent(c *gin.Context) {
	c.AbortWithStatus(204)
}
