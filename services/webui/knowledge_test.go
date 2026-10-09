package webui

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
)

func TestKnowledgeAPILifecycleSearchAndIsolation(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodPost, "/api/webui/groups/100/knowledge", `{"title":"入群规则","content":"新成员需要修改群名片","shortcuts":["新人规则","入群须知"]}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"title":"入群规则"`) || !strings.Contains(rec.Body.String(), `"trigger":"新人规则"`) {
		t.Fatalf("add code=%d body=%s", rec.Code, rec.Body.String())
	}
	var added struct {
		Knowledge db.GroupKnowledge `json:"knowledge"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &added); err != nil || added.Knowledge.ID == 0 {
		t.Fatalf("decode row=%+v err=%v", added.Knowledge, err)
	}
	if _, err := db.NewKnowledgeStore(testdb.Database).CreateGroupKnowledge(200, 1, "其他群", "不可见内容", ""); err != nil {
		t.Fatal(err)
	}
	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/groups/100/knowledge/search?q=%E7%BE%A4%E5%90%8D%E7%89%87", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "入群规则") || strings.Contains(rec.Body.String(), "不可见内容") {
		t.Fatalf("search code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/knowledge?group_id=100", "", cookie)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "其他群") || !strings.Contains(rec.Body.String(), "入群规则") || !strings.Contains(rec.Body.String(), "入群须知") {
		t.Fatalf("list code=%d body=%s", rec.Code, rec.Body.String())
	}

	path := "/api/webui/groups/100/knowledge/" + strconv.FormatUint(uint64(added.Knowledge.ID), 10) + "/shortcuts"
	rec = testAPIRequest(t, s, http.MethodPut, path, `{"shortcuts":["群规"]}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"trigger":"群规"`) || strings.Contains(rec.Body.String(), "新人规则") {
		t.Fatalf("set shortcuts code=%d body=%s", rec.Code, rec.Body.String())
	}
	path = "/api/webui/groups/200/knowledge/" + strconv.FormatUint(uint64(added.Knowledge.ID), 10) + "/shortcuts"
	rec = testAPIRequest(t, s, http.MethodPut, path, `{"shortcuts":["越权"]}`, cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-group shortcut update code=%d body=%s", rec.Code, rec.Body.String())
	}

	path = "/api/webui/groups/200/knowledge/" + strconv.FormatUint(uint64(added.Knowledge.ID), 10)
	rec = testAPIRequest(t, s, http.MethodDelete, path, "", cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-group delete code=%d body=%s", rec.Code, rec.Body.String())
	}
	path = "/api/webui/groups/100/knowledge/" + strconv.FormatUint(uint64(added.Knowledge.ID), 10)
	rec = testAPIRequest(t, s, http.MethodDelete, path, "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSharedKnowledgeAPILifecycleAndGroupVisibility(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodPost, "/api/webui/knowledge/shared", `{"title":"公共能力","content":"月灵支持自然语言提醒","shortcuts":["月灵功能"]}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"group_id":0`) || !strings.Contains(rec.Body.String(), `"trigger":"月灵功能"`) {
		t.Fatalf("shared add code=%d body=%s", rec.Code, rec.Body.String())
	}
	var added struct {
		Knowledge db.GroupKnowledge `json:"knowledge"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &added); err != nil || added.Knowledge.ID == 0 {
		t.Fatalf("decode shared row=%+v err=%v", added.Knowledge, err)
	}

	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/knowledge?group_id=0", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "公共能力") {
		t.Fatalf("shared list code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/groups/100/knowledge/search?q=%E8%87%AA%E7%84%B6%E8%AF%AD%E8%A8%80%E6%8F%90%E9%86%92", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "公共能力") {
		t.Fatalf("group shared search code=%d body=%s", rec.Code, rec.Body.String())
	}

	path := "/api/webui/knowledge/shared/" + strconv.FormatUint(uint64(added.Knowledge.ID), 10) + "/shortcuts"
	rec = testAPIRequest(t, s, http.MethodPut, path, `{"shortcuts":["公共帮助"]}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"trigger":"公共帮助"`) {
		t.Fatalf("shared shortcuts code=%d body=%s", rec.Code, rec.Body.String())
	}
	path = "/api/webui/groups/100/knowledge/" + strconv.FormatUint(uint64(added.Knowledge.ID), 10)
	rec = testAPIRequest(t, s, http.MethodDelete, path, "", cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("shared delete through group scope code=%d body=%s", rec.Code, rec.Body.String())
	}
	path = "/api/webui/knowledge/shared/" + strconv.FormatUint(uint64(added.Knowledge.ID), 10)
	rec = testAPIRequest(t, s, http.MethodDelete, path, "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("shared delete code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestKnowledgeAPIRejectsInvalidInput(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)
	for _, test := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/webui/groups/no/knowledge", `{"content":"x"}`},
		{http.MethodPost, "/api/webui/groups/100/knowledge", `{`},
		{http.MethodPost, "/api/webui/groups/100/knowledge", `{"content":""}`},
		{http.MethodPost, "/api/webui/groups/100/knowledge", `{"url":"file:///tmp/doc"}`},
		{http.MethodGet, "/api/webui/groups/100/knowledge/search?q=", ""},
		{http.MethodGet, "/api/webui/knowledge", ""},
		{http.MethodDelete, "/api/webui/groups/100/knowledge/no", ""},
	} {
		rec := testAPIRequest(t, s, test.method, test.path, test.body, cookie)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s %s code=%d body=%s", test.method, test.path, rec.Code, rec.Body.String())
		}
	}
}
