package webui

import (
	"net/http"
	"sync/atomic"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/logx"
	"github.com/gin-gonic/gin"
)

type Server struct {
	cfg      config.WebUIConfig
	current  atomic.Pointer[bot.BotAPI]
	sessions *sessionStore
}

func New(cfg config.WebUIConfig) *Server {
	gin.SetMode(gin.ReleaseMode)
	return &Server{
		cfg:      cfg,
		sessions: newSessionStore(),
	}
}

func (s *Server) BindBot(b *bot.Bot) {
	b.OnConnect(func(api *bot.BotAPI) { s.current.Store(api) })
}

func (s *Server) Handler() http.Handler {
	r := gin.New()
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

	s.mountStatic(r)
	return r
}

func (s *Server) Start(addr string) {
	logx.Infof("[webui] serving on %s", addr)
	if err := http.ListenAndServe(addr, s.Handler()); err != nil {
		logx.Fatalf("[webui] server error: %v", err)
	}
}
