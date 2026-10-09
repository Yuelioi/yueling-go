package webui

import (
	"net/http"

	systemplugin "github.com/Yuelioi/yueling-go/plugins/system"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleOverview(c *gin.Context) {
	overview, err := s.overview.GetWebUIOverview(affinityConfig().BlockBelow)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	groupCount := 0
	botConnected := false
	if lister := s.resolveGroupLister(); lister != nil {
		if groups, listErr := lister.GetGroupList(); listErr == nil {
			botConnected = true
			groupCount = len(groups)
		}
	}
	recentAffinity, err := s.affinity.ListAIAffinityAdmin(0, "", 5)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	recentMemories, err := s.memory.ListSemanticMemoriesAdmin("", 5)
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
