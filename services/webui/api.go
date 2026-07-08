package webui

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	systemplugin "github.com/Yuelioi/yueling-go/plugins/system"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type groupLister interface {
	GetGroupList() ([]bot.GroupInfo, error)
}

func parseInt64Param(c *gin.Context, name string) (int64, bool) {
	v, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || v <= 0 {
		jsonError(c, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return v, true
}

func parseIntParam(c *gin.Context, name string) (int, bool) {
	v, err := strconv.Atoi(c.Param(name))
	if err != nil || v <= 0 {
		jsonError(c, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return v, true
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	v, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || v == 0 {
		jsonError(c, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return uint(v), true
}

func parseOptionalGroupID(c *gin.Context) (int64, bool) {
	raw := strings.TrimSpace(c.Query("group_id"))
	if raw == "" {
		return 0, true
	}
	groupID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || groupID <= 0 {
		jsonError(c, http.StatusBadRequest, "invalid group_id")
		return 0, false
	}
	return groupID, true
}

func (s *Server) handleGroups(c *gin.Context) {
	lister := s.resolveGroupLister()
	if lister == nil {
		jsonError(c, http.StatusServiceUnavailable, "bot not connected")
		return
	}
	groups, err := lister.GetGroupList()
	if err != nil {
		jsonError(c, http.StatusBadGateway, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "groups": groups})
}

func (s *Server) handlePlugins(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "plugins": systemplugin.Catalog()})
}

func (s *Server) handleGroupPlugins(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	disabled, err := db.GetDisabledPlugins(groupID)
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
	if err := db.SetGroupPluginDisabled(groupID, pluginID, *req.Disabled); err != nil {
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
	if err := db.SetPluginDisabledForGroups(pluginID, req.GroupIDs, *req.Disabled); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func affinityConfig() config.AffinityConfig {
	return ai.NormalizeAffinityConfig(config.C.AI.Affinity)
}

func affinityBounds() (initial, minScore, maxScore int) {
	cfg := affinityConfig()
	return cfg.Initial, cfg.Min, cfg.Max
}

func (s *Server) handleAffinityList(c *gin.Context) {
	groupID, ok := parseOptionalGroupID(c)
	if !ok {
		return
	}
	rows, err := db.ListAIAffinityAdmin(groupID, c.Query("q"), 100)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":          true,
		"affinity":    rows,
		"block_below": affinityConfig().BlockBelow,
	})
}

func (s *Server) handleAffinitySetScore(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Score int `json:"score"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	_, minScore, maxScore := affinityBounds()
	row, err := db.SetAIAffinityScore(id, req.Score, minScore, maxScore, "webui_set")
	if err != nil {
		affinityMutationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "affinity": row})
}

func (s *Server) handleAffinityAdjust(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Delta int `json:"delta"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Delta == 0 {
		jsonError(c, http.StatusBadRequest, "delta required")
		return
	}
	_, minScore, maxScore := affinityBounds()
	row, err := db.AdjustAIAffinityScore(id, req.Delta, minScore, maxScore, "webui_adjust")
	if err != nil {
		affinityMutationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "affinity": row})
}

func (s *Server) handleAffinityReset(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	initial, minScore, maxScore := affinityBounds()
	row, err := db.ResetAIAffinityScore(id, initial, minScore, maxScore, "webui_reset")
	if err != nil {
		affinityMutationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "affinity": row})
}

func affinityMutationError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		jsonError(c, http.StatusNotFound, err.Error())
		return
	}
	jsonError(c, http.StatusInternalServerError, err.Error())
}
