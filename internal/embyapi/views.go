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
	c.JSON(http.StatusOK, model.NewQueryResult(views))
}

// GetMediaFolders handles GET /emby/Library/MediaFolders and /Library/MediaFolders
func (h *ViewsHandler) GetMediaFolders(c *gin.Context) {
	folders := mapper.CreateCollectionFolders(h.cfg.ServerId)
	c.JSON(http.StatusOK, model.NewQueryResult(folders))
}

// GetGroupingOptions handles GET /Users/:id/GroupingOptions
func (h *ViewsHandler) GetGroupingOptions(c *gin.Context) {
	c.JSON(http.StatusOK, []any{})
}

// GetRootFolder handles GET /Items/Root and /Users/:id/Items/Root
func (h *ViewsHandler) GetRootFolder(c *gin.Context) {
	c.JSON(http.StatusOK, model.BaseItemDto{
		Name:                     "Root",
		ServerId:                 h.cfg.ServerId,
		Id:                       "root",
		Guid:                     "root",
		Type:                     "AggregateFolder",
		IsFolder:                 true,
		LocationType:             "FileSystem",
		Path:                     "/media",
		SortName:                 "Root",
		EnableMediaSourceDisplay: true,
	})
}

