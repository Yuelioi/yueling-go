package webui

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Yuelioi/yueling-go/services/wuxia"
)

func TestWuxiaEquipmentPreviewAuthAndValidation(t *testing.T) {
	s := newTestServer()
	for _, path := range []string{"/api/webui/wuxia-preview/catalog", "/api/webui/wuxia-preview/card?build={}"} {
		if r := testAPIRequest(t, s, http.MethodGet, path, "", nil); r.Code != 401 {
			t.Fatal("preview bypasses auth")
		}
	}
	cookie := login(t, s)
	rec := testAPIRequest(t, s, "GET", "/api/webui/wuxia-preview/catalog", "", cookie)
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var data struct {
		Catalog wuxia.Catalog   `json:"catalog"`
		Balance json.RawMessage `json:"balance"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data.Catalog.Version == wuxia.DefaultCatalog().Version || len(data.Catalog.Weapons) != 30 {
		t.Fatal("draft missing or live config mutated")
	}
	payload := `{"left":{"profession":"swordsman","level":1},"right":{"profession":"blade","level":10}}`
	rec = testAPIRequest(t, s, "POST", "/api/webui/wuxia-preview/build", payload, cookie)
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	bad := `{"left":{"profession":"swordsman","level":1,"weapon":"刀-1"},"right":{"profession":"blade","level":10}}`
	if r := testAPIRequest(t, s, "POST", "/api/webui/wuxia-preview/build", bad, cookie); r.Code != 400 {
		t.Fatal("illegal loadout accepted")
	}
}

func TestWuxiaCardCacheSkipsRenderingOnRevalidation(t *testing.T) {
	s := newTestServer()
	cookie := login(t, s)
	b := wuxia.PreviewBuild{Profession: "swordsman", Level: 1}
	raw, _ := json.Marshal(b)
	path := "/api/webui/wuxia-preview/card?build=" + url.QueryEscape(string(raw))
	// A matching token is calculated from the same validated view as the HTTP endpoint.
	c := wuxia.EquipmentPreviewCatalog()
	v, err := c.PreviewBuild(b)
	if err != nil {
		t.Fatal(err)
	}
	serialized, _ := json.Marshal(v)
	hash := sha256.Sum256(append([]byte(c.Version+"-preview-card-1"), serialized...))
	req := httptest.NewRequest("GET", path, nil)
	req.AddCookie(cookie)
	req.Header.Set("If-None-Match", fmt.Sprintf(`W/"%x"`, hash[:]))
	rec := serve(s, req)
	if rec.Code != 304 || rec.Body.Len() != 0 {
		t.Fatalf("card cache miss: %d %s", rec.Code, rec.Body.String())
	}
}

func TestWuxiaPreviewDraw(t *testing.T) {
	s := newTestServer()
	path := "/api/webui/wuxia-preview/draw"
	body := `{"profession":"swordsman","count":10}`
	if r := testAPIRequest(t, s, "POST", path, body, nil); r.Code != 401 {
		t.Fatal("draw preview bypassed auth")
	}
	cookie := login(t, s)
	r := testAPIRequest(t, s, "POST", path, body, cookie)
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	var response struct {
		Items []wuxia.Weapon `json:"items"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 10 {
		t.Fatal("missing rolls")
	}
	for _, w := range response.Items {
		if w.Kind != "剑" || w.WeaponLevel != 1 {
			t.Fatal("wrong drawn weapon")
		}
	}
	if r := testAPIRequest(t, s, "POST", path, `{"profession":"swordsman","count":100000}`, cookie); r.Code != 400 {
		t.Fatal("invalid count accepted")
	}
}
