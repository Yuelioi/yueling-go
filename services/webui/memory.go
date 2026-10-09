package webui

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (s *Server) handleMemoryList(c *gin.Context) {
	rows, err := s.memory.ListSemanticMemoriesAdmin(c.Query("q"), 200)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "memories": rows})
}

func (s *Server) handleMemoryDelete(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	row, err := s.memory.GetSemanticMemoryAdmin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			jsonError(c, http.StatusNotFound, "memory not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	deleted, err := s.memory.DeleteSemanticMemory(row.UserID, id)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !deleted {
		jsonError(c, http.StatusNotFound, "memory not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleUserMemoriesClear(c *gin.Context) {
	userID, ok := parseInt64Param(c, "userID")
	if !ok {
		return
	}
	count, err := s.memory.ClearSemanticMemories(userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": count})
}
