package ai

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/chatsummary"
	openai "github.com/sashabaranov/go-openai"
)

const dsmlOutputFixture = `<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="invoke_available_tool">
<｜｜DSML｜｜ parameter name="arguments" string="true">{"image_id":"image_23","prompt":"private-image-prompt-marker"}</｜｜DSML｜｜ parameter>
<｜｜DSML｜｜ parameter name="name" string="true">edit_image</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

func TestDispatchNeverDeliversDSMLProtocolText(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	cfg.AI.Model = "fixture"
	config.Publish(cfg)
	calls := 0
	engine := NewEngine(EngineDependencies{ModelBackend: intelligenceModel(func(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		calls++
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: "assistant", Content: dsmlOutputFixture}, FinishReason: openai.FinishReasonStop}}}, nil
	})})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	event := &bot.GroupMessageEvent{SelfID: 1, GroupID: 100, UserID: 42, MessageID: 123, Message: bot.Msg().At(1).Text("帮我改这张图").Build(), Sender: bot.Sender{Role: "member"}}
	var replies []string
	if err := engine.Dispatch(ctx, &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}}, func(_ context.Context, reply string) error { replies = append(replies, reply); cancel(); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(replies) != 1 || strings.Contains(replies[0], "DSML") || strings.Contains(replies[0], "private-image-prompt-marker") {
		t.Fatal("internal DSML text reached the QQ delivery callback")
	}
	if calls != 2 {
		t.Fatalf("malformed text was not corrected once: calls=%d", calls)
	}
	for _, message := range engine.Sessions.Get(100, 42).Messages {
		if strings.Contains(message.Content, "DSML") {
			t.Fatal("invalid DSML reply persisted in shared history")
		}
	}
}

func TestDeliveryBoundaryDoesNotForwardDSML(t *testing.T) {
	var replies []string
	if err := sendTurnReply(context.Background(), nil, func(_ context.Context, reply string) error { replies = append(replies, reply); return nil }, dsmlOutputFixture); err != nil {
		t.Fatal(err)
	}
	if len(replies) != 1 || strings.Contains(replies[0], "DSML") || strings.Contains(replies[0], "private-image-prompt-marker") {
		t.Fatal("delivery boundary forwarded raw protocol text")
	}
}

func TestDSMLCorrectionDoesNotRepeatCompletedAction(t *testing.T) {
	for _, persistent := range []bool{false, true} {
		t.Run(map[bool]string{false: "recovered", true: "persistent"}[persistent], func(t *testing.T) {
			old := config.Current()
			t.Cleanup(func() { config.Publish(old) })
			cfg := old
			cfg.AI.Model = "fixture"
			config.Publish(cfg)
			calls, executed := 0, 0
			engine := NewEngine(EngineDependencies{ModelBackend: intelligenceModel(func(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
				calls++
				for _, msg := range req.Messages {
					if strings.Contains(msg.Content, "private-image-prompt-marker") {
						t.Error("malformed protocol output entered retry history")
					}
				}
				message := openai.ChatCompletionMessage{Role: "assistant", Content: "操作已完成。"}
				switch {
				case calls == 1 || calls == 3 && !persistent:
					message.Content = ""
					message.ToolCalls = []openai.ToolCall{{ID: fmt.Sprintf("action_%d", calls), Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "fixture_action", Arguments: `{}`}}}
				case calls == 2 || persistent:
					message.Content = dsmlOutputFixture
				}
				return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: message, FinishReason: openai.FinishReasonStop}}}, nil
			})})
			engine.Registry.Register(ToolMeta{Name: "fixture_action", Triggers: []string{"执行操作"}, ResultHandler: func(*ToolContext) ToolResult {
				executed++
				return ToolResult{Status: ToolSucceeded, Content: "已执行", PossibleSideEffects: true}
			}})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			event := &bot.GroupMessageEvent{SelfID: 1, GroupID: 100, UserID: 42, MessageID: 123, Message: bot.Msg().At(1).Text("执行操作").Build(), Sender: bot.Sender{Role: "member"}}
			var reply string
			if err := engine.Dispatch(ctx, &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}}, func(_ context.Context, text string) error { reply = text; cancel(); return nil }); err != nil {
				t.Fatal(err)
			}
			wantCalls := 4
			if persistent {
				wantCalls = 3
			}
			if executed != 1 || calls != wantCalls || strings.Contains(reply, "DSML") || strings.Contains(reply, "private-image-prompt-marker") {
				t.Fatalf("unsafe correction: executed=%d calls=%d", executed, calls)
			}
			if persistent && !strings.Contains(reply, "核对") {
				t.Fatal("failure after completed action lost the outcome warning")
			}
			if !persistent && reply != "操作已完成。" {
				t.Fatal("valid corrected reply was lost")
			}
		})
	}
}

func TestConfirmedToolDSMLResultDoesNotPollutePublicHistory(t *testing.T) {
	engine := NewEngine(EngineDependencies{})
	executed := 0
	engine.Registry.Register(ToolMeta{Name: "fixture_confirmed", ResultHandler: func(*ToolContext) ToolResult {
		executed++
		return ToolResult{Status: ToolSucceeded, Content: dsmlOutputFixture, PossibleSideEffects: true}
	}})
	action, code := engine.Confirms.Store(42, 100, "fixture_confirmed", nil)
	event := &bot.GroupMessageEvent{SelfID: 1, GroupID: 100, UserID: 42, MessageID: 123, Message: bot.Msg().Text("确认 " + code + " " + action).Build(), Sender: bot.Sender{Role: "member"}}
	var reply string
	handled, err := engine.handleConfirmation(context.Background(), &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}}, func(_ context.Context, text string) error { reply = text; return nil })
	if err != nil || !handled || executed != 1 || strings.Contains(reply, "DSML") {
		t.Fatalf("confirmation leaked protocol: handled=%v executed=%d err=%v", handled, executed, err)
	}
	for _, msg := range engine.Sessions.Get(100, 42).Messages {
		if strings.Contains(msg.Content, "DSML") || strings.Contains(msg.Content, "private-image-prompt-marker") {
			t.Fatal("confirmation stored raw protocol in public history")
		}
	}
}

func TestSummaryDSMLReplyDoesNotBecomeRetainedTaskContext(t *testing.T) {
	session := newSession(42, 100)
	reader := summaryReaderFunc(func(context.Context, chatsummary.Query) (chatsummary.Source, error) {
		return chatsummary.Source{Scope: "fixture", Records: []chatsummary.Record{{MessageID: 1, Text: "发布计划"}}}, nil
	})
	reply, handled, err := runSummaryWorkflowWith(nil, context.Background(), summaryGroup("总结群聊"), session, []*ToolMeta{{Name: "summarize_chat", ReadOnly: true}}, "总结群聊", reader, func(context.Context, openai.ChatCompletionRequest) (string, error) { return dsmlOutputFixture, nil })
	if !handled || err == nil || strings.Contains(reply, "DSML") || strings.Contains(session.summaryContext(), "private-image-prompt-marker") {
		t.Fatal("summary retained malformed protocol output")
	}
}
