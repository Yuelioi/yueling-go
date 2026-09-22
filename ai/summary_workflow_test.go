package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/services/chatsummary"
	openai "github.com/sashabaranov/go-openai"
)

type summaryReaderFunc func(context.Context, chatsummary.Query) (chatsummary.Source, error)

func (f summaryReaderFunc) Read(ctx context.Context, q chatsummary.Query) (chatsummary.Source, error) {
	return f(ctx, q)
}

func summaryGroup(input string) *bot.GroupContext {
	return &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: &bot.GroupMessageEvent{GroupID: 100, UserID: 42, MessageID: 300, Sender: bot.Sender{Role: "member"}, Message: bot.Msg().Text(input).Build()}}}
}

func TestSummaryRequestGrammarPreservesOrRejectsParameters(t *testing.T) {
	oldName := config.C.Bot.Name
	config.C.Bot.Name = "月灵"
	t.Cleanup(func() { config.C.Bot.Name = oldName })
	prior := &SummaryTask{Query: chatsummary.Query{Count: 50, Period: "today", Mode: "summary", Focus: "发布"}}
	for _, test := range []struct {
		input               string
		prior               *SummaryTask
		matched             bool
		hint                bool
		period, mode, focus string
		count               int
	}{
		{"总结群聊", nil, true, false, "recent", "summary", "", 30},
		{"总结昨天的讨论", nil, true, false, "yesterday", "summary", "", 30},
		{"提取今天的群聊待办", nil, true, false, "today", "actions", "", 30},
		{"总结今天群聊，只列待办", nil, true, false, "today", "actions", "", 30},
		{"总结最近50条群聊", nil, true, false, "recent", "summary", "", 50},
		{"今天群里聊了什么", nil, true, false, "today", "summary", "", 30},
		{"总结近7天群聊", nil, true, false, "7days", "summary", "", 30},
		{"总结过去7天群聊", nil, true, false, "7days", "summary", "", 30},
		{"总结最近20条群聊，关注发布", nil, true, false, "recent", "summary", "发布", 20},
		{"昨天呢", prior, true, false, "yesterday", "summary", "发布", 50},
		{"只列待办", prior, true, false, "today", "actions", "发布", 50},
		{"昨天呢", nil, false, false, "", "", "", 0},
		{"只列待办", nil, false, false, "", "", "", 0},
		{"总结群聊1000条", nil, true, true, "", "", "", 0},
		{"总结全部1000条群聊", nil, true, true, "", "", "", 0},
		{"总结今天全部群聊", nil, true, true, "", "", "", 0},
		{"总结群聊0条", nil, true, true, "", "", "", 0},
		{"总结上周群聊", nil, true, true, "", "", "", 0},
		{"总结过去30天群聊", nil, true, true, "", "", "", 0},
		{"总结今天和昨天的群聊", nil, true, true, "", "", "", 0},
		{"总结群聊并创建提醒", nil, false, false, "", "", "", 0},
		{"总结以下群聊：甲：周五发布", nil, false, false, "", "", "", 0},
		{"帮我总结这段文章", nil, false, false, "", "", "", 0},
		{"总结群聊关于发布", nil, false, false, "", "", "", 0},
		{"总结群聊里的全部一万条消息", nil, true, true, "", "", "", 0},
	} {
		t.Run(test.input, func(t *testing.T) {
			q, matched, hint := parseSummaryRequest(test.input, test.prior, 30)
			if matched != test.matched || (hint != "") != test.hint {
				t.Fatalf("query=%+v matched=%t hint=%q", q, matched, hint)
			}
			if matched && !test.hint && (q.Period != test.period || q.Mode != test.mode || q.Focus != test.focus || q.Count != test.count) {
				t.Fatalf("query=%+v", q)
			}
		})
	}
}

// The reader and model are synthetic; this verifies orchestration, not model quality.
func TestSummaryWorkflowRereadsFollowupWithoutProviderHistory(t *testing.T) {
	session := newSession(42, 100)
	session.Messages = []openai.ChatCompletionMessage{{Role: "assistant", Content: "previous general answer", ReasoningContent: "exact provider reasoning"}}
	original := append([]openai.ChatCompletionMessage(nil), session.Messages...)
	allowed := []*ToolMeta{{Name: "summarize_chat", ReadOnly: true}}
	var queries []chatsummary.Query
	reader := summaryReaderFunc(func(_ context.Context, q chatsummary.Query) (chatsummary.Source, error) {
		queries = append(queries, q)
		return chatsummary.Source{Scope: q.Period + " sample", Records: []chatsummary.Record{{MessageID: 101, Text: q.Period + " source"}}}, nil
	})
	modelCalls := 0
	complete := func(_ context.Context, req openai.ChatCompletionRequest) (string, error) {
		modelCalls++
		if len(req.Tools) != 0 || len(req.Messages) != 2 {
			t.Fatal("fixed workflow entered tool-selection loop")
		}
		q := queries[len(queries)-1]
		if !strings.Contains(req.Messages[1].Content, q.Period+" source") {
			t.Fatal("reused old source")
		}
		return fmt.Sprintf("%s %s [101]", q.Period, q.Mode), nil
	}
	for _, input := range []string{"总结群聊", "昨天呢", "只列待办"} {
		reply, handled, err := runSummaryWorkflowWith(context.Background(), summaryGroup(input), session, allowed, input, reader, complete)
		if err != nil || !handled || reply == "" {
			t.Fatalf("input=%q reply=%q handled=%t err=%v", input, reply, handled, err)
		}
	}
	if len(queries) != 3 || modelCalls != 3 || queries[1].Period != "yesterday" || queries[2].Period != "yesterday" || queries[2].Mode != "actions" {
		t.Fatalf("queries=%+v modelCalls=%d", queries, modelCalls)
	}
	if !reflect.DeepEqual(session.Messages, original) {
		t.Fatal("workflow fabricated or altered provider transcript")
	}
	if !strings.Contains(session.summaryContext(), "yesterday actions") {
		t.Fatal("generic follow-up lost summary reply")
	}
}

func TestSummaryWorkflowDoesNotCallModelForMissingDataOrDeniedAccess(t *testing.T) {
	for _, test := range []struct {
		name        string
		allowed     []*ToolMeta
		sourceError error
		wantRead    bool
	}{
		{"empty", []*ToolMeta{{Name: "summarize_chat"}}, nil, true},
		{"unavailable", []*ToolMeta{{Name: "summarize_chat"}}, chatsummary.ErrUnavailable, true},
		{"not_allowed", nil, nil, false},
		{"permission", []*ToolMeta{{Name: "summarize_chat", Permission: PermAdmin}}, nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			reads := 0
			reader := summaryReaderFunc(func(context.Context, chatsummary.Query) (chatsummary.Source, error) {
				reads++
				return chatsummary.Source{}, test.sourceError
			})
			complete := func(context.Context, openai.ChatCompletionRequest) (string, error) {
				t.Fatal("model called without authorized source records")
				return "", nil
			}
			reply, handled, err := runSummaryWorkflowWith(context.Background(), summaryGroup("总结群聊"), newSession(42, 100), test.allowed, "总结群聊", reader, complete)
			if !handled || reply == "" || (reads > 0) != test.wantRead {
				t.Fatalf("reply=%q handled=%t reads=%d err=%v", reply, handled, reads, err)
			}
			if test.sourceError != nil && !errors.Is(err, test.sourceError) {
				t.Fatalf("lost error: %v", err)
			}
		})
	}
}

func TestSummaryWorkflowDoesNotGenerateFromBlankSourceRecords(t *testing.T) {
	reader := summaryReaderFunc(func(context.Context, chatsummary.Query) (chatsummary.Source, error) {
		return chatsummary.Source{Records: []chatsummary.Record{{MessageID: 1, Text: " \n\t "}}}, nil
	})
	complete := func(context.Context, openai.ChatCompletionRequest) (string, error) {
		t.Fatal("model received a material payload with no usable records")
		return "", nil
	}
	reply, handled, err := runSummaryWorkflowWith(context.Background(), summaryGroup("总结群聊"), newSession(42, 100), []*ToolMeta{{Name: "summarize_chat"}}, "总结群聊", reader, complete)
	if err != nil || !handled || !strings.Contains(reply, "暂无") {
		t.Fatalf("reply=%q handled=%v err=%v", reply, handled, err)
	}
}

func TestSummaryWorkflowResetDiscardsLateModelReply(t *testing.T) {
	manager := &SessionManager{}
	session := manager.Get(100, 42)
	ctx, stop := session.turnContext(context.Background())
	defer stop()
	reader := summaryReaderFunc(func(context.Context, chatsummary.Query) (chatsummary.Source, error) {
		return chatsummary.Source{Scope: "sample", Records: []chatsummary.Record{{MessageID: 1, Text: "source"}}}, nil
	})
	started := make(chan struct{})
	complete := func(ctx context.Context, _ openai.ChatCompletionRequest) (string, error) {
		close(started)
		<-ctx.Done()
		return "late reply", nil
	}
	done := make(chan string, 1)
	go func() {
		reply, _, _ := runSummaryWorkflowWith(ctx, summaryGroup("总结群聊"), session, []*ToolMeta{{Name: "summarize_chat"}}, "总结群聊", reader, complete)
		done <- reply
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("model did not start")
	}
	manager.Delete(100, 42)
	select {
	case reply := <-done:
		if reply != "" {
			t.Fatalf("stale reply=%q", reply)
		}
	case <-time.After(time.Second):
		t.Fatal("summary did not stop")
	}
	if manager.Get(100, 42).SummaryTask != nil {
		t.Fatal("summary context crossed reset")
	}
}

func TestSummaryTaskSurvivesElaborationButEndsForNewSubject(t *testing.T) {
	for _, test := range []struct {
		input   string
		segment string
		keep    bool
	}{
		{input: "详细解释一下", keep: true},
		{input: "月灵，请展开说说？", keep: true},
		{input: "详细解释一下量子力学"},
		{input: "今天上海天气怎么样"},
		{input: "详细解释一下", segment: "reply"},
		{input: "详细解释一下", segment: "image"},
	} {
		t.Run(test.input+test.segment, func(t *testing.T) {
			cleanupAIConfigAndDB(t)
			resetAILimiterForTest(t)
			oldClient, oldSessions, oldRegistry := _client, Sessions, global
			t.Cleanup(func() { _client, Sessions, global = oldClient, oldSessions, oldRegistry })
			config.C.Bot.Name = "月灵"
			config.C.AI = config.AIConfig{Model: "fixture"}
			db.DB = nil
			Sessions = &SessionManager{}
			global = &registry{tools: map[string]*ToolMeta{}}
			Register(ToolMeta{Name: "summarize_chat", ReadOnly: true})
			session := Sessions.Get(100, 42)
			var periods []string
			reader := summaryReaderFunc(func(_ context.Context, q chatsummary.Query) (chatsummary.Source, error) {
				periods = append(periods, q.Period)
				return chatsummary.Source{Scope: q.Period + " sample", Records: []chatsummary.Record{{MessageID: 101, Text: q.Period + " source"}}}, nil
			})
			complete := func(_ context.Context, req openai.ChatCompletionRequest) (string, error) {
				period := periods[len(periods)-1]
				if !strings.Contains(req.Messages[1].Content, period+" source") {
					t.Fatal("summary reused records from another period")
				}
				return period + " summary [101]", nil
			}
			if _, handled, err := runSummaryWorkflowWith(context.Background(), summaryGroup("总结今天群聊"), session, AllTools(), "总结今天群聊", reader, complete); err != nil || !handled {
				t.Fatalf("initial summary handled=%t err=%v", handled, err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"这是更详细的说明。"},"finish_reason":"stop"}]}`)
			}))
			defer server.Close()
			_client = NewClient("fixture", server.URL)
			gctx := summaryGroup(test.input)
			gctx.Event.MessageID++
			if test.segment != "" {
				gctx.Event.Message = append(gctx.Event.Message, bot.Segment{Type: test.segment})
			}
			requestCtx, finishRequest := context.WithCancel(context.Background())
			defer finishRequest()
			if err := Dispatch(requestCtx, gctx, func(context.Context, string) error {
				// Finish this transport request after delivery; background memory work
				// is outside this test and must not outlive its isolated global state.
				finishRequest()
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			reply, handled, err := runSummaryWorkflowWith(context.Background(), summaryGroup("昨天呢"), session, AllTools(), "昨天呢", reader, complete)
			if err != nil || handled != test.keep {
				t.Fatalf("date follow-up handled=%t want=%t err=%v", handled, test.keep, err)
			}
			if test.keep {
				if !reflect.DeepEqual(periods, []string{"today", "yesterday"}) || !strings.Contains(reply, "yesterday") {
					t.Fatalf("follow-up did not fetch yesterday: periods=%v reply=%q", periods, reply)
				}
			} else if len(periods) != 1 {
				t.Fatalf("unrelated subject triggered group history: periods=%v", periods)
			}
		})
	}
}
