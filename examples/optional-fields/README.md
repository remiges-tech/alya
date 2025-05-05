# Alya Service with Optional Fields Example

This example demonstrates how to use the `Optional[T]` type from Alya's wscutils package to properly handle optional fields in JSON requests.

## Overview

This example implements a simple user profile web service that allows updating a user profile using POST (`POST /profileUpdate`)

The key feature is the use of `Optional[T]` in the update endpoint to properly distinguish between:
- Fields the client wants to update (present in the JSON)
- Fields the client explicitly wants to remove (present as `null` in the JSON)
- Fields the client doesn't want to modify (omitted from the JSON)

## Running the Example

Navigate to the example directory and run:
   ```bash
   cd examples/optional-fields
   go run main.go
   ```

The server will start on port 8081

## Implementation Details

1. **Define the data model with Optional fields**: We define structs that use `Optional[T]` for fields that need to distinguish between present, null, and missing values:
   ```go
   type Preferences struct {
       Theme         wscutils.Optional[string]           `json:"theme"`
       Tags          map[string]wscutils.Optional[string] `json:"tags"`
   }

   type UserProfile struct {
       Username    string                         `json:"username"`
       Email       wscutils.Optional[string]      `json:"email"`
       IsActive    wscutils.Optional[bool]        `json:"isActive"`
       Preferences wscutils.Optional[Preferences] `json:"preferences"`
       Score       int                            `json:"score"` // Regular non-optional field
   }
   ```

2. **Set up the web service**: We use Alya's service abstraction to create a simple web service:
   ```go
   // Create service with minimal configuration
   s := service.NewService(router)
   
   // Register route directly without groups
   s.RegisterRoute("POST", "/profileUpdate", updateProfileHandler)
   ```

3. **Implement the handler function**: The handler processes JSON requests and prepares a response:
   ```go
   func updateProfileHandler(c *gin.Context, s *service.Service) {
       var request UserProfile
       
       // Use wscutils to bind the request
       wscutils.BindJSON(c, &request)
       
       // Create a response with updates that would be applied
       response := make(map[string]any)
       response["username"] = request.Username // Always include username
       response["updates"] = make(map[string]any)
       
       // Process fields...
       
       // Send response
       successResponse := wscutils.NewSuccessResponse(response)
       wscutils.SendSuccessResponse(c, successResponse)
   }
   ```

4. **Handle regular Optional fields**: For basic fields, we only update when a value is present:
   ```go
   if email, ok := request.Email.Get(); ok {
       // Email was provided with a value - update it
       updates["email"] = email
   }
   // If not present or null, email field remains unchanged
   
   // Same pattern applies to other regular Optional fields
   if isActive, ok := request.IsActive.Get(); ok {
       // IsActive was provided with a value - update it
       updates["isActive"] = isActive
   }
   ```

5. **Special handling for tags**: For tags, we implement behavior where null means removal:
   ```go
   if prefs, ok := request.Preferences.Get(); ok {
       prefsMap := make(map[string]interface{})
       
       // Process theme field - only update if value is present
       if theme, themeOk := prefs.Theme.Get(); themeOk {
           prefsMap["theme"] = theme
       }
       
       // Handle tags with special null=remove semantics
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
               }
           }
           prefsMap["tags"] = tagsMap
       }
       
       updates["preferences"] = prefsMap
   }
   ```

6. **Limitation of regular fields**: The example includes a non-optional field to demonstrate why `Optional[T]` is valuable:
   ```go
   // Handle regular non-optional field (score)
   // We can't tell if score was actually provided or just defaulted to zero!
   updates["score"] = request.Score
   fmt.Printf("Score field (non-optional): %d (Can't distinguish between 0 and missing!)\n", request.Score)
   ```
   
   This shows why `Optional[T]` is valuable - with regular fields, you can't tell if a zero value was explicitly provided or if the field was missing from the request entirely.

## Testing the API

You can use curl to test the API:

### Update a user profile with specific fields

```bash
curl -X POST http://localhost:8081/profileUpdate \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "username": "johndoe",
      "email": "john@example.com",
      "isActive": true,
      "score": 100
    }
  }'
```

Preferences is omitted from the request, so it is not updated

### Update a user profile with tags (including null for removal)

```bash
curl -X POST http://localhost:8081/profileUpdate \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "username": "johndoe",
      "score": 100,
      "preferences": {
        "tags": {
          "location": null,
          "car": "Swift"
        }
      }
    }
  }'
```

This example demonstrates how `Optional[T]` handles the three distinct states in a simple tags update:

- `location: null` - The server **removes** this tag (explicit null is used for removal only with tags)
- Any unmentioned tags - The server **preserves** their existing values (omitted fields)
- `car: "Swift"` - The server **updates** this tag (provided value)

### Update with missing score (showing limitation of regular fields)

```bash
curl -X POST http://localhost:8081/profileUpdate \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "username": "johndoe",
      "email": "new@example.com"
    }
  }'
```

This example demonstrates a key limitation of regular fields like `score`: When not provided in the JSON, they are initialized to their zero value (`0` for integers). Unlike `Optional` fields, the server cannot distinguish whether `score` was intentionally set to `0` or simply not included in the request.

