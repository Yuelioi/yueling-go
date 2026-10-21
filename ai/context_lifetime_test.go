package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/internal/testconfig"
	openai "github.com/sashabaranov/go-openai"
)

type lifetimeCheckpoint struct{ data []byte }

func (s *lifetimeCheckpoint) Load(context.Context, int64, int64) (int64, []byte, error) {
	return 1, bytes.Clone(s.data), nil
}
func (s *lifetimeCheckpoint) Save(_ context.Context, _, _, _ int64, data []byte) error {
	s.data = bytes.Clone(data)
	return nil
}
func (s *lifetimeCheckpoint) Reset(context.Context, int64, int64) error { return nil }

func TestNewImageDoesNotInheritExpiredDrawingRules(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	cfg.AI = testconfig.AI("fixture", "", "fixture")
	config.Publish(cfg)
	for _, active := range []bool{false, true} {
		name := "idle checkpoint"
		if active {
			name = "active chat with an old image"
		}
		t.Run(name, func(t *testing.T) {
			now := time.Now()
			past := now.Add(-2 * time.Hour)
			turns := []map[string]any{{"input": "画成Q版，浅蓝背带裙 EXPIRED_DRAWING_RULE", "reply": "Q版已发送", "at": past}}
			updated := past
			if active {
				updated = now.Add(-time.Minute)
				turns = append(turns, map[string]any{"input": "RECENT_CHAT聊个新话题", "reply": "日常聊天", "at": updated})
			}
			data, err := json.Marshal(map[string]any{"version": 1, "context_version": 2, "updated_at": updated, "turns": turns, "images": []map[string]any{{"ID": "image_1", "URL": "https://fixture.invalid/old.png", "Source": "机器人生成的PNG", "kind": "generated", "caption": "EXPIRED_DRAWING_RULE：Q版，浅蓝背带裙", "created_at": past}}, "image_sequence": 1})
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			e := NewEngine(EngineDependencies{Conversations: &lifetimeCheckpoint{data: data}, ModelBackend: intelligenceModel(func(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
				calls++
				wire, _ := json.Marshal(req.Messages)
				if strings.Contains(string(wire), "EXPIRED_DRAWING_RULE") {
					t.Error("two-hour-old drawing instructions leaked into a new image request")
				}
				if active && !strings.Contains(string(wire), "RECENT_CHAT") {
					t.Error("unrelated recent chat was discarded")
				}
				if !strings.Contains(string(wire), "image_2：当前消息第1张") {
					t.Error("new original image is missing")
				}
				return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: "stop", Message: openai.ChatCompletionMessage{Role: "assistant", Content: "只按新原图换双马尾、不戴帽子。"}}}}, nil
			})})
			event := &bot.GroupMessageEvent{GroupID: 100, UserID: 42, SelfID: 1, MessageID: 1001, Message: bot.Msg().Image("https://fixture.invalid/new-original.png").Text("生成一个双马尾，不带帽子的版本").Build()}
			if err := e.Dispatch(context.Background(), &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { return nil }); err != nil {
				t.Fatal(err)
			}
			s := e.Sessions.Get(100, 42)
			if calls != 1 {
				t.Errorf("unexpected extra model calls: %d", calls)
			}
			history := (&ToolContext{session: s, event: event}).ImageHistory("EXPIRED_DRAWING_RULE", 8)
			if !strings.Contains(history, "image_1") {
				t.Error("expired task rules were removed by deleting searchable image history")
			}
		})
	}
}

func TestRuntimeImageHintsDoNotBecomeFreshUserInstructions(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	cfg.AI = testconfig.AI("fixture", "", "fixture")
	config.Publish(cfg)
	data, _ := json.Marshal(map[string]any{"version": 1, "context_version": 2})
	store := &lifetimeCheckpoint{data: data}
	calls := 0
	e := NewEngine(EngineDependencies{Conversations: store, ModelBackend: intelligenceModel(func(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		calls++
		wire, _ := json.Marshal(req.Messages)
		if calls == 1 && !strings.Contains(string(wire), "OLD_IMAGE_HINT") {
			t.Error("fixture did not expose the runtime image description")
		}
		if calls == 2 && strings.Contains(string(wire), "OLD_IMAGE_HINT") {
			t.Error("runtime image description was refreshed through a recent user turn")
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: "stop", Message: openai.ChatCompletionMessage{Role: "assistant", Content: "已收到当前要求。"}}}}, nil
	})})
	s := e.Sessions.Get(100, 42)
	if err := e.restoreConversation(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	s.addImage(sessionImage{URL: "https://fixture.invalid/old.png", Source: "历史生成图", Kind: "generated", Caption: "OLD_IMAGE_HINT 保留Q版和背带裙"})
	dispatch := func(id int32, msg bot.Message) {
		t.Helper()
		event := &bot.GroupMessageEvent{GroupID: 100, UserID: 42, SelfID: 1, MessageID: id, Message: msg}
		if err := e.Dispatch(context.Background(), &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	dispatch(1, bot.Msg().Text("先聊个新话题").Build())
	var saved conversationCheckpoint
	if err := json.Unmarshal(store.data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.ContextVersion != 2 || len(saved.Turns) != 1 {
		t.Fatal("missing versioned domain history")
	}
	input := saved.Turns[0].Input
	for _, invalid := range []string{"OLD_IMAGE_HINT", "参考图用edit_image", "上轮保留图片", "省略的对象优先"} {
		if strings.Contains(input, invalid) {
			t.Errorf("persisted runtime hint %q", invalid)
		}
	}
	if !strings.Contains(input, "先聊个新话题") || !strings.Contains(input, `"speaker_id":42`) {
		t.Error("lost user text or identity")
	}
	s.images[0].CreatedAt = time.Now().Add(-2 * time.Hour)
	dispatch(2, bot.Msg().Image("https://fixture.invalid/new.png").Text("只把新原图换成双马尾").Build())
	if calls != 2 {
		t.Errorf("added model calls: %d", calls)
	}
}

func TestLegacyPromptCheckpointsDoNotReplayTaskRules(t *testing.T) {
	now := time.Now()
	data, _ := json.Marshal(map[string]any{"version": 1, "updated_at": now, "turns": []map[string]any{{"input": "LEGACY_TASK_RULE", "reply": "旧要求"}}, "summary": "LEGACY_SUMMARY", "images": []sessionImage{{ID: "image_9", Source: "旧图", Caption: "历史角色", Kind: "generated", MessageID: 123}}, "image_sequence": 9})
	e := NewEngine(EngineDependencies{Conversations: &lifetimeCheckpoint{data: data}})
	s := e.Sessions.Get(100, 42)
	if err := e.restoreConversation(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if len(s.Messages) != 0 || len(s.turns) != 0 || s.historySummary != "" {
		t.Error("legacy full prompts were replayed as current user turns")
	}
	if len(s.images) != 1 || s.imageSequence != 9 {
		t.Error("legacy image lookup index was deleted")
	}
	if input := s.attachImages("新任务", nil, nil); strings.Contains(input, "历史角色") {
		t.Error("undated legacy image rules were implicitly shown")
	}
}

func TestRecentTaskAndImageContinuationRemainsAvailable(t *testing.T) {
	now := time.Now()
	recent := now.Add(-2 * time.Minute)
	s := newSession(42, 100)
	s.turns = []turnRecord{{At: recent, Input: "继续用银白发角色", Reply: "已生成"}}
	s.lastTurnAt = recent
	s.addImage(sessionImage{CreatedAt: recent, Kind: "generated", Source: "机器人生成PNG", Caption: "银白发角色"})
	s.prepareRecentContext(now)
	if len(s.Messages) != 2 || !strings.Contains(s.attachImages("刚才那张继续改", nil, nil), "银白发角色") {
		t.Error("short follow-up lost its task or image")
	}
	s.prepareRecentContext(now.Add(2 * time.Hour))
	if len(s.Messages) != 0 {
		t.Error("active in-memory session retained an expired task")
	}
}
