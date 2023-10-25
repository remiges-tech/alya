package rigel

import (
	"github.com/gin-gonic/gin"
	"go-framework/internal/pg/sqlc-gen"
	"go-framework/logharbour"
)

type RigelHandler struct {
	sqlq *sqlc.Queries
	lh   *logharbour.LogHarbour
}

func NewHandler(sqlq *sqlc.Queries, lh *logharbour.LogHarbour) *RigelHandler {
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
