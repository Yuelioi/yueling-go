package webui

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) groupAIStyleResponse(groupID int64) (gin.H, error) {
	prompt, custom, err := s.groups.GetGroupAIStylePrompt(groupID)
	if err != nil {
		return nil, err
	}
	defaultPrompt := ai.DefaultGroupStylePrompt
	if groupID != db.DefaultAIStyleGroupID {
		configuredDefault, configured, defaultErr := s.groups.GetGroupAIStylePrompt(db.DefaultAIStyleGroupID)
		if defaultErr != nil {
			return nil, defaultErr
		}
		if configured && strings.TrimSpace(configuredDefault) != "" {
			defaultPrompt = strings.TrimSpace(configuredDefault)
		}
	}
	return gin.H{
		"ok":                   true,
		"group_id":             groupID,
		"style_prompt":         prompt,
		"custom":               custom,
		"default_scope":        groupID == db.DefaultAIStyleGroupID,
		"default_style_prompt": defaultPrompt,
		"max_chars":            db.MaxGroupAIStylePromptChars,
	}, nil
}

func (s *Server) handleDefaultAIStyleGet(c *gin.Context) {
	payload, err := s.groupAIStyleResponse(db.DefaultAIStyleGroupID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (s *Server) handleGroupAIStyleGet(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	payload, err := s.groupAIStyleResponse(groupID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (s *Server) handleGroupAIStyleSet(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	s.handleAIStyleSetForScope(c, groupID)
}

func (s *Server) handleDefaultAIStyleSet(c *gin.Context) {
	s.handleAIStyleSetForScope(c, db.DefaultAIStyleGroupID)
}

func (s *Server) handleAIStyleSetForScope(c *gin.Context, groupID int64) {
	var req struct {
		StylePrompt *string `json:"style_prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.StylePrompt == nil {
		jsonError(c, http.StatusBadRequest, "style_prompt required")
		return
	}
	if strings.TrimSpace(*req.StylePrompt) == "" {
		if err := s.groups.DeleteGroupAIStylePrompt(groupID); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
	} else if _, err := s.groups.SetGroupAIStylePrompt(groupID, *req.StylePrompt); err != nil {
		if errors.Is(err, db.ErrGroupAIStylePromptTooLong) {
			jsonError(c, http.StatusBadRequest, "style_prompt exceeds 4000 characters")
			return
		}
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	payload, err := s.groupAIStyleResponse(groupID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (s *Server) handleGroupAIStyleDelete(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	s.handleAIStyleDeleteForScope(c, groupID)
}

func (s *Server) handleDefaultAIStyleDelete(c *gin.Context) {
	s.handleAIStyleDeleteForScope(c, db.DefaultAIStyleGroupID)
}

func (s *Server) handleAIStyleDeleteForScope(c *gin.Context, groupID int64) {
	if err := s.groups.DeleteGroupAIStylePrompt(groupID); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	payload, err := s.groupAIStyleResponse(groupID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, payload)
}
