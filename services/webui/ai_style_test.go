package webui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
)

func TestGroupAIStyleGetSetAndReset(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/groups/100/ai-style", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("initial get code=%d body=%s", rec.Code, rec.Body.String())
	}
	var initial struct {
		OK                 bool   `json:"ok"`
		GroupID            int64  `json:"group_id"`
		StylePrompt        string `json:"style_prompt"`
		Custom             bool   `json:"custom"`
		DefaultStylePrompt string `json:"default_style_prompt"`
		MaxChars           int    `json:"max_chars"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &initial); err != nil {
		t.Fatalf("decode initial: %v", err)
	}
	if !initial.OK || initial.GroupID != 100 || initial.Custom || initial.StylePrompt != "" ||
		initial.DefaultStylePrompt != ai.DefaultGroupStylePrompt || initial.MaxChars != db.MaxGroupAIStylePromptChars {
		t.Fatalf("initial response=%+v", initial)
	}

	rec = testAPIRequest(t, s, http.MethodPut, "/api/webui/groups/100/ai-style", `{"style_prompt":"  冷幽默，少用感叹号。  "}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"custom":true`) ||
		!strings.Contains(rec.Body.String(), "冷幽默，少用感叹号。") {
		t.Fatalf("set code=%d body=%s", rec.Code, rec.Body.String())
	}
	prompt, custom, err := db.NewGroupStore(testdb.Database).GetGroupAIStylePrompt(100)
	if err != nil || !custom || prompt != "冷幽默，少用感叹号。" {
		t.Fatalf("stored prompt=%q custom=%v err=%v", prompt, custom, err)
	}
	_, otherCustom, err := db.NewGroupStore(testdb.Database).GetGroupAIStylePrompt(200)
	if err != nil || otherCustom {
		t.Fatalf("other group custom=%v err=%v, want isolated default", otherCustom, err)
	}

	rec = testAPIRequest(t, s, http.MethodDelete, "/api/webui/groups/100/ai-style", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"custom":false`) {
		t.Fatalf("reset code=%d body=%s", rec.Code, rec.Body.String())
	}
	_, custom, err = db.NewGroupStore(testdb.Database).GetGroupAIStylePrompt(100)
	if err != nil || custom {
		t.Fatalf("after reset custom=%v err=%v", custom, err)
	}
}

func TestDefaultAIStyleSetAndGroupInheritance(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/ai-style/default", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"default_scope":true`) ||
		!strings.Contains(rec.Body.String(), `"custom":false`) {
		t.Fatalf("initial default code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = testAPIRequest(t, s, http.MethodPut, "/api/webui/ai-style/default", `{"style_prompt":"所有群默认简洁回答。"}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"custom":true`) {
		t.Fatalf("set default code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/groups/100/ai-style", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"style_prompt":""`) ||
		!strings.Contains(rec.Body.String(), `"default_style_prompt":"所有群默认简洁回答。"`) {
		t.Fatalf("group inheritance code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = testAPIRequest(t, s, http.MethodDelete, "/api/webui/ai-style/default", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"custom":false`) ||
		!strings.Contains(rec.Body.String(), ai.DefaultGroupStylePrompt) {
		t.Fatalf("reset default code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGroupAIStyleRejectsInvalidInput(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	oversized, err := json.Marshal(map[string]string{
		"style_prompt": strings.Repeat("月", db.MaxGroupAIStylePromptChars+1),
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/webui/groups/no/ai-style", ""},
		{http.MethodGet, "/api/webui/groups/0/ai-style", ""},
		{http.MethodPut, "/api/webui/groups/100/ai-style", `{}`},
		{http.MethodPut, "/api/webui/groups/100/ai-style", `{`},
		{http.MethodPut, "/api/webui/groups/100/ai-style", string(oversized)},
		{http.MethodPut, "/api/webui/ai-style/default", `{}`},
		{http.MethodPut, "/api/webui/ai-style/default", string(oversized)},
	}
	for _, tt := range tests {
		rec := testAPIRequest(t, s, tt.method, tt.path, tt.body, cookie)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s %s code=%d body=%s", tt.method, tt.path, rec.Code, rec.Body.String())
		}
	}
}
