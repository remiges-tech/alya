package main

import (
	"github.com/gin-gonic/gin"
)

type User struct {
	Fullname string `json:"fullname" valid:"required"`    // Requires a fullname.
	Email    string `json:"email" valid:"required,email"` // Requires a properly formatted email.
	Age      int    `json:"age" valid:"range(1|150)"`     // Requires age to be between 1 and 150.
}

func main() {
	router := gin.Default()

	router.POST("/user", func(c *gin.Context) {
		var user User
		c.BindJSON(&user)

		// If the validation fails, validationErrors will contain the details of the errors.
		validationErrors := Validate(user)

		// If there are any errors, respond with a 400 status code (Bad Request)
		// and the validation errors.
		if len(validationErrors) > 0 {
			c.JSON(400, gin.H{
				"status":   "error",
				"data":     gin.H{},
				"messages": validationErrors,
			})
			return
		}

		// If the incoming JSON could be successfully validated and processed,
		// proceed with the rest of your handler here and send a positive response
		c.JSON(200, gin.H{
			"status":   "success",
			"data":     user,
			"messages": []string{},
		})
	})

	router.Run(":8080") // Starts the Gin server.
}
