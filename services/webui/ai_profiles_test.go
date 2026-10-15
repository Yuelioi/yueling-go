package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func TestAIAccountImportAndCapabilityProbeAreProtected(t *testing.T) {
	old := config.Current()
	defer config.Publish(old)
	cfg := old.Clone()
	cfg.Bot.DataDir = t.TempDir()
	cfg.AI.Connections = []config.AIConnection{{ID: "sub", Name: "Codex", Kind: "codex"}}
	config.Publish(cfg)
	s := newTestServer()
	cookie := login(t, s)
	credential := `{"auth_mode":"chatgpt","tokens":{"access_token":"private-access","refresh_token":"private-refresh","account_id":"fixture"}}`
	path := "/api/webui/ai/connections/sub/auth"
	if rec := testAPIRequest(t, s, http.MethodPut, path, credential, nil); rec.Code != 401 {
		t.Fatal(rec.Code)
	}
	for method, body := range map[string]string{http.MethodPut: credential, http.MethodGet: ""} {
		rec := testAPIRequest(t, s, method, path, body, cookie)
		if rec.Code != 200 || strings.Contains(rec.Body.String(), "private-") {
			t.Fatal(rec.Code, rec.Body.String())
		}
	}
	if rec := testAPIRequest(t, s, http.MethodPut, path, credential, cookie); rec.Code != 400 {
		t.Fatal("silently replaced credential", rec.Code)
	}
	if rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/ai/connections/missing/auth", credential, cookie); rec.Code != 404 {
		t.Fatal(rec.Code)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Error("wrong credential")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"role": "assistant", "content": "OK"}}}})
	}))
	defer srv.Close()
	cfg.AI.Connections = append(cfg.AI.Connections, config.AIConnection{ID: "api", Name: "API", Kind: "openai", BaseURL: srv.URL, APIKey: "fixture-key"})
	cfg.AI.Profiles = []config.AIProfile{{ID: "p", Name: "P", ConnectionID: "api", Model: "fixture", Capabilities: []string{"chat", "tools"}}}
	config.Publish(cfg)
	path = "/api/webui/ai/profiles/p/test"
	if rec := testAPIRequest(t, s, http.MethodPost, path, `{"capability":"chat"}`, nil); rec.Code != 401 {
		t.Fatal(rec.Code)
	}
	if rec := testAPIRequest(t, s, http.MethodPost, path, `{"capability":"chat"}`, cookie); rec.Code != 200 || !strings.Contains(rec.Body.String(), "verified") {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if rec := testAPIRequest(t, s, http.MethodPost, path, `{"capability":"tools"}`, cookie); rec.Code != 200 || !strings.Contains(rec.Body.String(), "failed") {
		t.Fatal("plain text accepted as tool support", rec.Body.String())
	}
	if rec := testAPIRequest(t, s, http.MethodPost, path, `{"capability":"vision"}`, cookie); rec.Code != 400 {
		t.Fatal(rec.Code)
	}
}
