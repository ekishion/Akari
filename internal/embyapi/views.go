package embyapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/config"
	"akari-bridge/internal/mapper"
	"akari-bridge/internal/model"
)

type ViewsHandler struct {
	cfg *config.Config
}

func NewViewsHandler(cfg *config.Config) *ViewsHandler {
	return &ViewsHandler{cfg: cfg}
}

// GetUserViews handles GET /emby/Users/:id/Views
func (h *ViewsHandler) GetUserViews(c *gin.Context) {
	views := mapper.CreateVirtualViews(h.cfg.ServerId)

	c.JSON(http.StatusOK, model.QueryResult[model.BaseItemDto]{
		Items:            views,
		TotalRecordCount: len(views),
	})
}
