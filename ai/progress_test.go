package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/internal/testconfig"
	"github.com/gorilla/websocket"
	openai "github.com/sashabaranov/go-openai"
)

func progressConnection(t *testing.T, replyToReaction bool) (*bot.BotAPI, <-chan struct{}, *atomic.Int32) {
	t.Helper()
	b := bot.New()
	t.Cleanup(func() { b.Stop(); b.Wait() })
	connected := make(chan *bot.BotAPI, 1)
	b.OnConnect(func(api *bot.BotAPI) { connected <- api })
	server := httptest.NewServer(b.ReverseHandler("fixture"))
	t.Cleanup(server.Close)
	peer, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/onebot/v11/ws", http.Header{"Authorization": {"Bearer fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { peer.Close() })
	if err := peer.WriteJSON(map[string]any{"post_type": "meta_event", "meta_event_type": "lifecycle", "self_id": 1}); err != nil {
		t.Fatal(err)
	}
	var api *bot.BotAPI
	select {
	case api = <-connected:
	case <-time.After(time.Second):
		t.Fatal("connection timeout")
	}
	reacted := make(chan struct{})
	count := &atomic.Int32{}
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
			switch req.Action {
			case "set_msg_emoji_like":
				if string(req.Params["emoji_id"]) != `"424"` || string(req.Params["message_id"]) != "123" {
					t.Errorf("wrong progress target: %v", req.Params)
				}
				if count.Add(1) == 1 {
					close(reacted)
				}
				if !replyToReaction {
					continue
				}
				if peer.WriteJSON(map[string]any{"echo": req.Echo, "status": "failed", "retcode": 100, "data": nil}) != nil {
					return
				}
			case "get_msg": // Leave the quote pending; progress must not depend on it.
			default:
				t.Errorf("unexpected action %s", req.Action)
			}
		}
	}()
	t.Cleanup(func() { peer.Close(); <-done })
	return api, reacted, count
}

func TestDispatchProgressDoesNotWaitForConversationOrQuote(t *testing.T) {
	for _, queued := range []bool{false, true} {
		t.Run(map[bool]string{false: "quote_read", true: "conversation_queue"}[queued], func(t *testing.T) {
			old := config.Current()
			t.Cleanup(func() { config.Publish(old) })
			cfg := old
			cfg.AI = testconfig.AI("fixture", "http://fixture.invalid", "fixture")
			config.Publish(cfg)
			api, reacted, count := progressConnection(t, true)
			var models atomic.Int32
			e := NewEngine(EngineDependencies{ModelBackend: intelligenceModel(func(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
				models.Add(1)
				return openai.ChatCompletionResponse{}, nil
			})})
			session := e.Sessions.Get(100, 42)
			if queued {
				if err := session.acquire(context.Background()); err != nil {
					t.Fatal(err)
				}
				defer session.release()
			}
			message := bot.Msg().Text("参考这张图换成水彩")
			if !queued {
				message.Reply(321)
			}
			event := &bot.GroupMessageEvent{SelfID: 1, GroupID: 100, UserID: 42, MessageID: 123, Message: message.Build()}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			completed := false
			go func() {
				done <- e.Dispatch(ctx, &bot.GroupContext{BotAPI: api, MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { return nil })
			}()
			defer func() {
				cancel()
				if completed {
					return
				}
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Error("dispatch did not cancel")
				}
			}()
			select {
			case <-reacted:
			case <-time.After(250 * time.Millisecond):
				t.Fatal("progress waited for conversation/quote instead of acknowledging accepted work")
			}
			if models.Load() != 0 {
				t.Fatal("model ran while its prerequisite was blocked")
			}
			if err := e.Dispatch(ctx, &bot.GroupContext{BotAPI: api, MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { return nil }); err != nil {
				t.Fatal(err)
			}
			cancel()
			<-done
			completed = true
			if count.Load() != 1 {
				t.Fatal("duplicate progress", count.Load())
			}
		})
	}
}

func TestProgressResponseDoesNotBlockModel(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	cfg.AI = testconfig.AI("fixture", "http://fixture.invalid", "fixture")
	config.Publish(cfg)
	api, reacted, _ := progressConnection(t, false)
	e := NewEngine(EngineDependencies{ModelBackend: intelligenceModel(func(ctx context.Context, _ openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		select {
		case <-reacted:
		case <-ctx.Done():
			return openai.ChatCompletionResponse{}, ctx.Err()
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: "stop", Message: openai.ChatCompletionMessage{Role: "assistant", Content: "收到"}}}}, nil
	})})
	event := &bot.GroupMessageEvent{SelfID: 1, GroupID: 100, UserID: 42, MessageID: 123, Message: bot.Msg().Text("你好").Build()}
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	reply := ""
	if err := e.Dispatch(ctx, &bot.GroupContext{BotAPI: api, MsgCtx: &bot.MsgCtx{Event: event}}, func(_ context.Context, s string) error { reply = s; cancel(); return nil }); err != nil {
		t.Fatal(err)
	}
	if reply != "收到" {
		t.Fatalf("reaction response held up model/delivery: %q", reply)
	}
}
