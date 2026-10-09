package webui

import (
	"net/http"

	systemplugin "github.com/Yuelioi/yueling-go/plugins/system"
	"github.com/gin-gonic/gin"
)

func (s *Server) handlePlugins(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "plugins": systemplugin.Catalog()})
}

func (s *Server) handleGroupPlugins(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	disabled, err := s.groups.GetDisabledPlugins(groupID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "disabled": disabled})
}

func (s *Server) handleSetGroupPlugin(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	pluginID, ok := parseIntParam(c, "pluginID")
	if !ok {
		return
	}
	var req struct {
		Disabled *bool `json:"disabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Disabled == nil {
		jsonError(c, http.StatusBadRequest, "disabled required")
		return
	}
	if err := s.groups.SetGroupPluginDisabled(groupID, pluginID, *req.Disabled); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleApplyPluginAll(c *gin.Context) {
	pluginID, ok := parseIntParam(c, "pluginID")
	if !ok {
		return
	}
	var req struct {
		GroupIDs []int64 `json:"group_ids"`
		Disabled *bool   `json:"disabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Disabled == nil {
		jsonError(c, http.StatusBadRequest, "disabled required")
		return
	}
	if len(req.GroupIDs) == 0 {
		jsonError(c, http.StatusBadRequest, "group_ids required")
		return
	}
	for _, groupID := range req.GroupIDs {
		if groupID <= 0 {
			jsonError(c, http.StatusBadRequest, "invalid group_id")
			return
		}
	}
	if err := s.groups.SetPluginDisabledForGroups(pluginID, req.GroupIDs, *req.Disabled); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
