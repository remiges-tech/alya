package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/service"
	"github.com/remiges-tech/alya/wscutils"
)

// UserProfile contains user profile data with optional fields
type UserProfile struct {
	Username    string                         `json:"username"`
	Email       wscutils.Optional[string]      `json:"email"`
	IsActive    wscutils.Optional[bool]        `json:"isActive"`
	Preferences wscutils.Optional[Preferences] `json:"preferences"`
	Score       int                            `json:"score"` // Regular non-optional field
}

// Preferences contains user preferences
type Preferences struct {
	Theme wscutils.Optional[string]            `json:"theme"`
	Tags  map[string]wscutils.Optional[string] `json:"tags"`
}

// updateProfileHandler handles profile updates with optional fields
func updateProfileHandler(c *gin.Context, s *service.Service) {
	var request UserProfile

	// Use wscutils to bind the request
	wscutils.BindJSON(c, &request)

	// Create a response to show what fields would be updated in the database
	// Our response data will be of the format:
	// {
	//   "username": "johndoe",
	//   "updates": { "email": "...", "isActive": false }
	// }
	response := make(map[string]any)
	response["username"] = request.Username // Always include in the response
	response["updates"] = make(map[string]any)

	// Check each optional field
	updates := response["updates"].(map[string]any)

	// Handle email - only update if value is present, otherwise don't update
	if email, ok := request.Email.Get(); ok {
		updates["email"] = email
	}

	// Handle IsActive status - only update if value is present, otherwise don't update
	if isActive, ok := request.IsActive.Get(); ok {
		updates["isActive"] = isActive
	}

	// Handle preferences (complex type) with special handling for tags
	if prefs, ok := request.Preferences.Get(); ok {
		prefsMap := make(map[string]interface{})

		// Process theme field with Optional - only update if value is present
		if theme, themeOk := prefs.Theme.Get(); themeOk {
			prefsMap["theme"] = theme
		}

		// Handle tags with detailed logging to show the three-state pattern
		if prefs.Tags != nil {
			tagsMap := make(map[string]string)
			for tagName, tagOptional := range prefs.Tags {
				if tagOptional.Null {
					// Tag explicitly set to null - should be removed
					fmt.Printf("Tag '%s' will be REMOVED (explicit null)\n", tagName)
					tagsMap[tagName] = "REMOVED"
				} else if tagValue, ok := tagOptional.Get(); ok {
					// Tag present with a value - should be updated
					fmt.Printf("Tag '%s' will be set to %s\n", tagName, tagValue)
					tagsMap[tagName] = tagValue
				} else {
					// This shouldn't happen with the current model, but added for completeness
					fmt.Printf("Tag '%s' has an unexpected state\n", tagName)
				}
			}
			prefsMap["tags"] = tagsMap
		}

		updates["preferences"] = prefsMap
	} else if request.Preferences.Null {
		updates["preferences"] = "REMOVED"
	}

	// Handle regular non-optional field (score)
	// We can't tell if score was actually provided or just defaulted to zero!
	updates["score"] = request.Score
	fmt.Printf("Score field (non-optional): %d (Can't distinguish between 0 and missing!)\n", request.Score)

	// Log what would happen in the database
	fmt.Printf("Updating user profile\n")
	for field, value := range updates {
		fmt.Printf("  Set %s = %v\n", field, value)
	}

	// Create a success response with the updates that would be applied
	successResponse := wscutils.NewSuccessResponse(response)
	wscutils.SendSuccessResponse(c, successResponse)
}

func main() {
	// Initialize Gin router
	router := gin.Default()

	// Create service with Alya's service abstraction
	s := service.NewService(router)

	// Register the profile update route directly
	s.RegisterRoute("POST", "/profileUpdate", updateProfileHandler)

	// Start the server
	fmt.Println("Server starting on :8081")
	router.Run(":8081")
}
