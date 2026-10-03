package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	bservice "github.com/Yuelioi/yueling-go/services/bilibili"
)

type bilibiliStoreStub struct{ data []byte }

func (s *bilibiliStoreStub) Load(int64) ([]byte, error) { return s.data, nil }
func (s *bilibiliStoreStub) Save(_ int64, data []byte) error {
	s.data = append([]byte(nil), data...)
	return nil
}

type bilibiliBackendStub struct{}

func (bilibiliBackendStub) AccountID() int64 { return 10 }
func (bilibiliBackendStub) Sessions(context.Context, int64) ([]bservice.Session, error) {
	return nil, nil
}
func (bilibiliBackendStub) Messages(context.Context, int64, int64) ([]bservice.PrivateMessage, error) {
	return nil, nil
}
func (bilibiliBackendStub) SendText(context.Context, int64, string) error { return nil }

func TestBilibiliSettingsAPIAuthValidationAndPersistence(t *testing.T) {
	s := newTestServer()
	cfg := config.BilibiliConfig{Enabled: true, Cookie: "SESSDATA=do-not-return", PollSeconds: 60, RecentCount: 5, ReplyMode: "off", ReplyCooldownSeconds: 60, ReplyPerMinute: 5}
	store := &bilibiliStoreStub{}
	m := bservice.NewManager(bilibiliBackendStub{}, store, cfg, []int64{99}, nil)
	s.BindBilibili(m)
	if rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/bilibili", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated %d", rec.Code)
	}
	cookie := login(t, s)
	settings, err := m.Settings()
	if err != nil {
		t.Fatal(err)
	}
	settings.PollSeconds = 300
	settings.Enabled = false
	settings.NotifyUsers = []int64{77}
	settings.Keywords = []config.BilibiliKeyword{{Keyword: "你好", Reply: "收到", Exact: true}}
	body, _ := json.Marshal(settings)
	rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/bilibili/settings", string(body), cookie)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "do-not-return") {
		t.Fatalf("save %d %s", rec.Code, rec.Body)
	}
	restarted := bservice.NewManager(bilibiliBackendStub{}, store, cfg, []int64{99}, nil)
	s.BindBilibili(restarted)
	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/bilibili", "", cookie)
	var response struct {
		Settings bservice.Settings `json:"settings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || response.Settings.Enabled || response.Settings.PollSeconds != 300 || len(response.Settings.Keywords) != 1 {
		t.Fatalf("reload %+v", response)
	}
	settings.PollSeconds = 5
	body, _ = json.Marshal(settings)
	if rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/bilibili/settings", string(body), cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid interval %d", rec.Code)
	}
	if rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/bilibili/settings", `{"cookie":"unexpected"}`, cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected field %d", rec.Code)
	}
}
