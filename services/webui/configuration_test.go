package webui

import (
	"encoding/json"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/runtimeconfig"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type configurationStore struct {
	data     []byte
	revision int64
}

func (s *configurationStore) Load() ([]byte, int64, error) { return s.data, s.revision, nil }
func (s *configurationStore) Seed(p []byte) error          { s.data = p; s.revision = 1; return nil }
func (s *configurationStore) Save(p []byte, r int64) error { s.data = p; s.revision++; return nil }
func TestConfigurationAPIAuthSecretAndStrictUpdate(t *testing.T) {
	old := config.Current()
	defer config.Publish(old)
	t.Setenv("YUELING_DATABASE_DSN", "postgres://test@localhost/test")
	t.Setenv("YUELING_WEBUI_PASSWORD", "private-password")
	t.Setenv("YUELING_AI_DEEPSEEK_KEY", "private-key")
	if err := config.LoadBootstrap(filepath.Join(t.TempDir(), "absent.toml")); err != nil {
		t.Fatal(err)
	}
	m := runtimeconfig.New(&configurationStore{}, nil)
	if err := m.Initialize(); err != nil {
		t.Fatal(err)
	}
	s := newTestServer()
	s.configuration = m
	if r := testAPIRequest(t, s, http.MethodGet, "/api/webui/config", "", nil); r.Code != 401 {
		t.Fatal("unauthenticated access")
	}
	cookie := login(t, s)
	r := testAPIRequest(t, s, http.MethodGet, "/api/webui/config", "", cookie)
	if r.Code != 200 || strings.Contains(r.Body.String(), "private-key") || strings.Contains(r.Body.String(), "private-password") {
		t.Fatal("view exposes secrets or failed")
	}
	body, _ := json.Marshal(map[string]any{"revision": m.View().Revision, "section": "ai", "values": map[string]any{"ai.reasoning_effort": "high"}})
	r = testAPIRequest(t, s, http.MethodPut, "/api/webui/config", string(body), cookie)
	if r.Code != 200 || config.Current().AI.ReasoningEffort != "high" {
		t.Fatalf("update failed %d", r.Code)
	}
	if r = testAPIRequest(t, s, http.MethodPut, "/api/webui/config", string(body), cookie); r.Code != 409 {
		t.Fatal("stale write accepted")
	}
	for _, bad := range []string{`{"unknown":true}`, `{} {}`, `{"revision":2,"section":"database","values":{"database.dsn":"bad"}}`} {
		if r = testAPIRequest(t, s, http.MethodPut, "/api/webui/config", bad, cookie); r.Code != 400 {
			t.Fatalf("bad request accepted %d", r.Code)
		}
	}
}

func TestProfileAPIHidesNestedKeysAndRejectsInvalidCapabilities(t *testing.T) {
	old := config.Current()
	defer config.Publish(old)
	t.Setenv("YUELING_DATABASE_DSN", "postgres://test@localhost/test")
	t.Setenv("YUELING_WEBUI_PASSWORD", "private-password")
	if err := config.LoadBootstrap(filepath.Join(t.TempDir(), "absent.toml")); err != nil {
		t.Fatal(err)
	}
	m := runtimeconfig.New(&configurationStore{}, nil)
	if err := m.Initialize(); err != nil {
		t.Fatal(err)
	}
	s := newTestServer()
	s.configuration = m
	cookie := login(t, s)
	write := func(profiles any, vision string) int {
		body, _ := json.Marshal(map[string]any{"revision": m.View().Revision, "section": "ai", "values": map[string]any{"ai.profiles": profiles, "ai.bindings": map[string]string{"conversation": "shared", "vision": vision}, "ai.connections": []config.AIConnection{{ID: "account", Name: "Account", Kind: "openai", BaseURL: "https://example.com/v1", APIKey: "private-profile-key"}}}})
		r := testAPIRequest(t, s, http.MethodPut, "/api/webui/config", string(body), cookie)
		if strings.Contains(r.Body.String(), "private-profile-key") {
			t.Fatal("credential echoed in response")
		}
		return r.Code
	}
	profiles := []config.AIProfile{{ID: "shared", Name: "Shared", Model: "multimodal", ConnectionID: "account", Capabilities: []string{"chat", "tools", "vision"}}}
	if code := write(profiles, "shared"); code != 200 {
		t.Fatalf("shared profile update: %d", code)
	}
	r := testAPIRequest(t, s, http.MethodGet, "/api/webui/config", "", cookie)
	if r.Code != 200 || strings.Contains(r.Body.String(), "private-profile-key") || !strings.Contains(r.Body.String(), `"key_configured":true`) {
		t.Fatal("profile view not redacted")
	}
	profiles[0].APIKey = ""
	if code := write(profiles, "shared"); code != 200 || config.Current().AI.Connections[0].APIKey != "private-profile-key" {
		t.Fatal("blank key did not preserve credential")
	}
	profiles[0].Capabilities = []string{"chat", "tools"}
	if code := write(profiles, "shared"); code != 400 {
		t.Fatal("text model accepted for vision")
	}
	if code := write([]map[string]any{{"id": "shared", "unexpected": true}}, ""); code != 400 {
		t.Fatal("nested unknown field accepted")
	}
}
