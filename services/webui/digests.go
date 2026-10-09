package webui

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleDigestList(c *gin.Context) {
	rows, err := s.schedules.Digests()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "digests": rows})
}

func (s *Server) handleDigestSet(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	var req struct {
		SendTime     string `json:"send_time"`
		MessageCount int    `json:"message_count"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	req.SendTime = strings.TrimSpace(req.SendTime)
	if req.SendTime == "" {
		jsonError(c, http.StatusBadRequest, "send_time required")
		return
	}
	if req.MessageCount < 10 || req.MessageCount > 100 {
		jsonError(c, http.StatusBadRequest, "message_count must be between 10 and 100")
		return
	}
	api := s.current.Load()
	if api == nil {
		jsonError(c, http.StatusServiceUnavailable, "bot not connected")
		return
	}
	row, err := s.schedules.SetDailyDigest(api, groupID, 0, req.SendTime, req.MessageCount)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "digest": row})
}

func (s *Server) handleDigestDelete(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	if err := s.schedules.RemoveDailyDigest(groupID); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
