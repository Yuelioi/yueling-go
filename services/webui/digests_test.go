package webui

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
)

func TestDigestListSetAndDelete(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	s.current.Store(&bot.BotAPI{})
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/groups/100/digest", `{"send_time":"21:30","message_count":80}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"SendTime":"21:30"`) {
		t.Fatalf("set code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/digests", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"GroupID":100`) {
		t.Fatalf("list code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodDelete, "/api/webui/groups/100/digest", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDigestSetRejectsInvalidInputAndOfflineBot(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	tests := []struct {
		path string
		body string
		code int
	}{
		{"/api/webui/groups/no/digest", `{"send_time":"21:30","message_count":80}`, http.StatusBadRequest},
		{"/api/webui/groups/100/digest", `{`, http.StatusBadRequest},
		{"/api/webui/groups/100/digest", `{"send_time":"","message_count":80}`, http.StatusBadRequest},
		{"/api/webui/groups/100/digest", `{"send_time":"21:30","message_count":9}`, http.StatusBadRequest},
		{"/api/webui/groups/100/digest", `{"send_time":"21:30","message_count":80}`, http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		rec := testAPIRequest(t, s, http.MethodPut, tt.path, tt.body, cookie)
		if rec.Code != tt.code {
			t.Fatalf("path=%s body=%s code=%d response=%s", tt.path, tt.body, rec.Code, rec.Body.String())
		}
	}
}
