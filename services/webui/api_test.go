package webui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
	"github.com/Yuelioi/yueling-go/services/feed"
	"gorm.io/gorm"
)

type stubGroupLister struct {
	groups []bot.GroupInfo
	err    error
}

func (s stubGroupLister) GetGroupList() ([]bot.GroupInfo, error) {
	return s.groups, s.err
}

type stubGroupSender struct {
	groupID   int64
	message   bot.Message
	messageID int32
	err       error
}

type stubFeedSender struct {
	groupID int64
	text    string
	err     error
}

func (s *stubFeedSender) SendGroupText(groupID int64, text string) error {
	s.groupID = groupID
	s.text = text
	return s.err
}

func (s *stubGroupSender) SendGroupMsg(groupID int64, msg bot.Message) (int32, error) {
	s.groupID = groupID
	s.message = msg
	return s.messageID, s.err
}

func segmentDataString(t *testing.T, seg bot.Segment, field string) string {
	t.Helper()
	var data map[string]string
	if err := json.Unmarshal(seg.Data, &data); err != nil {
		t.Fatalf("decode segment data: %v", err)
	}
	return data[field]
}

func testAPIRequest(t *testing.T, s *Server, method, target, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return serve(s, req)
}

func initWebUITestDB(t *testing.T) {
	t.Helper()
	oldDB := db.DB
	if err := testdb.Init(filepath.Join(t.TempDir(), "webui-test.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	tempDB := db.DB
	t.Cleanup(func() {
		if sqlDB, err := tempDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		db.DB = oldDB
	})
}

func withTestAffinityConfig(t *testing.T) {
	t.Helper()
	old := config.C
	config.C.AI.Affinity = config.AffinityConfig{
		Enabled:    true,
		Initial:    40,
		BlockBelow: 15,
		Min:        0,
		Max:        80,
	}
	t.Cleanup(func() {
		config.C = old
	})
}

func TestGroupsRequireLiveBot(t *testing.T) {
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/groups", "", cookie)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ok":false`) {
		t.Fatalf("body=%s, want error JSON", rec.Body.String())
	}
}

func TestGroupsReturnsListFromResolver(t *testing.T) {
	s := newTestServer()
	s.resolveGroupLister = func() groupLister {
		return stubGroupLister{groups: []bot.GroupInfo{
			{GroupID: 100, GroupName: "alpha"},
			{GroupID: 200, GroupName: "beta"},
		}}
	}
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/groups", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		OK     bool            `json:"ok"`
		Groups []bot.GroupInfo `json:"groups"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.OK || len(got.Groups) != 2 || got.Groups[0].GroupID != 100 || got.Groups[1].GroupName != "beta" {
		t.Fatalf("response = %+v, want stub groups", got)
	}
}

func TestGroupsReturnsBadGatewayOnListerError(t *testing.T) {
	s := newTestServer()
	s.resolveGroupLister = func() groupLister {
		return stubGroupLister{err: errors.New("napcat unavailable")}
	}
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/groups", "", cookie)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "napcat unavailable") {
		t.Fatalf("body=%s, want upstream error", rec.Body.String())
	}
}

func TestSendGroupMessageOK(t *testing.T) {
	s := newTestServer()
	sender := &stubGroupSender{messageID: 321}
	s.resolveGroupSender = func() groupMessageSender {
		return sender
	}
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodPost, "/api/webui/groups/100/messages", `{"text":"  hello group  "}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		OK        bool  `json:"ok"`
		MessageID int32 `json:"message_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.OK || got.MessageID != 321 {
		t.Fatalf("response = %+v, want ok with message_id 321", got)
	}
	if sender.groupID != 100 || sender.message.Text() != "hello group" {
		t.Fatalf("sent group=%d message=%q, want group 100 text trimmed", sender.groupID, sender.message.Text())
	}
}

func TestSendGroupMessageBuildsTextAtAndImages(t *testing.T) {
	s := newTestServer()
	sender := &stubGroupSender{messageID: 654}
	s.resolveGroupSender = func() groupMessageSender {
		return sender
	}
	cookie := login(t, s)

	body := `{"text":"  hello  ","at_user_ids":[123,456],"images":[" https://example.test/a.png ","base64://abc"]}`
	rec := testAPIRequest(t, s, http.MethodPost, "/api/webui/groups/100/messages", body, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(sender.message) != 5 {
		t.Fatalf("message segments=%+v, want 5 segments", sender.message)
	}
	wantTypes := []string{"text", "at", "at", "image", "image"}
	for i, wantType := range wantTypes {
		if sender.message[i].Type != wantType {
			t.Fatalf("segment %d type=%q, want %q", i, sender.message[i].Type, wantType)
		}
	}
	if got := segmentDataString(t, sender.message[0], "text"); got != "hello" {
		t.Fatalf("text=%q, want trimmed hello", got)
	}
	if got := segmentDataString(t, sender.message[1], "qq"); got != "123" {
		t.Fatalf("first at=%q, want 123", got)
	}
	if got := segmentDataString(t, sender.message[2], "qq"); got != "456" {
		t.Fatalf("second at=%q, want 456", got)
	}
	if got := segmentDataString(t, sender.message[3], "file"); got != "https://example.test/a.png" {
		t.Fatalf("first image=%q, want trimmed url", got)
	}
	if got := segmentDataString(t, sender.message[4], "file"); got != "base64://abc" {
		t.Fatalf("second image=%q, want base64", got)
	}
}

func TestSendGroupMessageAcceptsImageOnly(t *testing.T) {
	s := newTestServer()
	sender := &stubGroupSender{messageID: 987}
	s.resolveGroupSender = func() groupMessageSender {
		return sender
	}
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodPost, "/api/webui/groups/100/messages", `{"images":["https://example.test/a.png"]}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(sender.message) != 1 || sender.message[0].Type != "image" {
		t.Fatalf("message=%+v, want one image segment", sender.message)
	}
}

func TestSendGroupMessageRequiresLiveBot(t *testing.T) {
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodPost, "/api/webui/groups/100/messages", `{"text":"hello"}`, cookie)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "bot not connected") {
		t.Fatalf("body=%s, want bot connection error", rec.Body.String())
	}
}

func TestSendGroupMessageRejectsInvalidInput(t *testing.T) {
	s := newTestServer()
	s.resolveGroupSender = func() groupMessageSender {
		return &stubGroupSender{}
	}
	cookie := login(t, s)

	tests := []struct {
		name string
		path string
		body string
	}{
		{"bad group", "/api/webui/groups/abc/messages", `{"text":"hello"}`},
		{"zero group", "/api/webui/groups/0/messages", `{"text":"hello"}`},
		{"bad json", "/api/webui/groups/100/messages", `{`},
		{"empty text", "/api/webui/groups/100/messages", `{"text":"   "}`},
		{"bad at user", "/api/webui/groups/100/messages", `{"at_user_ids":[0]}`},
		{"blank image", "/api/webui/groups/100/messages", `{"images":["   "]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := testAPIRequest(t, s, http.MethodPost, tt.path, tt.body, cookie)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestSendGroupMessageReturnsBadGatewayOnSendError(t *testing.T) {
	s := newTestServer()
	s.resolveGroupSender = func() groupMessageSender {
		return &stubGroupSender{err: errors.New("napcat send failed")}
	}
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodPost, "/api/webui/groups/100/messages", `{"text":"hello"}`, cookie)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "napcat send failed") {
		t.Fatalf("body=%s, want upstream send error", rec.Body.String())
	}
}

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
		disabled, err := db.IsGroupPluginDisabled(groupID, 34)
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
	disabled, err := db.IsGroupPluginDisabled(100, 29)
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
		disabled, err := db.IsGroupPluginDisabled(groupID, 34)
		if err != nil {
			t.Fatalf("check apply mutation group %d: %v", groupID, err)
		}
		if disabled {
			t.Fatalf("missing disabled field mutated apply-all for group %d", groupID)
		}
	}
}

func TestAffinityListSetAdjustAndReset(t *testing.T) {
	initWebUITestDB(t)
	withTestAffinityConfig(t)
	s := newTestServer()
	cookie := login(t, s)

	row, err := db.UpdateAIAffinity(1, 100, "alice", 40, 5, 0, 80, "seed")
	if err != nil {
		t.Fatalf("seed affinity: %v", err)
	}
	if _, err := db.UpdateAIAffinity(2, 200, "bob", 40, 0, 0, 80, "seed"); err != nil {
		t.Fatalf("seed other affinity: %v", err)
	}

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/affinity?group_id=100&q=ali", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list code=%d body=%s", rec.Code, rec.Body.String())
	}
	var listed struct {
		OK         bool            `json:"ok"`
		Affinity   []db.AIAffinity `json:"affinity"`
		BlockBelow int             `json:"block_below"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if !listed.OK || listed.BlockBelow != 15 || len(listed.Affinity) != 1 || listed.Affinity[0].Nickname != "alice" {
		t.Fatalf("listed = %+v, want alice with block_below 15", listed)
	}

	rowID := strconv.FormatUint(uint64(row.ID), 10)
	rec = testAPIRequest(t, s, http.MethodPut, "/api/webui/affinity/"+rowID+"/score", `{"score":999}`, cookie)
	got := decodeAffinityMutation(t, rec, http.StatusOK)
	if got.Score != 80 || got.LastReason != "webui_set" {
		t.Fatalf("after set = %+v, want score 80 reason webui_set", got)
	}

	rec = testAPIRequest(t, s, http.MethodPost, "/api/webui/affinity/"+rowID+"/adjust", `{"delta":-200}`, cookie)
	got = decodeAffinityMutation(t, rec, http.StatusOK)
	if got.Score != 0 || got.LastReason != "webui_adjust" {
		t.Fatalf("after adjust = %+v, want score 0 reason webui_adjust", got)
	}

	rec = testAPIRequest(t, s, http.MethodPost, "/api/webui/affinity/"+rowID+"/reset", "", cookie)
	got = decodeAffinityMutation(t, rec, http.StatusOK)
	if got.Score != 40 || got.LastReason != "webui_reset" {
		t.Fatalf("after reset = %+v, want score 40 reason webui_reset", got)
	}
}

func TestAffinityAPIsRejectInvalidParamsAndBodies(t *testing.T) {
	initWebUITestDB(t)
	withTestAffinityConfig(t)
	s := newTestServer()
	cookie := login(t, s)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"bad query group", http.MethodGet, "/api/webui/affinity?group_id=abc", ""},
		{"zero query group", http.MethodGet, "/api/webui/affinity?group_id=0", ""},
		{"bad score id", http.MethodPut, "/api/webui/affinity/abc/score", `{"score":1}`},
		{"zero score id", http.MethodPut, "/api/webui/affinity/0/score", `{"score":1}`},
		{"bad score body", http.MethodPut, "/api/webui/affinity/1/score", `{`},
		{"zero delta", http.MethodPost, "/api/webui/affinity/1/adjust", `{"delta":0}`},
		{"bad adjust body", http.MethodPost, "/api/webui/affinity/1/adjust", `{`},
		{"bad reset id", http.MethodPost, "/api/webui/affinity/no/reset", ""},
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

func TestAffinityMissingRowReturnsNotFound(t *testing.T) {
	initWebUITestDB(t)
	withTestAffinityConfig(t)
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/affinity/999/score", `{"score":10}`, cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func decodeAffinityMutation(t *testing.T, rec *httptest.ResponseRecorder, wantCode int) db.AIAffinity {
	t.Helper()
	if rec.Code != wantCode {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		OK       bool          `json:"ok"`
		Affinity db.AIAffinity `json:"affinity"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode mutation: %v", err)
	}
	if !got.OK {
		t.Fatalf("response = %+v, want ok", got)
	}
	return got.Affinity
}

func TestMemoryListDeleteAndClear(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	first := db.SemanticMemory{UserID: 101, Content: "喜欢无糖咖啡", Category: "food", Score: 1, CreatedAt: 10}
	second := db.SemanticMemory{UserID: 202, Content: "每周末夜跑", Category: "hobby", Score: 1, CreatedAt: 20}
	if err := db.DB.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&second).Error; err != nil {
		t.Fatal(err)
	}

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/memories?q=101", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "喜欢无糖咖啡") || strings.Contains(rec.Body.String(), "每周末夜跑") {
		t.Fatalf("list code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = testAPIRequest(t, s, http.MethodDelete, "/api/webui/memories/"+strconv.FormatUint(uint64(first.ID), 10), "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete code=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := db.GetSemanticMemoryAdmin(first.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted memory err=%v, want not found", err)
	}

	rec = testAPIRequest(t, s, http.MethodDelete, "/api/webui/memories/users/202", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"deleted":1`) {
		t.Fatalf("clear code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMemoryAPIsRejectInvalidOrMissingIDs(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	for _, target := range []string{"/api/webui/memories/no", "/api/webui/memories/0", "/api/webui/memories/users/no", "/api/webui/memories/users/0"} {
		rec := testAPIRequest(t, s, http.MethodDelete, target, "", cookie)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("target=%s code=%d body=%s", target, rec.Code, rec.Body.String())
		}
	}
	rec := testAPIRequest(t, s, http.MethodDelete, "/api/webui/memories/999", "", cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOverviewReturnsOperationalCounts(t *testing.T) {
	initWebUITestDB(t)
	withTestAffinityConfig(t)
	s := newTestServer()
	s.resolveGroupLister = func() groupLister {
		return stubGroupLister{groups: []bot.GroupInfo{{GroupID: 100}, {GroupID: 200}}}
	}
	cookie := login(t, s)

	if _, err := db.UpdateAIAffinity(1, 100, "alice", 40, -30, 0, 80, "seed"); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&db.SemanticMemory{UserID: 1, Content: "偏好", Score: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpsertDailyDigest(100, 1, "21:30", "30 21 * * *", 80); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateFeedSubscription(100, 1, "https://example.com/feed.xml", "更新", "current"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateGroupKnowledge(100, 1, "规则", "文明交流", ""); err != nil {
		t.Fatal(err)
	}

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/overview", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, fragment := range []string{`"bot_connected":true`, `"group_count":2`, `"low_affinity_count":1`, `"memory_count":1`, `"digest_count":1`, `"feed_count":1`, `"knowledge_count":1`} {
		if !strings.Contains(rec.Body.String(), fragment) {
			t.Fatalf("body=%s, want %s", rec.Body.String(), fragment)
		}
	}
}

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
	if _, err := db.CreateGroupKnowledge(200, 1, "其他群", "不可见内容", ""); err != nil {
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

func TestFeedAPILifecycleAndManualCheck(t *testing.T) {
	initWebUITestDB(t)
	oldConfig := config.C
	config.C.Feed.RSSHubBase = "https://rss.example"
	t.Cleanup(func() { config.C = oldConfig })
	oldManager := feed.DefaultManager
	items := []feed.Item{{Key: "current", Title: "当前版本"}}
	feed.DefaultManager = feed.NewManager(func(string) (*feed.Feed, error) {
		return &feed.Feed{Title: "项目动态", Items: items}, nil
	})
	t.Cleanup(func() { feed.DefaultManager = oldManager })

	s := newTestServer()
	sender := &stubFeedSender{}
	s.resolveFeedSender = func() feedSender { return sender }
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodPost, "/api/webui/groups/100/feeds", `{"url":"https://example.com/feed.xml","name":""}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"项目动态"`) || !strings.Contains(rec.Body.String(), `"latest_title":"当前版本"`) {
		t.Fatalf("add code=%d body=%s", rec.Code, rec.Body.String())
	}
	var added struct {
		Feed db.FeedSubscription `json:"feed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &added); err != nil || added.Feed.ID == 0 {
		t.Fatalf("decode added feed: row=%+v err=%v", added.Feed, err)
	}
	rec = testAPIRequest(t, s, http.MethodPost, "/api/webui/groups/100/feeds/platform", `{"platform":"bilibili_video","target":"2267573","name":"UP投稿"}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"url":"https://rss.example/bilibili/user/video/2267573/1"`) {
		t.Fatalf("platform add code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/feeds", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"group_id":100`) {
		t.Fatalf("list code=%d body=%s", rec.Code, rec.Body.String())
	}
	feedPath := "/api/webui/groups/100/feeds/" + strconv.FormatUint(uint64(added.Feed.ID), 10)
	rec = testAPIRequest(t, s, http.MethodPut, feedPath, `{"enabled":false}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("pause feed code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/feeds", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("paused feed missing from list code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodPut, feedPath, `{"enabled":true}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":true`) {
		t.Fatalf("resume feed code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodPut, "/api/webui/groups/100/feeds/settings", `{"quiet_enabled":true,"quiet_start":"23:00","quiet_end":"08:00"}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"quiet_enabled":true`) || !strings.Contains(rec.Body.String(), `"quiet_start":"23:00"`) {
		t.Fatalf("set feed settings code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/groups/100/feeds/settings", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"quiet_end":"08:00"`) || !strings.Contains(rec.Body.String(), `"pending_count":0`) {
		t.Fatalf("get feed settings code=%d body=%s", rec.Code, rec.Body.String())
	}
	// Turn quiet delivery back off before the manual delivery assertion below.
	rec = testAPIRequest(t, s, http.MethodPut, "/api/webui/groups/100/feeds/settings", `{"quiet_enabled":false}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"quiet_enabled":false`) {
		t.Fatalf("disable feed settings code=%d body=%s", rec.Code, rec.Body.String())
	}

	items = []feed.Item{{Key: "new", Title: "新版本", Link: "https://example.com/new"}, {Key: "current", Title: "当前版本"}}
	rec = testAPIRequest(t, s, http.MethodPost, "/api/webui/groups/100/feeds/check", `{}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"updated":2`) || sender.groupID != 100 || !strings.Contains(sender.text, "新版本") {
		t.Fatalf("check code=%d body=%s sender=%+v", rec.Code, rec.Body.String(), sender)
	}

	path := "/api/webui/groups/100/feeds/" + strconv.FormatUint(uint64(added.Feed.ID), 10)
	rec = testAPIRequest(t, s, http.MethodDelete, path, "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodDelete, path, "", cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing delete code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFeedAPIRejectsInvalidInputAndOfflineCheck(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	tests := []struct {
		method string
		path   string
		body   string
		code   int
	}{
		{http.MethodPost, "/api/webui/groups/no/feeds", `{"url":"https://example.com/feed"}`, http.StatusBadRequest},
		{http.MethodPost, "/api/webui/groups/100/feeds", `{`, http.StatusBadRequest},
		{http.MethodPost, "/api/webui/groups/100/feeds", `{"url":"file:///tmp/feed"}`, http.StatusBadRequest},
		{http.MethodPost, "/api/webui/groups/100/feeds/platform", `{"platform":"x_user","target":"bad handle!"}`, http.StatusBadRequest},
		{http.MethodGet, "/api/webui/groups/no/feeds/settings", "", http.StatusBadRequest},
		{http.MethodPut, "/api/webui/groups/100/feeds/settings", `{`, http.StatusBadRequest},
		{http.MethodPut, "/api/webui/groups/100/feeds/settings", `{"quiet_enabled":true,"quiet_start":"08:00","quiet_end":"08:00"}`, http.StatusBadRequest},
		{http.MethodDelete, "/api/webui/groups/100/feeds/no", "", http.StatusBadRequest},
		{http.MethodPut, "/api/webui/groups/100/feeds/no", `{"enabled":true}`, http.StatusBadRequest},
		{http.MethodPut, "/api/webui/groups/100/feeds/999", `{"enabled":true}`, http.StatusNotFound},
		{http.MethodPut, "/api/webui/groups/100/feeds/1", `{}`, http.StatusBadRequest},
		{http.MethodPost, "/api/webui/groups/100/feeds/check", `{}`, http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		rec := testAPIRequest(t, s, tt.method, tt.path, tt.body, cookie)
		if rec.Code != tt.code {
			t.Fatalf("%s %s code=%d body=%s", tt.method, tt.path, rec.Code, rec.Body.String())
		}
	}
}

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
