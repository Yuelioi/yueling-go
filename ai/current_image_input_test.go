package ai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/gorilla/websocket"
)

func TestCurrentMentionAndQuoteTakePriorityOverOldImageTask(t *testing.T) {
	b := bot.New()
	defer func() { b.Stop(); b.Wait() }()
	connected := make(chan *bot.BotAPI, 1)
	b.OnConnect(func(api *bot.BotAPI) { connected <- api })
	server := httptest.NewServer(b.ReverseHandler("fixture"))
	defer server.Close()
	peer, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/onebot/v11/ws", http.Header{"Authorization": {"Bearer fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	if err := peer.WriteJSON(map[string]any{"post_type": "meta_event", "meta_event_type": "lifecycle", "self_id": 1}); err != nil {
		t.Fatal(err)
	}
	var api *bot.BotAPI
	select {
	case api = <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("OneBot not connected")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			var request struct{ Action, Echo string }
			if peer.ReadJSON(&request) != nil {
				return
			}
			if request.Action != "get_msg" {
				t.Errorf("unexpected action %s", request.Action)
			}
			// A quote's text and mentions are historical data, not current targets.
			message := bot.Msg().At(999).Text("上一版警车还缺今夕头像").Image("https://fixture.invalid/quoted.png").Build()
			if peer.WriteJSON(map[string]any{"echo": request.Echo, "status": "ok", "retcode": 0, "data": map[string]any{"message": message}}) != nil {
				return
			}
		}
	}()
	defer func() { peer.Close(); <-done }()
	s := newSession(42, 100)
	c := &ToolContext{session: s}
	avatar, err := c.UserAvatarImage(200)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		s.addImage(sessionImage{URL: "https://fixture.invalid/old.png", Kind: "generated", Source: "上一版警车", Caption: "犬警察牵绳"})
	}
	event := &bot.GroupMessageEvent{SelfID: 1, GroupID: 100, UserID: 42, Message: bot.Msg().Reply(900).Text("月灵 ").At(200).Text(" 把这个头像换成我引用的图片").Build()}
	c.event = event
	input := s.imageInput(&bot.GroupContext{BotAPI: api, MsgCtx: &bot.MsgCtx{Event: event}})
	currentAt, historyAt := strings.Index(input, "【本轮明确图片来源（优先）】"), strings.Index(input, "【历史图片（仅明确续接时使用）】")
	if currentAt < 0 || historyAt <= currentAt {
		t.Fatalf("current images were not separated ahead of the old task:\n%s", input)
	}
	if !strings.Contains(input, "月灵 [@200] 把这个头像换成我引用的图片") {
		t.Error("current @ position was stripped from the request")
	}
	current := input[currentAt:historyAt]
	if !strings.Contains(current, avatar+"：本轮@用户QQ200的头像") || !strings.Contains(current, "image_12：引用消息第1张") {
		t.Errorf("missing current mention avatar or quoted image:\n%s", current)
	}
	if strings.Contains(current, "上一版警车") || strings.Contains(current, "QQ999的头像") || strings.Contains(input, "https://") {
		t.Error("historical targets or raw URLs leaked into current image sources")
	}
	urls, err := c.ImageURLs([]string{avatar, "image_12"})
	if err != nil || len(urls) != 2 || urls[0] != bot.QQAvatarURL(200) || urls[1] != "https://fixture.invalid/quoted.png" {
		t.Fatalf("wrong current references %v: %v", urls, err)
	}
	if !strings.Contains(input, "当前明确的@对象、附件和引用优先于历史任务") {
		t.Error("missing current-target precedence rule")
	}
	if len(s.images) != 12 {
		t.Error("current avatar was duplicated or quoted mentions became targets")
	}
	next := s.imageInput(&bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: &bot.GroupMessageEvent{SelfID: 1, GroupID: 100, UserID: 43, Message: bot.Msg().Text("继续聊别的话题").Build()}}})
	if strings.Contains(next, "【本轮明确图片来源") || strings.Contains(next, "本轮@用户") {
		t.Error("previous mention leaked into next turn as a current target")
	}
}

func TestCurrentMentionRolesPreserveSentenceOrder(t *testing.T) {
	for _, ids := range [][2]int64{{200, 300}, {300, 200}} {
		message := bot.Msg().Text("把").At(ids[0]).Text("的头像换成").At(ids[1]).Text("的形象").Build()
		event := &bot.GroupMessageEvent{SelfID: 1, GroupID: 100, UserID: 42, Message: message}
		s := newSession(42, 100)
		input := s.imageInput(&bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}})
		if !strings.HasPrefix(input, message.Summary()) {
			t.Error("inline mention roles were lost or reordered")
		}
		urls, err := (&ToolContext{session: s, event: event}).ImageURLs([]string{"image_1", "image_2"})
		if err != nil || len(urls) != 2 || urls[0] != bot.QQAvatarURL(ids[0]) || urls[1] != bot.QQAvatarURL(ids[1]) {
			t.Errorf("mention order changed: %v %v", urls, err)
		}
	}
}
