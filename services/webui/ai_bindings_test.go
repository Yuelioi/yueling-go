package webui

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/runtimeconfig"
)

func TestFeatureSelectionsSurviveSaveResponseAndRefresh(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	t.Setenv("YUELING_DATABASE_DSN", "postgres://test@localhost/test")
	t.Setenv("YUELING_WEBUI_PASSWORD", "fixture-password")
	if err := config.LoadBootstrap(filepath.Join(t.TempDir(), "absent.toml")); err != nil {
		t.Fatal(err)
	}
	manager := runtimeconfig.New(&configurationStore{}, nil)
	if err := manager.Initialize(); err != nil {
		t.Fatal(err)
	}
	server := newTestServer()
	server.configuration = manager
	cookie := login(t, server)
	save := func(values map[string]any) []byte {
		t.Helper()
		body, err := json.Marshal(map[string]any{"revision": manager.View().Revision, "section": "ai", "values": values})
		if err != nil {
			t.Fatal(err)
		}
		response := testAPIRequest(t, server, http.MethodPut, "/api/webui/config", string(body), cookie)
		if response.Code != 200 {
			t.Fatalf("save failed %d: %s", response.Code, response.Body.String())
		}
		var result struct {
			Config json.RawMessage `json:"config"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Config
	}
	save(map[string]any{
		"ai.profiles":    []config.AIProfile{{ID: "fixture", Name: "多模态", Model: "fixture-model", ConnectionID: "account", Capabilities: []string{"chat", "tools", "vision", "image_generation"}}},
		"ai.bindings":    map[string]string{"conversation": "off", "vision": "off", "image_generation": "fixture"},
		"ai.connections": []config.AIConnection{{ID: "account", Name: "Account", Kind: "openai", BaseURL: "https://fixture.invalid/v1"}},
	})
	verify := func(data []byte) {
		t.Helper()
		var view runtimeconfig.View
		if err := json.Unmarshal(data, &view); err != nil {
			t.Fatal(err)
		}
		fields := map[string]any{}
		for _, section := range view.Sections {
			for _, field := range section.Fields {
				fields[field.Path] = field.Value
			}
		}
		bindings, ok := fields["ai.bindings"].(map[string]any)
		if !ok || len(bindings) != 3 || bindings["conversation"] != "fixture" || bindings["vision"] != "fixture" || bindings["image_generation"] != "fixture" {
			t.Fatal("binding reset in response", bindings)
		}
	}
	verify(save(map[string]any{"ai.bindings": map[string]string{"conversation": "fixture", "vision": "fixture", "image_generation": "fixture"}}))
	response := testAPIRequest(t, server, http.MethodGet, "/api/webui/config", "", cookie)
	if response.Code != 200 {
		t.Fatal(response.Code)
	}
	verify(response.Body.Bytes())
}
