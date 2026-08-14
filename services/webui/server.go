package webui

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/logx"
	"github.com/gin-gonic/gin"
)

type Server struct {
	cfg                config.WebUIConfig
	current            atomic.Pointer[bot.BotAPI]
	resolveGroupLister func() groupLister
	resolveGroupSender func() groupMessageSender
	resolveFeedSender  func() feedSender
	sessions           *sessionStore
	loginAttempts      *loginAttemptStore
}

const maxWebUIRequestBytes = 64 << 20

func New(cfg config.WebUIConfig) *Server {
	gin.SetMode(gin.ReleaseMode)
	s := &Server{
		cfg:           cfg,
		sessions:      newSessionStore(),
		loginAttempts: newLoginAttemptStore(),
	}
	s.resolveGroupLister = func() groupLister {
		api := s.current.Load()
		if api == nil {
			return nil
		}
		return api
	}
	s.resolveGroupSender = func() groupMessageSender {
		api := s.current.Load()
		if api == nil {
			return nil
		}
		return api
	}
	s.resolveFeedSender = func() feedSender {
		api := s.current.Load()
		if api == nil {
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
	protected.GET("/groups", s.handleGroups)
	protected.POST("/groups/:groupID/messages", s.handleSendGroupMessage)
	protected.GET("/ai-style/default", s.handleDefaultAIStyleGet)
	protected.PUT("/ai-style/default", s.handleDefaultAIStyleSet)
	protected.DELETE("/ai-style/default", s.handleDefaultAIStyleDelete)
	protected.GET("/groups/:groupID/ai-style", s.handleGroupAIStyleGet)
	protected.PUT("/groups/:groupID/ai-style", s.handleGroupAIStyleSet)
	protected.DELETE("/groups/:groupID/ai-style", s.handleGroupAIStyleDelete)
	protected.GET("/plugins", s.handlePlugins)
	protected.GET("/command-usage", s.handleCommandUsage)
	protected.GET("/groups/:groupID/command-usage", s.handleGroupCommandUsage)
	protected.GET("/groups/:groupID/plugins", s.handleGroupPlugins)
	protected.PUT("/groups/:groupID/plugins/:pluginID", s.handleSetGroupPlugin)
	protected.POST("/plugins/:pluginID/apply-all", s.handleApplyPluginAll)
	protected.GET("/feeds", s.handleFeedList)
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

func (s *Server) Start(addr string) {
	logx.Infof("[webui] serving on %s", addr)
	server := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Minute,
		IdleTimeout:       90 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		logx.Fatalf("[webui] server error: %v", err)
	}
}
