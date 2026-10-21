package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/gorilla/websocket"
	openai "github.com/sashabaranov/go-openai"
)

type imageRequestModel func(openai.ChatCompletionRequest) openai.ChatCompletionResponse

func (f imageRequestModel) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return f(req), nil
}

func TestImageRequestReactionAndDelivery(t *testing.T) {
	for _, scenario := range []struct {
		name                  string
		reactionFails, quoted bool
	}{
		{name: "generate"},
		{name: "reaction failure still generates", reactionFails: true},
		{name: "quoted image is resolved before edit", quoted: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			old := config.C
			t.Cleanup(func() { config.C = old })
			var pngData bytes.Buffer
			if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
				t.Fatal(err)
			}
			var reacted, generated, sent atomic.Bool
			reactionStarted := make(chan struct{})
			images := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if scenario.quoted {
					t.Error("invalid source must not reach the image provider")
				}
				var body struct{ Model, Prompt string }
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if r.URL.Path != "/images/generations" || body.Model != "drawing" || body.Prompt != "风格是水彩，保留人物和构图" {
					t.Errorf("unexpected image request: %s %+v", r.URL.Path, body)
				}
				generated.Store(true)
				_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(pngData.Bytes())}}})
			}))
			defer images.Close()
			config.C.AI = config.AIConfig{
				Connections: []config.AIConnection{{ID: "fixture", Kind: "openai", BaseURL: images.URL}},
				Profiles: []config.AIProfile{
					{ID: "chat", ConnectionID: "fixture", Model: "chat", Capabilities: []string{"chat", "tools"}},
					{ID: "image", ConnectionID: "fixture", Model: "drawing", Capabilities: []string{"image_generation", "image_edit"}},
				}, Bindings: map[string]string{"conversation": "chat", "image_generation": "image", "image_edit": "image"},
			}
			b := bot.New()
			defer func() { b.Stop(); b.Wait() }()
			connected := make(chan *bot.BotAPI, 1)
			b.OnConnect(func(api *bot.BotAPI) { connected <- api })
			transport := httptest.NewServer(b.ReverseHandler("fixture"))
			defer transport.Close()
			peer, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(transport.URL, "http")+"/onebot/v11/ws", http.Header{"Authorization": {"Bearer fixture"}})
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
				t.Fatal("OneBot did not connect")
			}
			peerDone := make(chan struct{})
			go func() {
				defer close(peerDone)
				for {
					var req struct {
						Action, Echo string
						Params       map[string]json.RawMessage
					}
					if peer.ReadJSON(&req) != nil {
						return
					}
					status, retcode := "ok", 0
					data := map[string]any{}
					switch req.Action {
					case "get_msg":
						if string(req.Params["message_id"]) != "321" {
							t.Error("wrong quoted message")
						}
						data["message"] = bot.Msg().Image("http://127.0.0.1/blocked-source").Build()
					case "set_msg_emoji_like":
						if string(req.Params["message_id"]) != "123" || string(req.Params["emoji_id"]) != `"424"` || string(req.Params["set"]) != "true" {
							t.Errorf("wrong reaction target: %s", req.Params)
						}
						reacted.Store(true)
						close(reactionStarted)
						if scenario.reactionFails {
							status, retcode = "failed", 100
						}
					case "send_group_msg":
						var msg bot.Message
						_ = json.Unmarshal(req.Params["message"], &msg)
						reply, _ := msg.ReplyID()
						if !generated.Load() || !msg.HasType("image") || reply != "123" || string(req.Params["group_id"]) != "100" {
							t.Errorf("wrong image delivery: %s", req.Params)
						}
						sent.Store(true)
						data["message_id"] = 456
					default:
						t.Errorf("unexpected OneBot action %s", req.Action)
					}
					if peer.WriteJSON(map[string]any{"echo": req.Echo, "status": status, "retcode": retcode, "data": data}) != nil {
						return
					}
				}
			}()
			defer func() { peer.Close(); <-peerDone }()
			var calls int
			engine := ai.NewEngine(ai.EngineDependencies{ModelBackend: imageRequestModel(func(req openai.ChatCompletionRequest) openai.ChatCompletionResponse {
				calls++
				if calls == 1 {
					// Hold only the fake model, not Dispatch, until progress is
					// observed. Its goroutine has no ordering guarantee with HTTP.
					select {
					case <-reactionStarted:
					case <-time.After(3 * time.Second):
						t.Error("progress reaction was not attempted")
					}
				}
				msg := openai.ChatCompletionMessage{Role: "assistant", Content: "处理完毕"}
				if calls == 1 {
					name, args := "generate_image", `{"prompt":"风格是水彩，保留人物和构图"}`
					if scenario.quoted {
						name, args = "edit_image", `{"prompt":"风格是水彩，保留人物和构图","image_ids":["image_1"]}`
						input := req.Messages[len(req.Messages)-1].Content
						if !strings.Contains(input, "image_1：引用消息第1张") || !strings.Contains(input, "风格是水彩") {
							t.Errorf("quoted image or style missing: %s", input)
						}
					}
					msg.Content = ""
					msg.ToolCalls = []openai.ToolCall{{ID: "draw", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: name, Arguments: args}}}
				} else {
					var result ai.ToolResult
					for _, m := range req.Messages {
						if m.Role == "tool" {
							_ = json.Unmarshal([]byte(m.Content), &result)
						}
					}
					if scenario.quoted {
						if result.Status != ai.ToolFailed || result.Content != "原图下载失败" {
							t.Errorf("quoted image did not reach validated download: %+v", result)
						}
					} else if result.Status != ai.ToolSucceeded {
						t.Errorf("generation failed: %+v", result)
					}
				}
				return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: msg, FinishReason: openai.FinishReasonStop}}}
			})})
			(&toolset{registry: engine.Registry}).registerImageGeneration()
			message := bot.Msg().At(1).Text("月灵 生图，风格是水彩，保留人物和构图")
			if scenario.quoted {
				message.Reply(321)
			}
			event := &bot.GroupMessageEvent{SelfID: 1, GroupID: 100, UserID: 42, MessageID: 123, Message: message.Build(), Sender: bot.Sender{Role: "member"}}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := engine.Dispatch(ctx, &bot.GroupContext{BotAPI: api, MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { cancel(); return nil }); err != nil {
				t.Fatal(err)
			}
			if calls != 2 || !reacted.Load() || sent.Load() == scenario.quoted {
				t.Fatalf("calls=%d reacted=%v sent=%v", calls, reacted.Load(), sent.Load())
			}
		})
	}
}
