package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/internal/testdb"
)

type stubGroupLister struct {
	groups []bot.GroupInfo
	err    error
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
	testdb.Init(t)
}
