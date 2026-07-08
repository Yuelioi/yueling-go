package webui

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/services/logx"
	"github.com/gin-gonic/gin"
)

const sessionCookieName = "yueling_webui_session"
const sessionTTL = 24 * time.Hour

type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]time.Time
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: map[string]time.Time{}}
}

func (s *sessionStore) create() (string, error) {
	var raw [32]byte
	n, err := rand.Read(raw[:])
	if err != nil {
		return "", err
	}
	if n != len(raw) {
		return "", io.ErrUnexpectedEOF
	}

	token := hex.EncodeToString(raw[:])
	s.mu.Lock()
	s.sessions[token] = time.Now().Add(sessionTTL)
	s.mu.Unlock()
	return token, nil
}

func (s *sessionStore) valid(token string) bool {
	if token == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	expires, ok := s.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(expires) {
		delete(s.sessions, token)
		return false
	}
	return true
}

func (s *sessionStore) delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

func (s *Server) handleLogin(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.cfg.Password)) != 1 {
		jsonError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	token, err := s.sessions.create()
	if err != nil {
		logx.Errorf("[webui] session token creation failed: %v", err)
		jsonError(c, http.StatusInternalServerError, "internal error")
		return
	}
	c.SetCookie(sessionCookieName, token, int(sessionTTL.Seconds()), "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleLogout(c *gin.Context) {
	if token, err := c.Cookie(sessionCookieName); err == nil {
		s.sessions.delete(token)
	}
	c.SetCookie(sessionCookieName, "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleMe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "authenticated": true})
}

func (s *Server) requireSession(c *gin.Context) {
	token, err := c.Cookie(sessionCookieName)
	if err != nil || !s.sessions.valid(token) {
		jsonError(c, http.StatusUnauthorized, "unauthorized")
		c.Abort()
		return
	}
	c.Next()
}

func jsonError(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"ok": false, "error": msg})
}
