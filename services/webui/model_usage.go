package webui

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func (s *Server) handleModelUsage(c *gin.Context) {
	days, ok := commandUsageDays(c)
	if !ok {
		return
	}
	var groupID *int64
	if raw, exists := c.GetQuery("group_id"); exists {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 0 {
			jsonError(c, http.StatusBadRequest, "invalid group_id")
			return
		}
		groupID = &value
	}
	rows, err := s.modelUsage.Stats(c.Request.Context(), groupID, days)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "无法读取模型用量")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "days": days, "stats": rows})
}
