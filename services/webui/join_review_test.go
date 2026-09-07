package webui

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Yuelioi/yueling-go/db"
)

func TestJoinReviewAPIValidationAndAuth(t *testing.T) {
	s := newTestServer()
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		rec := testAPIRequest(t, s, method, "/api/webui/join-review/default", `{"mode":"override"}`, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("unauth %s: %d", method, rec.Code)
		}
	}
	cookie := login(t, s)
	for _, tc := range []struct{ scope, body string }{
		{"-1", `{"mode":"override"}`}, {"0", `{"mode":"override"}`}, {"oops", `{"mode":"override"}`},
		{"default", `{"mode":"inherit"}`}, {"123", `{"mode":"typo"}`}, {"123", `{"allow":true}`},
	} {
		rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/join-review/"+tc.scope, tc.body, cookie)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%+v: %d %s", tc, rec.Code, rec.Body.String())
		}
	}
}

func TestJoinReviewAPISavesEffectiveConfig(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)
	for _, scope := range []string{"default", "123"} {
		body := `{"mode":"override","allow":["ABC","abc"],"deny":["广告"]}`
		if scope == "123" {
			body = `{"mode":"inherit","allow":[],"deny":[]}`
		}
		rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/join-review/"+scope, body, cookie)
		if rec.Code != http.StatusOK {
			t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
		}
	}
	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/join-review/123", "", cookie)
	var state db.JoinReviewState
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || state.Config.Mode != db.JoinModeInherit || len(state.Effective.Allow) != 1 || state.Effective.Allow[0] != "abc" {
		t.Fatalf("read: %d %+v", rec.Code, state)
	}
}
