package webui

import (
	"errors"

	"github.com/Yuelioi/yueling-go/bot"
	"net/http"
	"strings"
	"testing"
)

func TestOverviewReportsDisconnectedBotOffline(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	s.resolveGroupLister = func() groupLister {
		return stubGroupLister{err: errors.New("connection closed: get_group_list")}
	}
	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/overview", "", login(t, s))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"bot_connected":false`) {
		t.Fatalf("disconnected bot reported online: %s", rec.Body.String())
	}
}

func TestResolversRejectInactiveBotConnection(t *testing.T) {
	s := newTestServer()
	for _, api := range []*bot.BotAPI{nil, {}} {
		s.current.Store(api)
		if s.resolveGroupLister() != nil || s.resolveGroupSender() != nil || s.resolveFeedSender() != nil {
			t.Fatal("inactive bot exposed as live API")
		}
	}
}
