package webui

import (
	"errors"
	"net/http"

	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/services/feed"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (s *Server) handleFeedList(c *gin.Context) {
	rows, err := s.feeds.ListAll()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "feeds": rows})
}

func (s *Server) handleFeedAdd(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	var req struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	row, parsed, err := s.feeds.Add(groupID, 0, req.URL, req.Name)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	latest := ""
	if len(parsed.Items) > 0 {
		latest = parsed.Items[0].Title
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "feed": row, "latest_title": latest})
}

func (s *Server) handleFeedPlatformAdd(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	var req struct {
		Platform feed.PlatformKind `json:"platform"`
		Target   string            `json:"target"`
		Name     string            `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	row, parsed, err := s.feeds.AddPlatform(groupID, 0, req.Platform, req.Target, req.Name)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	latest := ""
	if len(parsed.Items) > 0 {
		latest = parsed.Items[0].Title
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "feed": row, "latest_title": latest})
}

func (s *Server) handleFeedSettingsGet(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	setting, pending, err := s.feeds.DeliveryStatus(groupID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "settings": setting, "pending_count": pending})
}

func (s *Server) handleFeedSettingsSet(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	var req struct {
		PollIntervalMinutes *int   `json:"poll_interval_minutes"`
		QuietEnabled        bool   `json:"quiet_enabled"`
		QuietStart          string `json:"quiet_start"`
		QuietEnd            string `json:"quiet_end"`
		ItemMaxChars        *int   `json:"item_max_chars"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	current, _, err := s.feeds.DeliveryStatus(groupID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	itemMaxChars := current.ItemMaxChars
	if req.ItemMaxChars != nil {
		itemMaxChars = *req.ItemMaxChars
	}
	minutes := current.PollIntervalMinutes
	if req.PollIntervalMinutes != nil {
		minutes = *req.PollIntervalMinutes
	}
	setting, err := s.feeds.SetDeliverySettings(
		groupID, req.QuietEnabled, req.QuietStart, req.QuietEnd, itemMaxChars, minutes,
	)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	pending, err := s.feeds.PendingCount(groupID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "settings": setting, "pending_count": pending})
}

func (s *Server) handleFeedDelete(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	feedID, ok := parseUintParam(c, "feedID")
	if !ok {
		return
	}
	if err := s.feeds.Remove(feedID, groupID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			jsonError(c, http.StatusNotFound, "feed not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleFeedSetEnabled(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	feedID, ok := parseUintParam(c, "feedID")
	if !ok {
		return
	}
	var req struct {
		Enabled            *bool `json:"enabled"`
		TranslateToChinese *bool `json:"translate_to_chinese"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Enabled == nil && req.TranslateToChinese == nil) || (req.Enabled != nil && req.TranslateToChinese != nil) {
		jsonError(c, http.StatusBadRequest, "exactly one feed setting required")
		return
	}
	var row *db.FeedSubscription
	var err error
	if req.Enabled != nil {
		row, err = s.feeds.SetEnabled(feedID, groupID, *req.Enabled)
	} else {
		row, err = s.feeds.SetTranslation(feedID, groupID, *req.TranslateToChinese)
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			jsonError(c, http.StatusNotFound, "feed not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "feed": row})
}

func (s *Server) handleFeedCheckAll(c *gin.Context) {
	sender := s.resolveFeedSender()
	if sender == nil {
		jsonError(c, http.StatusServiceUnavailable, "bot not connected")
		return
	}
	result, err := s.feeds.CheckAll(sender)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "result": result})
}

func (s *Server) handleFeedCheck(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	sender := s.resolveFeedSender()
	if sender == nil {
		jsonError(c, http.StatusServiceUnavailable, "bot not connected")
		return
	}
	result, err := s.feeds.CheckGroup(sender, groupID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "result": result})
}
