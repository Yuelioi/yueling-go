package ai

import (
	"context"
	"fmt"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
	"strings"
	"testing"
)

func TestImageReferencesFollowupReplacementAndIsolation(t *testing.T) {
	s := newSession(1, 2)
	input := s.attachImages("存到语录", []string{"https://example.com/current"}, []string{"https://example.com/quoted"})
	for _, want := range []string{"image_1：当前消息第1张", "image_2：引用消息第1张"} {
		if !strings.Contains(input, want) {
			t.Fatal(input)
		}
	}
	if strings.Contains(input, "https://") {
		t.Fatal("URL leaked to model")
	}
	s.resetTurn()
	if !strings.Contains(s.attachImages("名字叫小明", nil, nil), "上轮保留图片") {
		t.Fatal("lost followup attachments")
	}
	c := &ToolContext{session: s}
	urls, err := c.ImageURLs([]string{"image_2", "image_2", "image_1"})
	if err != nil || len(urls) != 2 || urls[0] != "https://example.com/quoted" {
		t.Fatalf("%v %v", urls, err)
	}
	for _, ids := range [][]string{nil, {"https://example.com/current"}, {"image_1", "unknown"}} {
		if u, e := c.ImageURLs(ids); e == nil || u != nil {
			t.Fatalf("accepted %v", ids)
		}
	}
	other := &ToolContext{session: newSession(1, 3)}
	if _, err := other.ImageURLs([]string{"image_1"}); err == nil {
		t.Fatal("cross session reference accepted")
	}
	s.attachImages("新图片", []string{"https://example.com/new"}, nil)
	if _, err := c.ImageURLs([]string{"image_1"}); err == nil {
		t.Fatal("stale attachment accepted")
	}
	if urls, err := c.ImageURLs([]string{"image_3"}); err != nil || urls[0] != "https://example.com/new" {
		t.Fatalf("%v %v", urls, err)
	}
}
func TestImageReferencesBounded(t *testing.T) {
	s := newSession(1, 2)
	text := s.attachImages("保存", []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}, nil)
	if len(s.images) != 8 || !strings.Contains(text, "仅前8张") {
		t.Fatal(text)
	}
}

func TestMissingQuotedImageDoesNotReusePrevious(t *testing.T) {
	s := newSession(1, 2)
	s.attachImages("old", []string{"https://example.com/old"}, nil)
	g := &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: &bot.GroupMessageEvent{Message: bot.Msg().Reply(42).Text("存这张到语录").Build()}}}
	text := s.imageInput(g)
	if len(s.images) != 0 || !strings.Contains(text, "读取失败") {
		t.Fatal(text)
	}
}

type imageModel struct {
	t     *testing.T
	calls int
}

func (m *imageModel) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	m.calls++
	msg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant}
	switch m.calls {
	case 1:
		if !strings.Contains(req.Messages[len(req.Messages)-1].Content, "image_1") {
			m.t.Error("model did not receive image reference")
		}
		msg.Content = "请告诉我语录名字。"
	case 2:
		if !strings.Contains(req.Messages[len(req.Messages)-1].Content, "上轮保留图片") {
			m.t.Error("followup lost image")
		}
		msg.ToolCalls = []openai.ToolCall{{ID: "save", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "save_image_asset", Arguments: `{"image_ids":["image_1"],"name":"小明"}`}}}
	case 3:
		msg.Content = "已保存。"
	default:
		return openai.ChatCompletionResponse{}, fmt.Errorf("unexpected call")
	}
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: msg, FinishReason: openai.FinishReasonStop}}}, nil
}
func TestImageFollowupThroughConversationAndExecutor(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	config.C.AI.Model = "fixture"
	model := &imageModel{t: t}
	engine := NewEngine(EngineDependencies{ModelBackend: model})
	count := 0
	tool := ToolMeta{Name: "save_image_asset", Triggers: []string{"语录"}, Params: []Param{{Name: "name", Type: "string", Required: true}, {Name: "image_ids", Type: "array", ItemsType: "string", Required: true}}, ResultHandler: func(c *ToolContext) ToolResult {
		urls, err := c.ImageURLs(c.StringSlice("image_ids"))
		if err != nil || len(urls) != 1 || urls[0] != "https://example.com/image" || c.String("name") != "小明" {
			t.Errorf("wrong resolved arguments %v %v", urls, err)
		}
		count++
		return ToolResult{Status: ToolSucceeded, Content: "saved", PossibleSideEffects: true}
	}}
	engine.Registry.Register(tool)
	manager := &SessionManager{}
	for i, text := range []string{"帮我把图片存到语录", "名字叫小明"} {
		s := manager.Get(2, int64(i+1))
		var images []string
		if i == 0 {
			images = []string{"https://example.com/image"}
		}
		input := s.attachImages(text, images, nil)
		s.resetTurn()
		routes := s.routeTools(text, engine.Registry.AllTools())
		if len(routes) != 1 {
			t.Fatalf("followup route lost: %v", routes)
		}
		_, err := engine.runConversation(context.Background(), s, input, "system", []openai.Tool{tool.Schema()}, func(call openai.ToolCall) ToolResult {
			return engine.executeTool(context.Background(), nil, &bot.GroupMessageEvent{GroupID: 2, UserID: 1}, s, PermMember, call, map[string]bool{"save_image_asset": true})
		})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && count != 0 {
			t.Fatal("saved without a name")
		}
	}
	if count != 1 || model.calls != 3 {
		t.Fatalf("actions=%d model calls=%d", count, model.calls)
	}
}
