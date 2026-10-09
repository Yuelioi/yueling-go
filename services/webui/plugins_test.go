package webui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
)

func TestPluginsRequiresSession(t *testing.T) {
	s := newTestServer()

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/plugins", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPluginsReturnsCatalogAfterLogin(t *testing.T) {
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/plugins", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		OK      bool `json:"ok"`
		Plugins []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.OK || len(got.Plugins) == 0 {
		t.Fatalf("response = %+v, want non-empty catalog", got)
	}
}

func TestPluginsNeverReturnsNullCommands(t *testing.T) {
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/plugins", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"commands":null`) {
		t.Fatalf("body contains null commands: %s", rec.Body.String())
	}
}

func TestGroupPluginToggleAndApplyAll(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/groups/100/plugins/29", `{"disabled":true}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/groups/100/plugins", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list code=%d body=%s", rec.Code, rec.Body.String())
	}
	var listed struct {
		OK       bool         `json:"ok"`
		Disabled map[int]bool `json:"disabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode disabled map: %v", err)
	}
	if !listed.OK || !listed.Disabled[29] {
		t.Fatalf("listed = %+v, want plugin 29 disabled", listed)
	}

	rec = testAPIRequest(t, s, http.MethodPost, "/api/webui/plugins/34/apply-all", `{"group_ids":[100,200],"disabled":true}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply code=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, groupID := range []int64{100, 200} {
		disabled, err := db.NewGroupStore(testdb.Database).IsGroupPluginDisabled(groupID, 34)
		if err != nil {
			t.Fatalf("is disabled group %d: %v", groupID, err)
		}
		if !disabled {
			t.Fatalf("group %d plugin 34 disabled = false, want true", groupID)
		}
	}
}

func TestPluginAPIsRejectInvalidParamsAndBodies(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"zero group", http.MethodGet, "/api/webui/groups/0/plugins", ""},
		{"negative group", http.MethodPut, "/api/webui/groups/-1/plugins/29", `{"disabled":true}`},
		{"zero plugin", http.MethodPut, "/api/webui/groups/100/plugins/0", `{"disabled":true}`},
		{"bad toggle body", http.MethodPut, "/api/webui/groups/100/plugins/29", `{`},
		{"empty apply groups", http.MethodPost, "/api/webui/plugins/29/apply-all", `{"group_ids":[],"disabled":true}`},
		{"bad apply group id", http.MethodPost, "/api/webui/plugins/29/apply-all", `{"group_ids":[100,0],"disabled":true}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := testAPIRequest(t, s, tt.method, tt.path, tt.body, cookie)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestPluginsRequireDisabledField(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/groups/100/plugins/29", `{}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("toggle code=%d body=%s", rec.Code, rec.Body.String())
	}
	disabled, err := db.NewGroupStore(testdb.Database).IsGroupPluginDisabled(100, 29)
	if err != nil {
		t.Fatalf("check toggle mutation: %v", err)
	}
	if disabled {
		t.Fatalf("missing disabled field mutated single toggle")
	}

	rec = testAPIRequest(t, s, http.MethodPost, "/api/webui/plugins/34/apply-all", `{"group_ids":[100,200]}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("apply code=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, groupID := range []int64{100, 200} {
		disabled, err := db.NewGroupStore(testdb.Database).IsGroupPluginDisabled(groupID, 34)
		if err != nil {
			t.Fatalf("check apply mutation group %d: %v", groupID, err)
		}
		if disabled {
			t.Fatalf("missing disabled field mutated apply-all for group %d", groupID)
		}
	}
}
