package bot

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestGroupNameCacheBelongsToConnection(t *testing.T) {
	first := &BotAPI{}
	first.groupNames.Store(int64(100), "old-name")
	if got := first.WithContext(context.Background()).GroupName(100); got != "old-name" {
		t.Fatalf("scoped calls lost connection cache: %q", got)
	}
	send := make(chan []byte, 1)
	second := &BotAPI{sendCh: send, done: make(chan struct{})}
	result := make(chan string, 1)
	go func() { result <- second.GroupName(100) }()
	select {
	case data := <-send:
		var payload struct {
			Echo string `json:"echo"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatal(err)
		}
		second.deliver(payload.Echo, json.RawMessage(`{"status":"ok","retcode":0,"data":{"group_name":"new-name"}}`))
	case got := <-result:
		t.Fatalf("new connection reused old name: %q", got)
	case <-time.After(time.Second):
		t.Fatal("new connection did not fetch name")
	}
	select {
	case got := <-result:
		if got != "new-name" {
			t.Fatalf("name=%q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("group lookup did not finish")
	}
	if got := second.WithContext(context.Background()).GroupName(100); got != "new-name" {
		t.Fatal("new name not cached")
	}
}
