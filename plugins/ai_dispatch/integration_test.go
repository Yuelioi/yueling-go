package ai_dispatch

import (
	"encoding/json"
	"fmt"
	"github.com/Yuelioi/yueling-go/internal/testconfig"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/gorilla/websocket"
	openai "github.com/sashabaranov/go-openai"
)

// Uses the actual WebSocket event handler, dispatch, registered history tool,
// model transport, database context, and OneBot reply path. No real QQ service.
func TestSummaryThroughWebSocketAndNativeModel(t *testing.T) {
	runSummaryIntegration(t, false, false)
}

func TestDatedSummaryThroughWebSocketAndDatabase(t *testing.T) {
	runSummaryIntegration(t, true, false)
}

func TestSummaryThroughWebSocketUnderSmallOutputBudget(t *testing.T) {
	runSummaryIntegration(t, false, true)
}

func runSummaryIntegration(t *testing.T, withDatabase, smallBudget bool) {
	oldConfig, oldSessions, oldDB := config.C, testEngine.Sessions, testdb.Database
	t.Cleanup(func() { config.C = oldConfig; testEngine.Sessions = oldSessions; testdb.Database = oldDB })
	if withDatabase {
		testdb.Init(t)
		oldAffinity := testEngine.Affinity
		testEngine.Affinity = ai.NewAffinityManager(db.NewAffinityStore(testdb.Database))
		t.Cleanup(func() { testEngine.Affinity = oldAffinity })
	} else {
		testdb.Database = nil
	}
	oldGroups := testEngine.Groups
	testEngine.Groups = db.NewGroupStore(testdb.Database)
	t.Cleanup(func() { testEngine.Groups = oldGroups })
	oldChats, oldMemory := testEngine.Chats, testEngine.Memory
	testEngine.Chats = db.NewChatStore(testdb.Database)
	testEngine.Memory = ai.NewMemoryManager(testdb.Database)
	t.Cleanup(func() { testEngine.Chats = oldChats; testEngine.Memory = oldMemory })
	testEngine.Sessions = &ai.SessionManager{}
	if withDatabase {
		if err := db.NewChatStore(testdb.Database).SaveGroupChatMessages([]db.GroupChatMessage{
			{GroupID: 100, MessageID: 280, UserID: 201, Nickname: "甲", Content: "数据库本群资料：周四完成测试", CreatedAt: bot.Now().Unix()},
			{GroupID: 999, MessageID: 280, UserID: 999, Nickname: "其他群", Content: "不可泄露的其他群资料", CreatedAt: bot.Now().Unix()},
		}); err != nil {
			t.Fatal(err)
		}
	}
	var requests atomic.Int32
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if len(req.Tools) != 0 || len(req.Messages) != 2 {
			t.Error("fixed summary unexpectedly entered tool-calling protocol")
		}
		if smallBudget && (req.MaxTokens != 300 || req.ReasoningEffort != "none" || !strings.Contains(req.Messages[0].Content, "400字")) {
			// Shape observed in the real-provider reproduction: successful HTTP,
			// exhausted generation budget, and no complete user-facing response.
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","reasoning_content":"synthetic reasoning"},"finish_reason":"length"}],"usage":{"completion_tokens":300}}`)
			return
		}
		material := req.Messages[len(req.Messages)-1].Content
		if strings.Contains(material, `"message_id"`) || strings.Contains(material, `"user_id"`) {
			t.Error("summary material exposed internal identifiers")
		}
		switch n {
		case 1:
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"讨论结论：周五发布，周四完成测试。"},"finish_reason":"stop"}]}`)
		case 2:
			if !strings.Contains(material, `"mode":"actions"`) {
				t.Error("follow-up did not change output mode")
			}
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"群聊待办：周四完成测试。"},"finish_reason":"stop"}]}`)
		case 3:
			if !strings.Contains(material, "数据库本群资料") || strings.Contains(material, "其他群资料") {
				t.Error("dated history was not isolated")
			}
			if !strings.Contains(material, `"mode":"actions"`) {
				t.Error("date follow-up lost task mode")
			}
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"今日待办：周四完成测试。"},"finish_reason":"stop"}]}`)
		default:
			t.Errorf("unexpected model request %d", n)
			w.WriteHeader(500)
		}
	}))
	defer model.Close()
	config.C.Bot.Name = "月灵"
	config.C.AI = testconfig.AI("test", model.URL, "fixture")
	if smallBudget {
		testconfig.Set(&config.C.AI, "Model", "deepseek-v4-pro")
		config.C.AI.MaxTokens = 300
		config.C.AI.ReplyMaxChars = 200
	}
	b := bot.New()
	t.Cleanup(func() { b.Stop(); b.Wait() })
	Register(b, testEngine)
	var competing atomic.Int32
	b.OnCommand("总结群聊").Handle(func(*bot.CommandContext) error { competing.Add(1); return nil })
	b.OnGroupMessage().Priority(0).Handle(func(*bot.GroupContext) error { competing.Add(1); return nil })
	server := httptest.NewServer(b.ReverseHandler("fixture-token"))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/onebot/v11/ws", http.Header{"Authorization": []string{"Bearer fixture-token"}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(map[string]any{"post_type": "meta_event", "meta_event_type": "lifecycle", "self_id": 42}); err != nil {
		t.Fatal(err)
	}
	historyCalls := 0
	turns := 2
	if withDatabase {
		turns = 4
	}
	for turn := 0; turn < turns; turn++ {
		input := "总结群聊"
		wantReply := "讨论结论：周五发布，周四完成测试。"
		if turn == 1 {
			input = "只列待办"
			wantReply = "群聊待办：周四完成测试。"
		}
		if turn == 2 {
			input = "今天呢"
			wantReply = "今日待办：周四完成测试。"
		}
		if turn == 3 {
			if err := db.NewGroupStore(testdb.Database).SetGroupPluginDisabled(100, catalog.PluginDailyDigest, true); err != nil {
				t.Fatal(err)
			}
			wantReply = "当前群聊或你的权限未开放群聊总结。"
		}
		event := map[string]any{"post_type": "message", "message_type": "group", "self_id": 42, "group_id": 100, "user_id": 200, "message_id": 300 + turn, "sender": map[string]any{"user_id": 200, "nickname": "测试用户", "role": "member"}, "message": bot.Msg().At(42).Text(input).Build()}
		if turn == 1 {
			event["message"] = bot.Msg().Text("月灵 " + input).Build()
		}
		if err := conn.WriteJSON(event); err != nil {
			t.Fatal(err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		for {
			var action struct {
				Action string `json:"action"`
				Echo   string `json:"echo"`
				Params struct {
					GroupID int64       `json:"group_id"`
					Message bot.Message `json:"message"`
				} `json:"params"`
			}
			if err := conn.ReadJSON(&action); err != nil {
				t.Fatal(err)
			}
			if action.Action != "set_msg_emoji_like" && action.Params.GroupID != 100 {
				t.Fatalf("wrong group: %d", action.Params.GroupID)
			}
			var data any = map[string]any{"message_id": 900 + turn}
			switch action.Action {
			case "set_msg_emoji_like":
			case "get_group_info":
				data = map[string]any{"group_id": 100, "group_name": "测试群"}
			case "get_group_msg_history":
				historyCalls++
				data = map[string]any{"messages": []any{map[string]any{"message_id": 290, "user_id": 201, "sender": map[string]any{"nickname": "甲"}, "message": bot.Msg().Text("周五发布，周四完成测试").Build()}}}
			case "send_group_msg":
				if got := action.Params.Message.Text(); got != wantReply {
					t.Fatalf("unexpected reply=%q", got)
				}
			default:
				t.Fatalf("unexpected action %s", action.Action)
			}
			if err := conn.WriteJSON(map[string]any{"status": "ok", "retcode": 0, "data": data, "echo": action.Echo}); err != nil {
				t.Fatal(err)
			}
			if action.Action == "send_group_msg" {

				break
			}
		}
	}
	b.Stop()
	b.Wait()
	if competing.Load() != 0 {
		t.Fatal("AI request also triggered another handler")
	}
	wantRequests := int32(2)
	if withDatabase {
		wantRequests = 3
	}
	if historyCalls != 2 || requests.Load() != wantRequests {
		t.Fatalf("history calls=%d model calls=%d", historyCalls, requests.Load())
	}
}
