package webui

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Yuelioi/yueling-go/internal/testdb"
	"github.com/Yuelioi/yueling-go/services/wuxia"
)

func TestWuxiaSettingsScopeAndAuth(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	s.wuxia = wuxia.New(testdb.Database)
	if got := testAPIRequest(t, s, http.MethodGet, "/api/webui/wuxia/0", "", nil); got.Code != 401 {
		t.Fatal(got.Code)
	}
	cookie := login(t, s)
	read := func(scope string) wuxia.Configuration {
		t.Helper()
		rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/wuxia/"+scope, "", cookie)
		if rec.Code != 200 {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
		var body struct {
			Config wuxia.Configuration `json:"config"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body.Config
	}
	if read("100").Settings.DailyRuns != 3 {
		t.Fatal("default missing")
	}
	for _, scope := range []string{"0", "100"} {
		value := `{"daily_runs":5,"sign_silver":42,"practice_base":60}`
		if scope == "100" {
			value = `{"daily_runs":2,"sign_silver":35,"practice_base":75}`
		}
		rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/wuxia/"+scope, value, cookie)
		if rec.Code != 200 {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	}
	if read("100").Settings.PracticeBase != 75 || read("200").Settings.PracticeBase != 60 || read("100").Settings.SignSilver != 0 || read("100").Settings.DailyRuns != 3 || !read("100").Custom || read("200").Settings.DailyRuns != 3 || read("200").Custom {
		t.Fatal("scope settings mixed")
	}
	rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/wuxia/100", `{"daily_runs":99,"sign_silver":0,"practice_base":1}`, cookie)
	if rec.Code != 400 {
		t.Fatal("invalid settings accepted")
	}
	if read("100").Settings.DailyRuns != 3 {
		t.Fatal("invalid settings were saved")
	}
	rec = testAPIRequest(t, s, http.MethodDelete, "/api/webui/wuxia/100", "", cookie)
	if rec.Code != 200 || read("100").Custom || read("100").Settings.DailyRuns != 3 {
		t.Fatal("inheritance did not restore")
	}
}
