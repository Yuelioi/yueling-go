package wuxia

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/pgtest"
	"github.com/Yuelioi/yueling-go/services"
	game "github.com/Yuelioi/yueling-go/services/wuxia"
	"github.com/gorilla/websocket"
)

func TestGameCommandsThroughOneBotWithoutAI(t *testing.T) {
	previousDataDir := services.DataDir
	services.DataDir = filepath.Join("..", "..", "data")
	t.Cleanup(func() { services.DataDir = previousDataDir })
	database, err := db.Open(config.DatabaseConfig{DSN: pgtest.NewSchema(t), MaxOpen: 5, MaxIdle: 1, ConnMaxLifetime: 5})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { sql, _ := database.DB(); _ = sql.Close() }()
	b := bot.New()
	defer func() { b.Stop(); b.Wait() }()
	service := game.New(database)
	Register(b, service)
	if err := service.SetSettings(context.Background(), 100, game.Settings{DailyRuns: 3, SignSilver: 200, PracticeBase: 50}); err != nil {
		t.Fatal(err)
	}
	var competing atomic.Int32
	b.OnGroupMessage().Priority(50).Handle(func(*bot.GroupContext) error { competing.Add(1); return nil })
	connected := make(chan struct{}, 1)
	b.OnConnect(func(*bot.BotAPI) { connected <- struct{}{} })
	server := httptest.NewServer(b.ReverseHandler("fixture"))
	defer server.Close()
	peer, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/onebot/v11/ws", http.Header{"Authorization": {"Bearer fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	var writeMu sync.Mutex
	write := func(value any) error { writeMu.Lock(); defer writeMu.Unlock(); return peer.WriteJSON(value) }
	if err := write(map[string]any{"post_type": "meta_event", "meta_event_type": "lifecycle", "self_id": 1}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("bot did not connect")
	}
	replies := make(chan bot.Message, 5)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			var req struct {
				Action, Echo string
				Params       map[string]json.RawMessage
			}
			if peer.ReadJSON(&req) != nil {
				return
			}
			if req.Action == "get_group_info" {
				if write(map[string]any{"echo": req.Echo, "status": "ok", "retcode": 0, "data": map[string]any{"group_id": 100, "group_name": "测试群", "member_count": 2}}) != nil {
					return
				}
				continue
			}
			if req.Action == "get_group_member_info" {
				var userID int64
				if err := json.Unmarshal(req.Params["user_id"], &userID); err != nil {
					t.Error(err)
					return
				}
				if write(map[string]any{"echo": req.Echo, "status": "ok", "retcode": 0, "data": bot.Sender{UserID: userID, Nickname: "QQ昵称", Card: "群内侠客"}}) != nil {
					return
				}
				continue
			}
			if req.Action != "send_group_msg" {
				t.Errorf("unexpected external operation %s", req.Action)
				return
			}

			var message bot.Message
			if err := json.Unmarshal(req.Params["message"], &message); err != nil {
				t.Error(err)
				return
			}
			if write(map[string]any{"echo": req.Echo, "status": "ok", "retcode": 0, "data": map[string]any{"message_id": 900}}) != nil {
				return
			}
			replies <- message
		}
	}()
	defer func() { peer.Close(); <-done }()
	send := func(user int64, id int32, message bot.Message, expectImage bool) bot.Message {
		t.Helper()
		if err := write(map[string]any{"post_type": "message", "message_type": "group", "self_id": 1, "group_id": 100, "user_id": user, "message_id": id, "message": message, "sender": bot.Sender{Nickname: "测试侠客", Role: "member"}}); err != nil {
			t.Fatal(err)
		}
		select {
		case reply := <-replies:
			if !expectImage {
				if reply.HasType("image") || reply.Text() == "" {
					t.Fatalf("expected textual options or guidance: %v", reply)
				}
				return reply
			}
			if !reply.HasType("image") {
				t.Fatalf("missing rendered card: %s", reply.Text())
			}
			for _, seg := range reply {
				if seg.Type == "image" {
					var v struct {
						File string `json:"file"`
					}
					_ = json.Unmarshal(seg.Data, &v)
					raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(v.File, "base64://"))
					if err != nil {
						t.Fatal(err)
					}
					img, format, err := image.DecodeConfig(bytes.NewReader(raw))
					if err != nil || img.Width != 1200 {
						t.Fatalf("invalid card %v %+v", err, img)
					}
					if dir := os.Getenv("YUELING_WUXIA_PREVIEW_DIR"); dir != "" {
						if err := os.MkdirAll(dir, 0755); err != nil {
							t.Fatal(err)
						}
						for _, ext := range []string{"png", "jpeg"} {
							if ext != format {
								err := os.Remove(filepath.Join(dir, fmt.Sprintf("sent-%d.%s", id, ext)))
								if err != nil && !os.IsNotExist(err) {
									t.Fatal(err)
								}
							}
						}
						if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("sent-%d.%s", id, format)), raw, 0644); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			return reply
		case <-time.After(30 * time.Second):
			t.Fatal("command did not return")
		}
		return nil
	}
	send(10, 11, bot.Msg().Text("江湖").Build(), true)
	send(10, 11, bot.Msg().Text("江湖").Build(), true)
	send(10, 13, bot.Msg().At(1).Text(" 切磋 ").At(20).Build(), true)
	target, daily, err := service.Profile(context.Background(), 100, 20)
	if err != nil || target.Character.Name != "群内侠客" || target.Character.Silver != 0 || daily.Signed || daily.Runs != 0 {
		t.Fatalf("invalid new opponent: %+v %+v %v", target, daily, err)
	}
	send(10, 13, bot.Msg().At(1).Text(" 切磋 ").At(20).Build(), true)
	send(20, 12, bot.Msg().Text("江湖").Build(), true)
	send(10, 14, bot.Msg().Text("我的侠客").Build(), true)
	send(10, 15, bot.Msg().Text("江湖转职 狂刀").Build(), true)
	send(10, 16, bot.Msg().Text("江湖形象 3").Build(), true)
	send(10, 17, bot.Msg().Text("江湖武器 剑").Build(), true)
	send(10, 18, bot.Msg().Text("江湖练武").Build(), true)
	before, _, err := service.Profile(context.Background(), 100, 10)
	if err != nil {
		t.Fatal(err)
	}
	send(10, 19, bot.Msg().At(1).Text(" 江湖战报").Build(), true)
	after, _, err := service.Profile(context.Background(), 100, 10)
	if err != nil || after.Character.Silver != before.Character.Silver || after.Character.Mastery != 1 || after.Profession.ID != "blade" || after.Fighter().Portrait != "boxer" || after.Weapon.Kind != "刀" {
		t.Fatalf("journey state inconsistent: %+v %v", after, err)
	}
	send(10, 20, bot.Msg().Text("江湖转职").Build(), true)
	send(10, 21, bot.Msg().Text("江湖帮助").Build(), true)
	send(10, 22, bot.Msg().Text("江湖榜").Build(), true)
	send(10, 23, bot.Msg().Text("切磋 ").At(10).Build(), true)
	prepared, _, err := service.Profile(context.Background(), 100, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Catalog().Grant(&prepared.Character, []game.Reward{{ItemID: "manual-blade-rift", Quantity: 1}, {ItemID: "armor-cloth", Quantity: 1}}); err != nil {
		t.Fatal(err)
	}
	prepared.Character.Silver = 1000
	if err := database.Save(&prepared.Character).Error; err != nil {
		t.Fatal(err)
	}
	send(10, 24, bot.Msg().Text("江湖奖池").Build(), true)
	send(10, 25, bot.Msg().Text("江湖寻宝 5").Build(), true)
	send(10, 26, bot.Msg().Text("江湖背包").Build(), true)
	send(10, 27, bot.Msg().Text("江湖装备 青布劲装").Build(), true)
	send(10, 28, bot.Msg().Text("江湖学艺 断岳刀诀秘籍").Build(), true)
	send(10, 29, bot.Msg().Text("江湖武学").Build(), true)
	send(10, 30, bot.Msg().Text("江湖武学 断岳刀诀").Build(), true)
	send(10, 31, bot.Msg().Text("我的侠客").Build(), true)
	send(10, 25, bot.Msg().Text("江湖寻宝 5").Build(), true)
	expanded, _, err := service.Profile(context.Background(), 100, 10)
	if err != nil || expanded.Character.Silver < 700 || expanded.Character.Draws["jianghu"] != 5 || expanded.Skill == nil || expanded.Skill.ID != "blade-rift" || expanded.ArmorItem == nil || expanded.ArmorItem.ID != "armor-cloth" {
		t.Fatalf("expansion journey invalid: %+v %v", expanded, err)
	}
	send(10, 32, bot.Msg().Text("江湖赌场帮助").Build(), true)
	send(10, 33, bot.Msg().Text("江湖赌场 100").Build(), true)
	wagered, _, err := service.Profile(context.Background(), 100, 10)
	if err != nil {
		t.Fatal(err)
	}
	send(10, 33, bot.Msg().Text("江湖赌场 100").Build(), true)
	replayed, _, err := service.Profile(context.Background(), 100, 10)
	if err != nil || wagered.Character.Silver != replayed.Character.Silver {
		t.Fatal("casino replay changed balance")
	}
	send(10, 34, bot.Msg().Text("江湖赌场").Build(), true)
	if competing.Load() != 0 {
		t.Fatalf("game command reached AI/catch-all %d times", competing.Load())
	}
}
