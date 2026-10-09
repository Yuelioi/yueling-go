package bot

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestStopClosesReverseConnectionAndWaitsForWork(t *testing.T) {
	b := New()
	server := httptest.NewServer(b.ReverseHandler("test"))
	defer server.Close()
	header := map[string][]string{"Authorization": {"Bearer test"}}
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/onebot/v11/ws", header)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	started, release := make(chan struct{}), make(chan struct{})
	b.Go(func() { close(started); <-release })
	<-started
	b.Stop()
	b.Stop()
	if b.Go(func() { t.Error("admitted work after stop") }) {
		t.Fatal("accepted work")
	}
	done := make(chan struct{})
	go func() { b.Wait(); close(done) }()
	select {
	case <-done:
		t.Fatal("did not wait")
	default:
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("connection still open")
	}
	close(release)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("wait did not finish")
	}
}

func TestCanceledForwardConnectionDoesNotRetry(t *testing.T) {
	b := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.StartContext(ctx, "ws://127.0.0.1:1", ""); err != nil {
		t.Fatal(err)
	}
	b.Stop()
	b.Wait()
}

func TestNewConnectionCancelsOldConnectionHooks(t *testing.T) {
	b := New()
	defer func() { b.Stop(); b.Wait() }()
	connected := make(chan *BotAPI, 2)
	b.OnConnect(func(api *BotAPI) { connected <- api })
	server := httptest.NewServer(b.ReverseHandler("test"))
	defer server.Close()
	dial := func() *websocket.Conn {
		c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/onebot/v11/ws", map[string][]string{"Authorization": {"Bearer test"}})
		if err != nil {
			t.Fatal(err)
		}
		if err := c.WriteMessage(websocket.TextMessage, []byte(`{"post_type":"meta_event","meta_event_type":"lifecycle","self_id":123}`)); err != nil {
			t.Fatal(err)
		}
		return c
	}
	first := dial()
	defer first.Close()
	var old *BotAPI
	select {
	case old = <-connected:
	case <-time.After(time.Second):
		t.Fatal("missing first hook")
	}
	second := dial()
	defer second.Close()
	select {
	case <-connected:
	case <-time.After(time.Second):
		t.Fatal("missing replacement hook")
	}
	select {
	case <-old.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("old connection was not canceled")
	}
}
