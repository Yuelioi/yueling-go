package webui

import (
	"errors"
	"net/http"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func affinityConfig() config.AffinityConfig {
	return ai.NormalizeAffinityConfig(config.Current().AI.Affinity)
}

func affinityBounds() (initial, minScore, maxScore int) {
	cfg := affinityConfig()
	return cfg.Initial, cfg.Min, cfg.Max
}

func (s *Server) affinitySettingsResponse() (gin.H, error) {
	cfg := affinityConfig()
	tiers, err := s.affinity.ListAIAffinityTiers()
	if err != nil {
		return nil, err
	}
	custom := len(tiers) > 0
	if !custom {
		tiers = ai.DefaultAffinityTiers(cfg)
	}
	return gin.H{
		"ok":               true,
		"custom":           custom,
		"tiers":            tiers,
		"initial":          cfg.Initial,
		"min_score":        cfg.Min,
		"max_score":        cfg.Max,
		"block_below":      cfg.BlockBelow,
		"max_tiers":        ai.MaxAffinityTiers,
		"max_name_chars":   ai.MaxAffinityTierName,
		"max_prompt_chars": ai.MaxAffinityPromptChars,
	}, nil
}

func (s *Server) handleAffinitySettingsGet(c *gin.Context) {
	payload, err := s.affinitySettingsResponse()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (s *Server) handleAffinitySettingsSet(c *gin.Context) {
	var req struct {
		Tiers *[]db.AIAffinityTier `json:"tiers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Tiers == nil {
		jsonError(c, http.StatusBadRequest, "tiers required")
		return
	}
	tiers, err := ai.ValidateAffinityTiers(affinityConfig(), *req.Tiers)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.affinity.ReplaceAIAffinityTiers(tiers); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	payload, err := s.affinitySettingsResponse()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (s *Server) handleAffinitySettingsDelete(c *gin.Context) {
	if err := s.affinity.ReplaceAIAffinityTiers(nil); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	payload, err := s.affinitySettingsResponse()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (s *Server) handleAffinityList(c *gin.Context) {
	groupID, ok := parseOptionalGroupID(c)
	if !ok {
		return
	}
	rows, err := s.affinity.ListAIAffinityAdmin(groupID, c.Query("q"), 100)
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
	row, err := s.affinity.SetAIAffinityScore(id, req.Score, minScore, maxScore, "webui_set")
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
	row, err := s.affinity.AdjustAIAffinityScore(id, req.Delta, minScore, maxScore, "webui_adjust")
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
	row, err := s.affinity.ResetAIAffinityScore(id, initial, minScore, maxScore, "webui_reset")
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
