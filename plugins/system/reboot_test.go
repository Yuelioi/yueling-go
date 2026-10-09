package system

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/gorilla/websocket"
)

func TestRebootRequestsApplicationRestartAfterReply(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	c := old.Clone()
	c.Bot.SuperUsers = []int64{42}
	c.Plugins.Disabled = nil
	config.Publish(c)
	b := bot.New()
	defer func() { b.Stop(); b.Wait() }()
	restarted := make(chan struct{}, 1)
	RegisterReboot(b, func() { restarted <- struct{}{} })
	server := httptest.NewServer(b.ReverseHandler(""))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/onebot/v11/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	prefix, _ := bot.CommandOptions()
	event := map[string]any{"post_type": "message", "message_type": "group", "self_id": 100, "group_id": 1, "user_id": 42, "message": []map[string]any{{"type": "text", "data": map[string]string{"text": prefix + "reboot"}}}, "sender": map[string]any{"user_id": 42, "role": "member"}}
	if err := conn.WriteJSON(event); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var request struct {
		Action string          `json:"action"`
		Echo   string          `json:"echo"`
		Params json.RawMessage `json:"params"`
	}
	for {
		if err := conn.ReadJSON(&request); err != nil {
			t.Fatal(err)
		}
		if request.Action == "send_group_msg" {
			break
		}
		if request.Action != "get_group_info" {
			t.Fatalf("unexpected %s", request.Action)
		}
		if err := conn.WriteJSON(map[string]any{"status": "ok", "retcode": 0, "echo": request.Echo, "data": map[string]any{"group_id": 1, "group_name": "fixture"}}); err != nil {
			t.Fatal(err)
		}
	}

	select {
	case <-restarted:
		t.Fatal("restart preceded reply completion")
	default:
	}
	if err := conn.WriteJSON(map[string]any{"status": "ok", "retcode": 0, "echo": request.Echo, "data": map[string]int{"message_id": 1}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-restarted:
	case <-time.After(3 * time.Second):
		t.Fatal("restart not requested")
	}
}
