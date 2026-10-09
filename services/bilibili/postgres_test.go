package bilibili

import (
	"encoding/json"
	"testing"

	"github.com/Yuelioi/yueling-go/internal/testdb"
)

func TestBilibiliCheckpointPersistence(t *testing.T) {
	testdb.Init(t)
	data, err := NewPostgresStore(testdb.Database).Load(10)
	if err != nil || len(data) != 0 {
		t.Fatalf("empty=%s err=%v", data, err)
	}
	for _, payload := range []string{`{"mode":"off","cursors":{"20":{"seq":12}}}`, `{"mode":"keyword","cursors":{"20":{"seq":13}}}`} {
		if err := NewPostgresStore(testdb.Database).Save(10, []byte(payload)); err != nil {
			t.Fatal(err)
		}
	}
	data, err = NewPostgresStore(testdb.Database).Load(10)
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(data, &state); err != nil || state.Mode != "keyword" {
		t.Fatalf("state=%s err=%v", data, err)
	}
	other, err := NewPostgresStore(testdb.Database).Load(11)
	if err != nil || len(other) != 0 {
		t.Fatal("checkpoint leaked across accounts")
	}
}

func TestCheckpointStoresOwnTheirDatabase(t *testing.T) {
	testdb.Init(t)
	first := NewPostgresStore(testdb.Database)
	if err := first.Save(10, []byte(`{"mode":"off"}`)); err != nil {
		t.Fatal(err)
	}
	testdb.Init(t)
	second := NewPostgresStore(testdb.Database)
	if err := second.Save(10, []byte(`{"mode":"keyword"}`)); err != nil {
		t.Fatal(err)
	}
	a, err := first.Load(10)
	if err != nil {
		t.Fatal(err)
	}
	b, err := second.Load(10)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) == string(b) {
		t.Fatal("checkpoint stores shared data")
	}
}
