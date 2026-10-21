package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestCasualConversationUsesRecentTurnsWithoutSummaryCalls(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	cfg.AI.Model = "fixture"
	config.Publish(cfg)
	calls := 0
	var lastInputChars, fullInputChars int
	var s *Session
	e := NewEngine(EngineDependencies{ModelBackend: intelligenceModel(func(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		calls++
		users := 0
		for _, msg := range req.Messages {
			if msg.Role == "user" {
				users++
			}
		}
		if users > 3 || strings.Contains(req.Messages[0].Content, "OLDER_SUMMARY") {
			t.Fatalf("casual request grew: users=%d", users)
		}
		if calls == 12 {
			data, _ := json.Marshal(req.Messages)
			lastInputChars = len([]rune(string(data)))
			full := append([]openai.ChatCompletionMessage{req.Messages[0]}, s.Messages...)
			data, _ = json.Marshal(full)
			fullInputChars = len([]rune(string(data)))
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: "stop", Message: openai.ChatCompletionMessage{Role: "assistant", Content: "接着聊", ReasoningContent: "retained protocol"}}}}, nil
	})})
	s = e.Sessions.Get(100, 1)
	s.historySummary = "OLDER_SUMMARY"
	for i := 0; i < 12; i++ {
		d := newToolDiscovery(nil, map[string]bool{})
		identity := turnIdentity(&bot.GroupMessageEvent{GroupID: 100, UserID: int64(i%2 + 1)})
		if _, err := e.runConversation(context.Background(), s, fmt.Sprintf("第%d句：%s", i, strings.Repeat("聊天", 20))+identity, d.prompt(), nil, func(openai.ToolCall) ToolResult {
			t.Fatal("chat unexpectedly executed a tool")
			return ToolResult{}
		}, d); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 12 || len(s.Messages) <= 6 || e.Sessions.Get(100, 2) != s || len(e.Sessions.Get(101, 1).Messages) != 0 {
		t.Fatalf("calls=%d retained_messages=%d", calls, len(s.Messages))
	}
	if lastInputChars >= fullInputChars {
		t.Fatalf("history projection did not shrink input: sent=%d full=%d", lastInputChars, fullInputChars)
	}
	t.Logf("12 casual turns: model_calls=%d last_request_chars=%d full_history_request_chars=%d (character counts, not tokens)", calls, lastInputChars, fullInputChars)
}

func TestConversationRecallsEarlierHistoryWithoutExecutingActions(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	cfg.AI.Model = "fixture"
	config.Publish(cfg)
	s := newSession(1, 100)
	for i := 0; i < 5; i++ {
		s.pushUser(fmt.Sprintf("speaker_%d：%s", i, []string{"OLD_TARGET", "第二个话题", "第三个话题", "最近的话题", "另一个话题"}[i]))
		s.pushAssistant(openai.ChatCompletionMessage{Role: "assistant", Content: "收到", ReasoningContent: "preserve protocol"})
	}
	s.historySummary = "OLD_AUTHORIZATION：尚未授权执行"
	saved := append([]openai.ChatCompletionMessage(nil), s.Messages...)
	d := newToolDiscovery(nil, map[string]bool{})
	calls := 0
	e := NewEngine(EngineDependencies{ModelBackend: intelligenceModel(func(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		calls++
		data, _ := json.Marshal(req.Messages)
		msg := openai.ChatCompletionMessage{Role: "assistant"}
		if calls == 1 {
			if strings.Contains(string(data), "OLD_TARGET") || strings.Contains(string(data), "OLD_AUTHORIZATION") {
				t.Fatal("older history included before recall")
			}
			msg.ToolCalls = []openai.ToolCall{{ID: "recall", Type: "function", Function: openai.FunctionCall{Name: discoveryName, Arguments: `{"history":true}`}}}
		} else {
			if !strings.Contains(string(data), "OLD_TARGET") || !strings.Contains(req.Messages[0].Content, "OLD_AUTHORIZATION") || !reflect.DeepEqual(saved, req.Messages[1:len(saved)+1]) {
				t.Fatal("recall lost earlier speakers, authorization or reasoning")
			}
			msg.Content = "找到了之前的目标，仍需确认。"
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: "stop", Message: msg}}}, nil
	})})
	if _, err := e.runConversation(context.Background(), s, "更早那个目标是什么", d.prompt(), nil, func(openai.ToolCall) ToolResult {
		t.Fatal("history recall invoked a business action")
		return ToolResult{}
	}, d); err != nil || calls != 2 || !reflect.DeepEqual(saved, s.Messages[:len(saved)]) {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestRecentContextKeepsOversizedToolTurnWhole(t *testing.T) {
	s := newSession(1, 2)
	s.pushUser("较早闲聊")
	s.pushAssistant(openai.ChatCompletionMessage{Role: "assistant", Content: "闲聊回答"})
	s.pushUser("读取资料")
	s.pushAssistant(openai.ChatCompletionMessage{Role: "assistant", ReasoningContent: "required reasoning", ToolCalls: []openai.ToolCall{{ID: "read", Function: openai.FunctionCall{Name: "read", Arguments: "{}"}}}})
	s.pushToolResult("read", strings.Repeat("资料", 2000))
	s.pushAssistant(openai.ChatCompletionMessage{Role: "assistant", Content: "资料已读取"})
	start := s.recentHistoryStart()
	if start != 2 || s.Messages[start+1].ReasoningContent != "required reasoning" || s.Messages[start+2].ToolCallID != "read" {
		t.Fatalf("partial tool turn: start=%d", start)
	}
}

func TestDiscoveryReturnsOnlyNewSchemasAndValidatesRecallAtomically(t *testing.T) {
	a := &ToolMeta{Name: "a", Description: "简短用途。完整限制不能在目录中重复", Params: []Param{{Name: "value", Type: "string"}}}
	b := &ToolMeta{Name: "b", Description: "另一个能力", Requires: []string{"a"}}
	d := newToolDiscovery([]*ToolMeta{a, b}, map[string]bool{})
	if d.loadedContext() != "" || strings.Contains(d.prompt(), "完整限制") {
		t.Fatal("idle prompt includes unused definitions")
	}
	if d.discover(`{"names":["a"]}`).Status != ToolSucceeded {
		t.Fatal("first discovery failed")
	}
	result := d.discover(`{"names":["b"]}`)
	if result.Status != ToolSucceeded || strings.Contains(result.Content, `"name":"a"`) || !strings.Contains(result.Content, `"name":"b"`) {
		t.Fatal("discovery repeated already loaded schemas")
	}
	d = newToolDiscovery([]*ToolMeta{a}, map[string]bool{})
	if d.discover(`{"names":["forbidden"],"history":true}`).Status != ToolRejected || d.recallHistory {
		t.Fatal("rejected discovery changed history projection")
	}
	if d.discover(`{"history":true}`).Status != ToolSucceeded || !d.recallHistory {
		t.Fatal("local recall should be available without business tools")
	}
}

func TestInitialDefinitionsReuseOnlyMatchingSchemasInSelectedHistory(t *testing.T) {
	meta := &ToolMeta{Name: "reminder", Description: "提醒", Params: []Param{{Name: "time", Type: "string", Required: true}}}
	d := newToolDiscovery([]*ToolMeta{meta}, map[string]bool{"reminder": true})
	definition := d.loadedContext()
	for _, message := range []openai.ChatCompletionMessage{
		{Role: "user", Content: definition + "明天提醒我"},
		{Role: "tool", Content: (ToolResult{Status: ToolSucceeded, Content: definition}).modelContent()},
	} {
		if got := d.loadedContextWithout([]openai.ChatCompletionMessage{message}); got != "" {
			t.Fatal("same schema was repeated in the recent window")
		}
		meta.Params[0].Description = "使用绝对时间"
		if got := d.loadedContextWithout([]openai.ChatCompletionMessage{message}); !strings.Contains(got, "使用绝对时间") {
			t.Fatal("changed schema was hidden behind an old definition")
		}
		meta.Params[0].Description = ""
	}
	if got := d.loadedContextWithout(nil); !strings.Contains(got, `"name":"reminder"`) {
		t.Fatal("schema did not return after the defining turn left the window")
	}
}
