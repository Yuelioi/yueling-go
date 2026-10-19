package emoticon

import (
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services"
	"github.com/gorilla/websocket"
)

func TestSpaceLookupMessageBoundaries(t *testing.T) {
	oldRoot := services.DataDir
	services.DataDir = t.TempDir()
	t.Cleanup(func() { services.DataDir = oldRoot })
	dir := filepath.Join(services.DataDir, "images", "表情")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "开心_fixture.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	b := bot.New()
	Register(b)
	const done = "fixture: dispatch complete"
	completed := make(chan struct{}, 1)
	b.OnGroupMessage().Priority(-100).Handle(func(c *bot.GroupContext) error {
		err := c.Send(done)
		completed <- struct{}{}
		return err
	})
	server := httptest.NewServer(b.ReverseHandler("fixture"))
	t.Cleanup(server.Close)
	t.Cleanup(func() { b.Stop(); b.Wait() })
	peer, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/onebot/v11/ws", http.Header{"Authorization": {"Bearer fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = peer.Close() })
	tests := []struct {
		name    string
		message bot.Message
		want    int
	}{
		{"images_with_padding", bot.Msg().Text(" ").Image("x").Text(" ").Image("y").Image("z").Build(), 0},
		{"image_and_keyword", bot.Msg().Text("  开心").Image("x").Build(), 0},
		{"reply_string_id", bot.Msg().Reply(9).Text("  开心").Build(), 0},
		{"reply_numeric_id", append(bot.Message{{Type: "reply", Data: json.RawMessage(`{"id":9}`)}}, bot.Msg().Text("  开心").Build()...), 0},
		{"reply_missing_id", append(bot.Message{{Type: "reply", Data: json.RawMessage(`{}`)}}, bot.Msg().Text("  开心").Build()...), 0},
		{"mention", bot.Msg().At(9).Text("  开心").Build(), 0},
		{"face_and_keyword", bot.Msg().Text("  开心").Face("1").Build(), 0},
		{"video_and_keyword", append(bot.Msg().Text("  开心").Build(), bot.Segment{Type: "video", Data: json.RawMessage(`{}`)}), 0},
		{"file_and_keyword", append(bot.Msg().Text("  开心").Build(), bot.Segment{Type: "file", Data: json.RawMessage(`{}`)}), 0},
		{"forward_and_keyword", append(bot.Msg().Text("  开心").Build(), bot.Segment{Type: "forward", Data: json.RawMessage(`{}`)}), 0},
		{"only_spaces", bot.Msg().Text("  ").Build(), 0},
		{"one_space_only", bot.Msg().Text(" ").Build(), 0},
		{"multiline_chat", bot.Msg().Text("  开心\n聊天").Build(), 0},
		{"three_spaces", bot.Msg().Text("   开心").Build(), 0},
		{"ordinary_chat", bot.Msg().Text("开心").Build(), 0},
		{"explicit_list", bot.Msg().Text("  开心").Build(), 1},
		{"split_text_list", bot.Msg().Text(" ").Text(" 开心").Build(), 1},
		{"explicit_random", bot.Msg().Text(" 开心").Build(), 1},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := peer.WriteJSON(bot.GroupMessageEvent{PostType: "message", MessageType: "group", MessageID: int32(i + 1), SelfID: 99, UserID: 42, GroupID: 7, Message: tt.message}); err != nil {
				t.Fatal(err)
			}
			_ = peer.SetReadDeadline(time.Now().Add(3 * time.Second))
			count := 0
			for {
				var call struct {
					Action string `json:"action"`
					Echo   string `json:"echo"`
					Params struct {
						Message bot.Message `json:"message"`
					} `json:"params"`
				}
				if err := peer.ReadJSON(&call); err != nil {
					t.Fatal(err)
				}
				if err := peer.WriteJSON(map[string]any{"echo": call.Echo, "status": "ok", "retcode": 0, "data": map[string]any{"message_id": 1, "group_name": "fixture"}}); err != nil {
					t.Fatal(err)
				}
				if call.Action == "get_group_info" {
					continue
				}
				if call.Action != "send_group_msg" {
					t.Fatalf("unexpected API call %s", call.Action)
				}
				if call.Params.Message.Text() == done {
					break
				}
				count++
			}
			if count != tt.want {
				t.Errorf("space lookup sent %d messages, want %d", count, tt.want)
			}
			select {
			case <-completed:
			case <-time.After(3 * time.Second):
				t.Fatal("dispatch did not finish")
			}
		})
	}
}
