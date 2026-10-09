package webui

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/db"
	systemplugin "github.com/Yuelioi/yueling-go/plugins/system"
	"github.com/gin-gonic/gin"
)

func commandUsageDays(c *gin.Context) (int, bool) {
	days := 7
	if raw := strings.TrimSpace(c.Query("days")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > db.MaxCommandUsageDays {
			jsonError(c, http.StatusBadRequest, "days must be between 1 and 90")
			return 0, false
		}
		days = parsed
	}
	return days, true
}

func (s *Server) handleCommandUsage(c *gin.Context) {
	groupID := int64(0)
	if raw := strings.TrimSpace(c.Query("group_id")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			jsonError(c, http.StatusBadRequest, "invalid group_id")
			return
		}
		groupID = parsed
	}
	s.handleCommandUsageForScope(c, groupID)
}

func (s *Server) handleGroupCommandUsage(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	s.handleCommandUsageForScope(c, groupID)
}

func (s *Server) handleCommandUsageForScope(c *gin.Context, groupID int64) {
	days, ok := commandUsageDays(c)
	if !ok {
		return
	}
	stats, err := s.usage.GetGroupCommandUsageStats(groupID, days, time.Now())
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	pluginNames := make(map[int]string)
	for _, plugin := range systemplugin.Catalog() {
		pluginNames[plugin.ID] = plugin.Name
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":           true,
		"stats":        stats,
		"plugin_names": pluginNames,
	})
}
