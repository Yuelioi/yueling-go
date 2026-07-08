package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func newTestServer() *Server {
	return New(config.WebUIConfig{Enabled: true, Addr: ":0", Password: "secret"})
}

func serve(s *Server, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func login(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/webui/auth/login", strings.NewReader(`{"password":"secret"}`))
	rec := serve(s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login code=%d body=%s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("login did not set a session cookie")
	}
	return cookies[0]
}

func TestLoginSetsSessionCookie(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/webui/auth/login", strings.NewReader(`{"password":"secret"}`))
	rec := serve(s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("login did not set a session cookie")
	}
	if !cookies[0].HttpOnly {
		t.Fatalf("session cookie is not HTTP-only")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/webui/auth/login", strings.NewReader(`{"password":"wrong"}`))
	rec := serve(s, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"error":"unauthorized"`) {
		t.Fatalf("body=%s, want unauthorized JSON", rec.Body.String())
	}
}

func TestProtectedRouteRequiresSession(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/webui/auth/me", nil)
	rec := serve(s, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	s := newTestServer()
	cookie := login(t, s)

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/webui/auth/logout", nil)
	logoutReq.AddCookie(cookie)
	logoutRec := serve(s, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout code=%d body=%s", logoutRec.Code, logoutRec.Body.String())
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/webui/auth/me", nil)
	meReq.AddCookie(cookie)
	meRec := serve(s, meReq)
	if meRec.Code != http.StatusUnauthorized {
		t.Fatalf("me code=%d body=%s, want unauthorized after logout", meRec.Code, meRec.Body.String())
	}
}

func TestStaticServesIndexForSPARoute(t *testing.T) {
	dir := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	if err := os.MkdirAll(filepath.Join(dir, "webui", "dist"), 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "webui", "dist", "index.html"), []byte("spa shell"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/groups", nil)
	rec := serve(s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "spa shell" {
		t.Fatalf("body=%q, want index.html", rec.Body.String())
	}
}

func TestStaticReturnsJSONForUnmatchedAPI(t *testing.T) {
	s := newTestServer()
	for _, requestPath := range []string{"/api/webui/missing", "/api"} {
		req := httptest.NewRequest(http.MethodGet, requestPath, nil)
		rec := serve(s, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s code=%d body=%s", requestPath, rec.Code, rec.Body.String())
		}
		if ct := rec.Result().Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("%s content-type=%q, want JSON", requestPath, ct)
		}
	}
}
