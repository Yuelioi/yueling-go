package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
)

type stubSender struct {
	calledGroup, calledPrivate bool
	groupID, userID            int64
	msg                        bot.Message
	retID                      int32
	retErr                     error
}

func (s *stubSender) SendGroupMsg(groupID int64, msg bot.Message) (int32, error) {
	s.calledGroup = true
	s.groupID = groupID
	s.msg = msg
	return s.retID, s.retErr
}

func (s *stubSender) SendPrivateMsg(userID int64, msg bot.Message) (int32, error) {
	s.calledPrivate = true
	s.userID = userID
	s.msg = msg
	return s.retID, s.retErr
}

func newTestServer(key string, sender Sender) *Server {
	s := New(key)
	s.resolve = func() Sender { return sender }
	return s
}

func do(s *Server, method, auth, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/send", strings.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

const groupBody = `{"message_type":"group","group_id":123,"message":[{"type":"text","data":{"text":"hi"}}]}`

func TestSendGroupOK(t *testing.T) {
	stub := &stubSender{retID: 555}
	s := newTestServer("k", stub)
	rec := do(s, http.MethodPost, "Bearer k", groupBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !stub.calledGroup || stub.groupID != 123 {
		t.Fatalf("stub not called correctly: %+v", stub)
	}
	if len(stub.msg) != 1 || stub.msg[0].Type != "text" {
		t.Fatalf("message not passed through: %+v", stub.msg)
	}
}

func TestSendPrivateOK(t *testing.T) {
	stub := &stubSender{retID: 7}
	s := newTestServer("k", stub)
	body := `{"message_type":"private","user_id":42,"message":[{"type":"text","data":{"text":"yo"}}]}`
	rec := do(s, http.MethodPost, "Bearer k", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !stub.calledPrivate || stub.userID != 42 {
		t.Fatalf("stub not called correctly: %+v", stub)
	}
}

func TestMissingKey(t *testing.T) {
	s := newTestServer("k", &stubSender{})
	rec := do(s, http.MethodPost, "", groupBody)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestWrongKey(t *testing.T) {
	s := newTestServer("k", &stubSender{})
	rec := do(s, http.MethodPost, "Bearer nope", groupBody)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	s := newTestServer("k", &stubSender{})
	rec := do(s, http.MethodGet, "Bearer k", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestBadJSON(t *testing.T) {
	s := newTestServer("k", &stubSender{})
	rec := do(s, http.MethodPost, "Bearer k", "{not json")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestMissingGroupID(t *testing.T) {
	s := newTestServer("k", &stubSender{})
	body := `{"message_type":"group","message":[{"type":"text","data":{"text":"hi"}}]}`
	rec := do(s, http.MethodPost, "Bearer k", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestInvalidType(t *testing.T) {
	s := newTestServer("k", &stubSender{})
	body := `{"message_type":"channel","group_id":1,"message":[{"type":"text","data":{"text":"hi"}}]}`
	rec := do(s, http.MethodPost, "Bearer k", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestEmptyMessage(t *testing.T) {
	s := newTestServer("k", &stubSender{})
	body := `{"message_type":"group","group_id":1,"message":[]}`
	rec := do(s, http.MethodPost, "Bearer k", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestNotConnected(t *testing.T) {
	s := newTestServer("k", nil) // resolve 返回 nil 接口
	rec := do(s, http.MethodPost, "Bearer k", groupBody)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestSendError(t *testing.T) {
	stub := &stubSender{retErr: http.ErrServerClosed}
	s := newTestServer("k", stub)
	rec := do(s, http.MethodPost, "Bearer k", groupBody)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code=%d", rec.Code)
	}
}
