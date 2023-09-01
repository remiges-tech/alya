package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
)

type User struct {
	Fullname string `json:"fullname" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Age      int    `json:"age" validate:"min=10,max=150"`
}

func main() {
	router := gin.Default()

	router.POST("/user", func(c *gin.Context) {
		var user User
		c.BindJSON(&user)

		getValuesFunc := func(data User, fieldName string) []string {
			var inputValue, validValue string

			switch fieldName {
			case "Age":
				inputValue = fmt.Sprintf("%v", data.Age)
				// Define the logic to get valid value for Age
				validValue = "10-150"
			}

			return []string{inputValue, validValue}
		}

		validationErrors := Validate(user, getValuesFunc)

		if len(validationErrors) > 0 {
			c.JSON(400, gin.H{
				"status":   "error",
				"data":     gin.H{},
				"messages": validationErrors,
			})
			return
		}

		c.JSON(200, gin.H{
			"status":   "success",
			"data":     user,
			"messages": []string{},
		})
	})

	router.Run(":8080")
}
