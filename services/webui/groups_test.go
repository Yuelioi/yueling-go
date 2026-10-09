package webui

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
)

func (s stubGroupLister) GetGroupList() ([]bot.GroupInfo, error) {
	return s.groups, s.err
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
