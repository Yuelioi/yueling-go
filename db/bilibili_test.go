package db

import (
	"encoding/json"
	"testing"
)

func TestBilibiliCheckpointPersistence(t *testing.T) {
	initPostgresForTest(t)
	data, err := LoadBilibiliCheckpoint(10)
	if err != nil || len(data) != 0 {
		t.Fatalf("empty=%s err=%v", data, err)
	}
	for _, payload := range []string{`{"mode":"off","cursors":{"20":{"seq":12}}}`, `{"mode":"keyword","cursors":{"20":{"seq":13}}}`} {
		if err := SaveBilibiliCheckpoint(10, []byte(payload)); err != nil {
			t.Fatal(err)
		}
	}
	data, err = LoadBilibiliCheckpoint(10)
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(data, &state); err != nil || state.Mode != "keyword" {
		t.Fatalf("state=%s err=%v", data, err)
	}
	other, err := LoadBilibiliCheckpoint(11)
	if err != nil || len(other) != 0 {
		t.Fatal("checkpoint leaked across accounts")
	}
}
