package restapi

import (
	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/cmd/wg-portal/common"
	"github.com/h44z/wg-portal/internal/core"
)

type Handler struct {
	config *common.Config

	backend core.Backend
}

func NewHandler(config *common.Config, backend core.Backend) (*Handler, error) {
	h := &Handler{
		config:  config,
		backend: backend,
	}
	return h, nil
}

func (h *Handler) RegisterRoutes(g *gin.Engine) {

}
