package webui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/llm"
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

func TestAIProfileProbeChecksReplyAndReturnsImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/images/generations" {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(testImage())}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"role": "assistant", "content": "Hello!"}}}})
	}))
	defer srv.Close()
	client := llm.New(llm.Settings{Model: "fixture", BaseURL: srv.URL})
	result := runAIProfileTest(context.Background(), client, "chat", "summary")
	if result["status"] != "failed" {
		t.Fatal("unrelated reply passed chat probe", result)
	}
	result = runAIProfileTest(context.Background(), client, "image_generation", "image_generation")
	if result["status"] != "verified" || !strings.HasPrefix(result["image_url"].(string), "data:image/png;base64,") {
		t.Fatal("generated image preview missing")
	}
}

func TestSearchCapabilityProbeUsesIndependentEndpoint(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/tools/search" || r.Header.Get("Authorization") != "Bearer search-fixture" {
			t.Error("search used wrong endpoint or credential")
		}
		w.Write([]byte(`{"results":[{"title":"官方文档","url":"https://example.com/a","summary":"内容"}]}`))
	}))
	defer server.Close()
	cfg := old.Clone()
	cfg.AI.Connections = []config.AIConnection{{ID: "api", Name: "DS", Kind: "openai", BaseURL: server.URL, APIKey: "search-fixture"}}
	cfg.AI.Profiles = []config.AIProfile{{ID: "search", Name: "Search", ConnectionID: "api", Capabilities: []string{"web_search"}}}
	cfg.AI.Bindings = nil
	config.Publish(cfg)
	s := newTestServer()
	cookie := login(t, s)
	path := "/api/webui/ai/profiles/search/test"
	if rec := testAPIRequest(t, s, "POST", path, `{"capability":"web_search"}`, nil); rec.Code != 401 {
		t.Fatal(rec.Code)
	}
	if rec := testAPIRequest(t, s, "POST", path, `{"capability":"web_search"}`, cookie); rec.Code != 200 || !strings.Contains(rec.Body.String(), "verified") || strings.Contains(rec.Body.String(), "search-fixture") {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if calls != 1 {
		t.Fatal(calls)
	}
	if config.Current().AI.Available("search") {
		t.Fatal("optional probe changed feature selection")
	}
}
