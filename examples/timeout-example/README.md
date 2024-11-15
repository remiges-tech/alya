This example demonstrates how Alya handles request timeouts using middleware.

It shows two endpoints:
1. /quick - Returns immediately to demonstrate normal request flow
2. /slow - Takes 3 minutes to process, triggering a 2-minute timeout configured in the middleware.

The example shows:
- How to configure timeout middleware
- How timeout errors are handled with custom message IDs and error codes
- How to use LogHarbour to track request timing

To test:
1. Quick endpoint: curl http://localhost:8080/quick
   - Returns immediately with success response
2. Slow endpoint: curl http://localhost:8080/slow
   - Times out after 2 minutes
   - Returns error response with msgID: 5001, errCode: "request_timeout"
