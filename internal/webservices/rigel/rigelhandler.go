package rigel

import (
	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/logharbour/logHarbour"
	"go-framework/internal/pg/sqlc-gen"
)

type RigelHandler struct {
	sqlq *sqlc.Queries
	lh   logHarbour.LogHandles
}

func NewHandler(sqlq *sqlc.Queries, lh logHarbour.LogHandles) *RigelHandler {
	return &RigelHandler{
		sqlq: sqlq,
		lh:   lh,
	}
}

func (h *RigelHandler) RegisterHandlers(router *gin.Engine) {
	router.POST("/rigel/schema", h.createSchema)
	router.POST("/rigel/config", h.createConfig)
	router.GET("/rigel/config", h.getConfig)
}
