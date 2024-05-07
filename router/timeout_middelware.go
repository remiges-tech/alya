package router

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/wscutils"
)

type MiddlewareErrorScenario string

const (
	RequestTimeout MiddlewareErrorScenario = "RequestTimeout"
)

var middlewareScenarioToMsgID = make(map[MiddlewareErrorScenario]int)
var middlewareScenarioToErrCode = make(map[MiddlewareErrorScenario]string)

func RegisterMiddlewareMsgID(scenario MiddlewareErrorScenario, msgID int) {
	middlewareScenarioToMsgID[scenario] = msgID
}

func RegisterMiddlewareErrCode(scenario MiddlewareErrorScenario, errCode string) {
	middlewareScenarioToErrCode[scenario] = errCode
}

func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		finCh := make(chan struct{}, 1)

		go func() {
			c.Next()
			finCh <- struct{}{}
		}()

		select {
		case <-ctx.Done():
			msgID, ok := middlewareScenarioToMsgID[RequestTimeout]
			if !ok {
				msgID = defaultMsgID
			}
			errCode, ok := middlewareScenarioToErrCode[RequestTimeout]
			if !ok {
				errCode = defaultErrCode
			}
			c.AbortWithStatusJSON(http.StatusGatewayTimeout, wscutils.NewErrorResponse(msgID, errCode))
		case <-finCh:
		}
	}
}
