package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/internal/testdb"
	openai "github.com/sashabaranov/go-openai"
)

func TestFollowupRoutesKeepOnlyCurrentlyAllowedCandidates(t *testing.T) {
	reminder := &ToolMeta{Name: "reminder", Triggers: []string{"提醒"}}
	weather := &ToolMeta{Name: "weather", Triggers: []string{"天气"}}
	session := newSession(1, 2)
	allowed := []*ToolMeta{reminder, weather}
	for _, step := range []struct {
		text    string
		allowed []*ToolMeta
		want    string
	}{
		{"提醒我喝水", allowed, "reminder"},
		{"明天八点", allowed, "reminder"},
		{"天气怎么样", allowed, "weather"},
		{"北京", allowed, "weather"},
		{"继续", []*ToolMeta{reminder}, ""},
		{"继续", allowed, ""},
	} {
		got := session.routeTools(step.text, step.allowed)
		if step.want == "" {
			if len(got) != 0 {
				t.Fatalf("%s: unexpected tools %v", step.text, got)
			}
			continue
		}
		if len(got) != 1 || got[0].Tool.Name != step.want {
			t.Fatalf("%s: tools=%v want=%s", step.text, got, step.want)
		}
		session.resetTurn()
	}
	if got := newSession(1, 2).routeTools("明天八点", allowed); len(got) != 0 {
		t.Fatal("new session inherited tools")
	}
}

func TestRouteIncludesAllowedPrerequisites(t *testing.T) {
	history := &ToolMeta{Name: "history"}
	revoke := &ToolMeta{Name: "revoke", Triggers: []string{"撤回"}, Requires: []string{"history"}}
	s := newSession(1, 2)
	got := s.routeTools("撤回刚才那条", []*ToolMeta{revoke, history})
	if len(got) != 2 || got[1].Tool.Name != "history" {
		t.Fatalf("missing prerequisite: %v", got)
	}
	got = s.routeTools("撤回刚才那条", []*ToolMeta{revoke})
	if len(got) != 1 {
		t.Fatal("exposed disabled prerequisite")
	}
}

func TestRouteDependencyCycleIsBoundedAndDeduplicated(t *testing.T) {
	a := &ToolMeta{Name: "a", Triggers: []string{"start"}, Requires: []string{"b"}}
	b := &ToolMeta{Name: "b", Requires: []string{"a"}}
	got := newSession(1, 2).routeTools("start", []*ToolMeta{a, b})
	if len(got) != 2 {
		t.Fatalf("cycle: %+v", got)
	}
}

func TestClarificationThroughDispatch(t *testing.T) {
	oldConfig, oldDB, oldRegistry, oldSessions, oldClient := config.Current(), testdb.Database, testEngine.Registry, testEngine.Sessions, testEngine.modelBackend
	t.Cleanup(func() {
		config.Publish(oldConfig)
		testdb.Database = oldDB
		testEngine.Registry = oldRegistry
		testEngine.Sessions = oldSessions
		testEngine.modelBackend = oldClient
	})
	testdb.Database = nil
	testEngine.Registry = &Registry{tools: map[string]*ToolMeta{}}
	testEngine.Sessions = &SessionManager{}
	testEngine.modelBackend = nil
	resetAILimiterForTest(t)
	calls := 0
	testEngine.Registry.Register(ToolMeta{Name: "schedule_fixture", Triggers: []string{"提醒"}, ResultHandler: func(*ToolContext) ToolResult {
		calls++
		return ToolResult{Status: ToolSucceeded, Content: "已安排", PossibleSideEffects: true}
	}})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if len(req.Tools) != 1 || req.Tools[0].Function.Name != "schedule_fixture" {
			t.Errorf("clarification lost candidate: %+v", req.Tools)
		}
		msg := openai.ChatCompletionMessage{Role: "assistant", Content: "几点？"}
		if requests.Add(1) == 2 {
			msg.Content = ""
			msg.ToolCalls = []openai.ToolCall{{ID: "schedule", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "schedule_fixture", Arguments: `{}`}}}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: msg, FinishReason: openai.FinishReasonStop}}})
	}))
	defer server.Close()
	cfg := oldConfig
	cfg.AI = config.AIConfig{DeepSeekKey: "fixture", BaseURL: server.URL, Model: "fixture"}
	config.Publish(cfg)
	for i, text := range []string{"提醒我喝水", "明天八点"} {
		ctx, cancel := context.WithCancel(context.Background())
		event := &bot.GroupMessageEvent{UserID: 882010, GroupID: 882010, MessageID: int32(i + 1), Message: bot.Msg().Text(text).Build()}
		err := testEngine.Dispatch(ctx, &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { cancel(); return nil })
		cancel()
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 || requests.Load() != 3 {
		t.Fatalf("actions=%d requests=%d", calls, requests.Load())
	}
}

func TestDispatchRechecksSuperuserBeforeAction(t *testing.T) {
	oldConfig, oldDB, oldRegistry, oldSessions, oldClient := config.Current(), testdb.Database, testEngine.Registry, testEngine.Sessions, testEngine.modelBackend
	t.Cleanup(func() {
		config.Publish(oldConfig)
		testdb.Database = oldDB
		testEngine.Registry = oldRegistry
		testEngine.Sessions = oldSessions
		testEngine.modelBackend = oldClient
	})
	testdb.Database = nil
	testEngine.Registry = &Registry{tools: map[string]*ToolMeta{}}
	testEngine.Sessions = &SessionManager{}
	testEngine.modelBackend = nil
	resetAILimiterForTest(t)
	testEngine.Registry.Register(ToolMeta{Name: "admin_fixture", Triggers: []string{"管理测试"}, Permission: PermSuperUser, ResultHandler: func(*ToolContext) ToolResult {
		t.Error("revoked user executed action")
		return ToolResult{Status: ToolSucceeded}
	}})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		msg := openai.ChatCompletionMessage{Role: "assistant", Content: "权限已变更"}
		if requests.Add(1) == 1 {
			if len(req.Tools) != 1 {
				t.Error("tool not exposed to initial superuser")
			}
			cfg := config.Current()
			cfg.Bot.SuperUsers = nil
			cfg.Bot.OwnerID = 0
			config.Publish(cfg)
			msg.Content = ""
			msg.ToolCalls = []openai.ToolCall{{ID: "admin", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "admin_fixture", Arguments: `{}`}}}
		} else {
			var result ToolResult
			if err := json.Unmarshal([]byte(req.Messages[len(req.Messages)-1].Content), &result); err != nil || result.Status != ToolRejected || result.PossibleSideEffects {
				t.Errorf("revocation outcome=%+v err=%v", result, err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: msg, FinishReason: openai.FinishReasonStop}}})
	}))
	defer server.Close()
	cfg := oldConfig
	cfg.Bot.SuperUsers = []int64{882011}
	cfg.AI = config.AIConfig{DeepSeekKey: "fixture", BaseURL: server.URL, Model: "fixture"}
	config.Publish(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	event := &bot.GroupMessageEvent{UserID: 882011, GroupID: 882011, MessageID: 1, Message: bot.Msg().Text("管理测试").Build()}
	if err := testEngine.Dispatch(ctx, &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { cancel(); return nil }); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 {
		t.Fatalf("requests=%d", requests.Load())
	}
}
