package webui

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Yuelioi/yueling-go/db"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (s *Server) handleKnowledgeList(c *gin.Context) {
	raw := strings.TrimSpace(c.Query("group_id"))
	if raw == "" {
		jsonError(c, http.StatusBadRequest, "group_id required")
		return
	}
	groupID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || groupID < db.SharedKnowledgeGroupID {
		jsonError(c, http.StatusBadRequest, "invalid group_id")
		return
	}
	rows, err := s.knowledge.List(groupID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "knowledge": rows})
}

func (s *Server) handleKnowledgeAdd(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	s.handleKnowledgeAddForScope(c, groupID)
}

func (s *Server) handleSharedKnowledgeAdd(c *gin.Context) {
	s.handleKnowledgeAddForScope(c, db.SharedKnowledgeGroupID)
}

func (s *Server) handleKnowledgeAddForScope(c *gin.Context, groupID int64) {
	var req struct {
		Title     string   `json:"title"`
		Content   string   `json:"content"`
		URL       string   `json:"url"`
		Shortcuts []string `json:"shortcuts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	var row *db.GroupKnowledge
	var err error
	if strings.TrimSpace(req.URL) != "" {
		row, err = s.knowledge.AddURLWithShortcuts(c.Request.Context(), groupID, 0, req.Title, req.URL, req.Shortcuts)
	} else {
		row, err = s.knowledge.AddTextWithShortcuts(groupID, 0, req.Title, req.Content, req.Shortcuts)
	}
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "knowledge": row})
}

func (s *Server) handleKnowledgeShortcutsSet(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	s.handleKnowledgeShortcutsSetForScope(c, groupID)
}

func (s *Server) handleSharedKnowledgeShortcutsSet(c *gin.Context) {
	s.handleKnowledgeShortcutsSetForScope(c, db.SharedKnowledgeGroupID)
}

func (s *Server) handleKnowledgeShortcutsSetForScope(c *gin.Context, groupID int64) {
	id, ok := parseUintParam(c, "knowledgeID")
	if !ok {
		return
	}
	var req struct {
		Shortcuts []string `json:"shortcuts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	shortcuts, err := s.knowledge.SetShortcuts(id, groupID, req.Shortcuts)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			jsonError(c, http.StatusNotFound, "knowledge not found")
			return
		}
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "shortcuts": shortcuts})
}

func (s *Server) handleKnowledgeDelete(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	s.handleKnowledgeDeleteForScope(c, groupID)
}

func (s *Server) handleSharedKnowledgeDelete(c *gin.Context) {
	s.handleKnowledgeDeleteForScope(c, db.SharedKnowledgeGroupID)
}

func (s *Server) handleKnowledgeDeleteForScope(c *gin.Context, groupID int64) {
	id, ok := parseUintParam(c, "knowledgeID")
	if !ok {
		return
	}
	if err := s.knowledge.Remove(id, groupID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			jsonError(c, http.StatusNotFound, "knowledge not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleKnowledgeSearch(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	s.handleKnowledgeSearchForScope(c, groupID)
}

func (s *Server) handleSharedKnowledgeSearch(c *gin.Context) {
	s.handleKnowledgeSearchForScope(c, db.SharedKnowledgeGroupID)
}

func (s *Server) handleKnowledgeSearchForScope(c *gin.Context, groupID int64) {
	rows, err := s.knowledge.Search(groupID, c.Query("q"), 5)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "knowledge": rows})
}
