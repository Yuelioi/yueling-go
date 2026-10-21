package webui

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/httpserver"
	"github.com/Yuelioi/yueling-go/scheduler"
	bservice "github.com/Yuelioi/yueling-go/services/bilibili"
	"github.com/Yuelioi/yueling-go/services/feed"
	"github.com/Yuelioi/yueling-go/services/imageassets"
	"github.com/Yuelioi/yueling-go/services/knowledge"
	"github.com/Yuelioi/yueling-go/services/logx"
	"github.com/Yuelioi/yueling-go/services/runtimeconfig"
	"github.com/gin-gonic/gin"
)

type Server struct {
	modelUsage         *db.ModelUsageStore
	images             *imageassets.Library
	usage              *db.CommandUsageStore
	groups             *db.GroupStore
	overview           *db.OverviewStore
	affinity           *db.AffinityStore
	chats              *db.ChatStore
	memory             *ai.MemoryManager
	knowledge          *knowledge.Manager
	schedules          *scheduler.Manager
	feeds              *feed.Manager
	configuration      *runtimeconfig.Manager
	cfg                config.WebUIConfig
	current            atomic.Pointer[bot.BotAPI]
	resolveGroupLister func() groupLister
	resolveGroupSender func() groupMessageSender
	resolveFeedSender  func() feedSender
	sessions           *sessionStore
	loginAttempts      *loginAttemptStore
	bilibili           *bservice.Manager
}

const maxWebUIRequestBytes = 64 << 20

// Dependencies are assembled before serving; request handlers do not resolve global services.
type Dependencies struct {
	ModelUsage    *db.ModelUsageStore
	Images        *imageassets.Library
	Usage         *db.CommandUsageStore
	Groups        *db.GroupStore
	Overview      *db.OverviewStore
	Affinity      *db.AffinityStore
	Chats         *db.ChatStore
	Memory        *ai.MemoryManager
	Knowledge     *knowledge.Manager
	Schedules     *scheduler.Manager
	Feeds         *feed.Manager
	Configuration *runtimeconfig.Manager
	Bilibili      *bservice.Manager
}

func New(cfg config.WebUIConfig, deps Dependencies) *Server {
	gin.SetMode(gin.ReleaseMode)
	s := &Server{modelUsage: deps.ModelUsage,
		images:        deps.Images,
		usage:         deps.Usage,
		groups:        deps.Groups,
		overview:      deps.Overview,
		affinity:      deps.Affinity,
		chats:         deps.Chats,
		memory:        deps.Memory,
		knowledge:     deps.Knowledge,
		schedules:     deps.Schedules,
		feeds:         deps.Feeds,
		configuration: deps.Configuration,
		bilibili:      deps.Bilibili,

		cfg:           cfg,
		sessions:      newSessionStore(),
		loginAttempts: newLoginAttemptStore(),
	}
	s.resolveGroupLister = func() groupLister {
		api := s.current.Load()
		if !api.Connected() {
			return nil
		}
		return api
	}
	s.resolveGroupSender = func() groupMessageSender {
		api := s.current.Load()
		if !api.Connected() {
			return nil
		}
		return api
	}
	s.resolveFeedSender = func() feedSender {
		api := s.current.Load()
		if !api.Connected() {
			return nil
		}
		return api
	}
	return s
}

func (s *Server) BindBot(b *bot.Bot) {
	b.OnConnect(func(api *bot.BotAPI) { s.current.Store(api) })
}

func (s *Server) Handler() http.Handler {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxWebUIRequestBytes)
		c.Next()
	})
	r.Use(gin.CustomRecovery(func(c *gin.Context, rec any) {
		logx.Errorf("[webui] panic: %v", rec)
		jsonError(c, http.StatusInternalServerError, "internal error")
	}))

	api := r.Group("/api/webui")
	api.POST("/auth/login", s.handleLogin)

	protected := api.Group("")
	protected.Use(s.requireSession)
	protected.POST("/auth/logout", s.handleLogout)
	protected.GET("/auth/me", s.handleMe)
	protected.GET("/overview", s.handleOverview)
	protected.GET("/config", s.handleConfigGet)
	protected.GET("/ai/features", s.handleAIFeatures)
	protected.POST("/ai/profiles/:id/test", s.handleAIProfileTest)
	protected.GET("/ai/connections/:id/auth", s.handleCodexStatus)
	protected.PUT("/ai/connections/:id/auth", s.handleCodexImport)
	protected.PUT("/config", s.handleConfigSet)
	protected.GET("/bilibili", s.handleBilibiliGet)
	protected.PUT("/bilibili/settings", s.handleBilibiliSet)
	protected.GET("/groups", s.handleGroups)
	protected.GET("/group-administration", s.handleGroupAdministration)
	protected.POST("/groups/:groupID/messages", s.handleSendGroupMessage)
	protected.GET("/ai-style/default", s.handleDefaultAIStyleGet)
	protected.PUT("/ai-style/default", s.handleDefaultAIStyleSet)
	protected.DELETE("/ai-style/default", s.handleDefaultAIStyleDelete)
	protected.GET("/groups/:groupID/ai-style", s.handleGroupAIStyleGet)
	protected.PUT("/groups/:groupID/ai-style", s.handleGroupAIStyleSet)
	protected.DELETE("/groups/:groupID/ai-style", s.handleGroupAIStyleDelete)
	protected.GET("/join-review/:scope", s.handleJoinReviewGet)
	protected.PUT("/join-review/:scope", s.handleJoinReviewSet)
	protected.GET("/plugins", s.handlePlugins)
	protected.POST("/image-assets/upload", s.handleImageUpload)
	protected.GET("/image-assets/categories", s.handleImageCategories)
	protected.GET("/image-assets", s.handleImageAssets)
	protected.GET("/image-assets/:id/preview", s.handleImagePreview)
	protected.DELETE("/image-assets/:id", s.handleImageDelete)
	protected.POST("/image-assets/:id/rename", s.handleImageRename)
	protected.GET("/ai/model-usage", s.handleModelUsage)
	protected.GET("/command-usage", s.handleCommandUsage)
	protected.GET("/groups/:groupID/command-usage", s.handleGroupCommandUsage)
	protected.GET("/chat-insights", s.handleChatInsights)
	protected.GET("/chat-overview", s.handleChatOverview)
	protected.GET("/groups/:groupID/chat-history", s.handleChatHistoryGet)
	protected.DELETE("/groups/:groupID/chat-history", s.handleChatHistoryDelete)
	protected.GET("/groups/:groupID/plugins", s.handleGroupPlugins)
	protected.PUT("/groups/:groupID/plugins/:pluginID", s.handleSetGroupPlugin)
	protected.POST("/plugins/:pluginID/apply-all", s.handleApplyPluginAll)
	protected.GET("/feeds", s.handleFeedList)
	protected.POST("/feeds/check", s.handleFeedCheckAll)
	protected.POST("/feeds/diagnose", s.handleFeedDiagnose)
	protected.POST("/groups/:groupID/feeds", s.handleFeedAdd)
	protected.POST("/groups/:groupID/feeds/platform", s.handleFeedPlatformAdd)
	protected.GET("/groups/:groupID/feeds/settings", s.handleFeedSettingsGet)
	protected.PUT("/groups/:groupID/feeds/settings", s.handleFeedSettingsSet)
	protected.PUT("/groups/:groupID/feeds/:feedID", s.handleFeedSetEnabled)
	protected.DELETE("/groups/:groupID/feeds/:feedID", s.handleFeedDelete)
	protected.POST("/groups/:groupID/feeds/check", s.handleFeedCheck)
	protected.GET("/knowledge", s.handleKnowledgeList)
	protected.POST("/knowledge/shared", s.handleSharedKnowledgeAdd)
	protected.PUT("/knowledge/shared/:knowledgeID/shortcuts", s.handleSharedKnowledgeShortcutsSet)
	protected.DELETE("/knowledge/shared/:knowledgeID", s.handleSharedKnowledgeDelete)
	protected.GET("/knowledge/shared/search", s.handleSharedKnowledgeSearch)
	protected.POST("/groups/:groupID/knowledge", s.handleKnowledgeAdd)
	protected.PUT("/groups/:groupID/knowledge/:knowledgeID/shortcuts", s.handleKnowledgeShortcutsSet)
	protected.DELETE("/groups/:groupID/knowledge/:knowledgeID", s.handleKnowledgeDelete)
	protected.GET("/groups/:groupID/knowledge/search", s.handleKnowledgeSearch)
	protected.GET("/affinity", s.handleAffinityList)
	protected.GET("/affinity/settings", s.handleAffinitySettingsGet)
	protected.PUT("/affinity/settings", s.handleAffinitySettingsSet)
	protected.DELETE("/affinity/settings", s.handleAffinitySettingsDelete)
	protected.PUT("/affinity/:id/score", s.handleAffinitySetScore)
	protected.POST("/affinity/:id/adjust", s.handleAffinityAdjust)
	protected.POST("/affinity/:id/reset", s.handleAffinityReset)
	protected.GET("/memories", s.handleMemoryList)
	protected.DELETE("/memories/:id", s.handleMemoryDelete)
	protected.DELETE("/memories/users/:userID", s.handleUserMemoriesClear)
	protected.GET("/digests", s.handleDigestList)
	protected.PUT("/groups/:groupID/digest", s.handleDigestSet)
	protected.DELETE("/groups/:groupID/digest", s.handleDigestDelete)

	s.mountStatic(r)
	return r
}

func (s *Server) Run(ctx context.Context, addr string) error {
	logx.Infof("[webui] serving on %s", addr)
	server := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Minute,
		IdleTimeout:       90 * time.Second,
	}
	return httpserver.Run(ctx, server)
}
