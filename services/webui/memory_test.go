package webui

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
	"gorm.io/gorm"
)

func TestMemoryListDeleteAndClear(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	first := db.SemanticMemory{UserID: 101, Content: "喜欢无糖咖啡", Category: "food", Score: 1, CreatedAt: 10}
	second := db.SemanticMemory{UserID: 202, Content: "每周末夜跑", Category: "hobby", Score: 1, CreatedAt: 20}
	if err := testdb.Database.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := testdb.Database.Create(&second).Error; err != nil {
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
	if _, err := s.memory.GetSemanticMemoryAdmin(first.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
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
