package webui

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/chatinsights"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleChatInsights(c *gin.Context) {
	groupID, ok := parseOptionalGroupID(c)
	if !ok {
		return
	}
	if groupID <= 0 {
		jsonError(c, http.StatusBadRequest, "group_id required")
		return
	}
	window, ok := chatinsights.ResolvePeriod(c.Query("period"), bot.Now())
	if !ok {
		jsonError(c, http.StatusBadRequest, "period must be today, yesterday, 7days or 30days")
		return
	}
	if window.Period == chatinsights.PeriodWeek {
		jsonError(c, http.StatusBadRequest, "period must be today, yesterday, 7days or 30days")
		return
	}
	insight, err := chatinsights.QueryGroup(s.chats, groupID, window)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":               true,
		"group_id":         groupID,
		"period":           strings.TrimSpace(c.Query("period")),
		"period_label":     window.Label,
		"start_at":         window.Start.Unix(),
		"end_at":           window.End.Unix(),
		"retention_days":   0,
		"retention_policy": "indefinite",
		"summary":          insight.Summary,
		"words":            insight.Words,
		"users":            insight.Users,
	})
}

func (s *Server) handleChatHistoryGet(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	var beforeAt int64
	if raw := strings.TrimSpace(c.Query("before_at")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			jsonError(c, http.StatusBadRequest, "invalid before_at")
			return
		}
		beforeAt = parsed
	}
	stats, err := s.chats.GetGroupChatHistoryStats(groupID, beforeAt)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "group_id": groupID, "before_at": beforeAt, "stats": stats})
}

func (s *Server) handleChatHistoryDelete(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	var req chatHistoryDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "provide either before_at or all")
		return
	}
	now := bot.Now()
	deleteAll := req.All != nil && *req.All
	if req.BeforeAt != nil {
		if req.All != nil || *req.BeforeAt <= 0 || *req.BeforeAt > now.Unix() {
			jsonError(c, http.StatusBadRequest, "before_at must be a past timestamp and cannot be combined with all")
			return
		}
	} else if !deleteAll {
		jsonError(c, http.StatusBadRequest, "provide either before_at or all=true")
		return
	}
	var deleted int64
	var err error
	if deleteAll {
		deleted, err = s.chats.DeleteAllGroupChatMessagesForGroup(groupID, now)
	} else {
		deleted, err = s.chats.DeleteGroupChatMessagesForGroupBefore(groupID, *req.BeforeAt)
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	remaining, err := s.chats.GetGroupChatHistoryStats(groupID, 0)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "group_id": groupID, "deleted": deleted, "remaining": remaining.Total})
}
