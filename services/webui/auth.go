package webui

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/services/logx"
	"github.com/gin-gonic/gin"
)

const sessionCookieName = "yueling_webui_session"
const sessionTTL = 24 * time.Hour

const (
	maxLoginBodyBytes  = 4 << 10
	loginFailureLimit  = 5
	loginFailureWindow = 5 * time.Minute
	loginLockout       = 10 * time.Minute
)

type loginAttempt struct {
	failures    []time.Time
	lockedUntil time.Time
}

type loginAttemptStore struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
}

func newLoginAttemptStore() *loginAttemptStore {
	return &loginAttemptStore{attempts: map[string]loginAttempt{}}
}

func (s *loginAttemptStore) allowed(key string) (bool, time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	attempt := s.attempts[key]
	if now.Before(attempt.lockedUntil) {
		return false, time.Until(attempt.lockedUntil)
	}
	cutoff := now.Add(-loginFailureWindow)
	kept := attempt.failures[:0]
	for _, failure := range attempt.failures {
		if failure.After(cutoff) {
			kept = append(kept, failure)
		}
	}
	attempt.failures = kept
	attempt.lockedUntil = time.Time{}
	if len(attempt.failures) == 0 {
		delete(s.attempts, key)
	} else {
		s.attempts[key] = attempt
	}
	return true, 0
}

func (s *loginAttemptStore) failed(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	attempt := s.attempts[key]
	cutoff := now.Add(-loginFailureWindow)
	kept := attempt.failures[:0]
	for _, failure := range attempt.failures {
		if failure.After(cutoff) {
			kept = append(kept, failure)
		}
	}
	attempt.failures = append(kept, now)
	if len(attempt.failures) >= loginFailureLimit {
		attempt.lockedUntil = now.Add(loginLockout)
	}
	s.attempts[key] = attempt
}

func (s *loginAttemptStore) succeeded(key string) {
	s.mu.Lock()
	delete(s.attempts, key)
	s.mu.Unlock()
}

func loginClientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}

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
	now := time.Now()
	for existing, expires := range s.sessions {
		if now.After(expires) {
			delete(s.sessions, existing)
		}
	}
	s.sessions[token] = now.Add(sessionTTL)
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
	clientKey := loginClientKey(c.Request)
	if allowed, retryAfter := s.loginAttempts.allowed(clientKey); !allowed {
		seconds := max(1, int(retryAfter.Round(time.Second).Seconds()))
		c.Header("Retry-After", strconv.Itoa(seconds))
		jsonError(c, http.StatusTooManyRequests, "尝试次数过多，请稍后再试")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxLoginBodyBytes)
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if !passwordMatches(req.Password, s.cfg.Password) {
		s.loginAttempts.failed(clientKey)
		jsonError(c, http.StatusUnauthorized, "密码错误")
		return
	}
	s.loginAttempts.succeeded(clientKey)

	token, err := s.sessions.create()
	if err != nil {
		logx.Errorf("[webui] session token creation failed: %v", err)
		jsonError(c, http.StatusInternalServerError, "internal error")
		return
	}
	setSessionCookie(c, token, int(sessionTTL.Seconds()))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleLogout(c *gin.Context) {
	if token, err := c.Cookie(sessionCookieName); err == nil {
		s.sessions.delete(token)
	}
	setSessionCookie(c, "", -1)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleMe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "authenticated": true})
}

func (s *Server) requireSession(c *gin.Context) {
	token, err := c.Cookie(sessionCookieName)
	if err != nil || !s.sessions.valid(token) {
		jsonError(c, http.StatusUnauthorized, "请先登录")
		c.Abort()
		return
	}
	c.Next()
}

func jsonError(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"ok": false, "error": msg})
}

func passwordMatches(provided, configured string) bool {
	providedDigest := sha256.Sum256([]byte(provided))
	configuredDigest := sha256.Sum256([]byte(configured))
	return subtle.ConstantTimeCompare(providedDigest[:], configuredDigest[:]) == 1
}

func setSessionCookie(c *gin.Context, value string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, value, maxAge, "/", "", requestUsesSecureCookie(c.Request), true)
}

func requestUsesSecureCookie(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if i := strings.IndexByte(proto, ','); i >= 0 {
		proto = proto[:i]
	}
	return strings.EqualFold(strings.TrimSpace(proto), "https")
}
