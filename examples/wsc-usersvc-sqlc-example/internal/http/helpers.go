package transport

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/wscutils"
)

func int64ToString(value int64) string {
	return strconv.FormatInt(value, 10)
}

func sendSuccess(c *gin.Context, data any) {
	wscutils.SendSuccessResponse(c, wscutils.NewSuccessResponse(data))
}

func sendError(c *gin.Context, status int, messages []wscutils.ErrorMessage) {
	c.JSON(status, wscutils.NewResponse(wscutils.ErrorStatus, nil, messages))
}
