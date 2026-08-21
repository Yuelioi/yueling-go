package webui

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	systemplugin "github.com/Yuelioi/yueling-go/plugins/system"
	"github.com/Yuelioi/yueling-go/scheduler"
	"github.com/Yuelioi/yueling-go/services/feed"
	knowledgeservice "github.com/Yuelioi/yueling-go/services/knowledge"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type groupLister interface {
	GetGroupList() ([]bot.GroupInfo, error)
}

type groupMessageSender interface {
	SendGroupMsg(groupID int64, msg bot.Message) (int32, error)
}

type feedSender interface {
	SendGroupText(groupID int64, text string) error
}

type groupMessageRequest struct {
	Text      string   `json:"text"`
	AtUserIDs []int64  `json:"at_user_ids"`
	Images    []string `json:"images"`
}

func buildGroupMessage(req groupMessageRequest) (bot.Message, error) {
	msg := bot.Msg()
	hasSegment := false

	text := strings.TrimSpace(req.Text)
	if text != "" {
		msg.Text(text)
		hasSegment = true
	}

	for _, userID := range req.AtUserIDs {
		if userID <= 0 {
			return nil, errors.New("invalid at_user_id")
		}
		msg.At(userID)
		hasSegment = true
	}

	for _, image := range req.Images {
		image = strings.TrimSpace(image)
		if image == "" {
			return nil, errors.New("image required")
		}
		msg.Image(image)
		hasSegment = true
	}

	if !hasSegment {
		return nil, errors.New("message required")
	}
	return msg.Build(), nil
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

func groupAIStyleResponse(groupID int64) (gin.H, error) {
	prompt, custom, err := db.GetGroupAIStylePrompt(groupID)
	if err != nil {
		return nil, err
	}
	defaultPrompt := ai.DefaultGroupStylePrompt
	if groupID != db.DefaultAIStyleGroupID {
		configuredDefault, configured, defaultErr := db.GetGroupAIStylePrompt(db.DefaultAIStyleGroupID)
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
	payload, err := groupAIStyleResponse(db.DefaultAIStyleGroupID)
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
	payload, err := groupAIStyleResponse(groupID)
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
		if err := db.DeleteGroupAIStylePrompt(groupID); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
	} else if _, err := db.SetGroupAIStylePrompt(groupID, *req.StylePrompt); err != nil {
		if errors.Is(err, db.ErrGroupAIStylePromptTooLong) {
			jsonError(c, http.StatusBadRequest, "style_prompt exceeds 4000 characters")
			return
		}
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	payload, err := groupAIStyleResponse(groupID)
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
	if err := db.DeleteGroupAIStylePrompt(groupID); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	payload, err := groupAIStyleResponse(groupID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (s *Server) handleOverview(c *gin.Context) {
	overview, err := db.GetWebUIOverview(affinityConfig().BlockBelow)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	groupCount := 0
	botConnected := false
	if lister := s.resolveGroupLister(); lister != nil {
		botConnected = true
		if groups, listErr := lister.GetGroupList(); listErr == nil {
			groupCount = len(groups)
		}
	}
	recentAffinity, err := db.ListAIAffinityAdmin(0, "", 5)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	recentMemories, err := db.ListSemanticMemoriesAdmin("", 5)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":                 true,
		"bot_connected":      botConnected,
		"group_count":        groupCount,
		"plugin_count":       len(systemplugin.Catalog()),
		"affinity_count":     overview.AffinityCount,
		"low_affinity_count": overview.LowAffinityCount,
		"memory_count":       overview.MemoryCount,
		"memory_user_count":  overview.MemoryUserCount,
		"digest_count":       overview.DigestCount,
		"feed_count":         overview.FeedCount,
		"knowledge_count":    overview.KnowledgeCount,
		"recent_affinity":    recentAffinity,
		"recent_memories":    recentMemories,
	})
}

func (s *Server) handleFeedList(c *gin.Context) {
	rows, err := db.ListAllFeedSubscriptions()
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
	row, parsed, err := feed.DefaultManager.Add(groupID, 0, req.URL, req.Name)
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
	row, parsed, err := feed.DefaultManager.AddPlatform(groupID, 0, req.Platform, req.Target, req.Name)
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
	setting, pending, err := feed.DefaultManager.DeliveryStatus(groupID)
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
		QuietEnabled bool   `json:"quiet_enabled"`
		QuietStart   string `json:"quiet_start"`
		QuietEnd     string `json:"quiet_end"`
		ItemMaxChars *int   `json:"item_max_chars"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	itemMaxChars := 0
	if req.ItemMaxChars == nil {
		current, _, err := feed.DefaultManager.DeliveryStatus(groupID)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		itemMaxChars = current.ItemMaxChars
	} else {
		itemMaxChars = *req.ItemMaxChars
	}
	setting, err := feed.DefaultManager.SetDeliverySettings(
		groupID, req.QuietEnabled, req.QuietStart, req.QuietEnd, itemMaxChars,
	)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	pending, err := db.CountFeedPendingItems(groupID)
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
	if err := feed.DefaultManager.Remove(feedID, groupID); err != nil {
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
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		jsonError(c, http.StatusBadRequest, "enabled required")
		return
	}
	row, err := feed.DefaultManager.SetEnabled(feedID, groupID, *req.Enabled)
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
	result, err := feed.DefaultManager.CheckGroup(sender, groupID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "result": result})
}

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
	rows, err := knowledgeservice.List(groupID)
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
		row, err = knowledgeservice.AddURLWithShortcuts(groupID, 0, req.Title, req.URL, req.Shortcuts)
	} else {
		row, err = knowledgeservice.AddTextWithShortcuts(groupID, 0, req.Title, req.Content, req.Shortcuts)
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
	shortcuts, err := knowledgeservice.SetShortcuts(id, groupID, req.Shortcuts)
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
	if err := knowledgeservice.Remove(id, groupID); err != nil {
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
	rows, err := knowledgeservice.Search(groupID, c.Query("q"), 5)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "knowledge": rows})
}

func (s *Server) handleSendGroupMessage(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	var req groupMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	msg, err := buildGroupMessage(req)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	sender := s.resolveGroupSender()
	if sender == nil {
		jsonError(c, http.StatusServiceUnavailable, "bot not connected")
		return
	}
	messageID, err := sender.SendGroupMsg(groupID, msg)
	if err != nil {
		jsonError(c, http.StatusBadGateway, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message_id": messageID})
}

func (s *Server) handlePlugins(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "plugins": systemplugin.Catalog()})
}

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
	stats, err := db.GetGroupCommandUsageStats(groupID, days, time.Now())
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

func affinitySettingsResponse() (gin.H, error) {
	cfg := affinityConfig()
	tiers, err := db.ListAIAffinityTiers()
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
	payload, err := affinitySettingsResponse()
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
	if err := db.ReplaceAIAffinityTiers(tiers); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	payload, err := affinitySettingsResponse()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (s *Server) handleAffinitySettingsDelete(c *gin.Context) {
	if err := db.ReplaceAIAffinityTiers(nil); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	payload, err := affinitySettingsResponse()
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

func (s *Server) handleMemoryList(c *gin.Context) {
	rows, err := db.ListSemanticMemoriesAdmin(c.Query("q"), 200)
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
	row, err := db.GetSemanticMemoryAdmin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			jsonError(c, http.StatusNotFound, "memory not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	deleted, err := ai.DeleteSemanticMemory(row.UserID, id)
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
	count, err := ai.ClearSemanticMemories(userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": count})
}

func (s *Server) handleDigestList(c *gin.Context) {
	rows, err := db.GetActiveDailyDigests()
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
	row, err := scheduler.SetDailyDigest(api, groupID, 0, req.SendTime, req.MessageCount)
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
	if err := scheduler.RemoveDailyDigest(groupID); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
