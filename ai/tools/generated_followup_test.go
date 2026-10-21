package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"github.com/gorilla/websocket"
	openai "github.com/sashabaranov/go-openai"
)

type imageCheckpointFixture struct {
	sync.Mutex
	records map[string][]byte
}

func (s *imageCheckpointFixture) Load(_ context.Context, g, u int64) (int64, []byte, error) {
	s.Lock()
	defer s.Unlock()
	return 1, bytes.Clone(s.records[fmt.Sprint(g, ":", u)]), nil
}
func (s *imageCheckpointFixture) Save(_ context.Context, g, u, _ int64, data []byte) error {
	s.Lock()
	defer s.Unlock()
	s.records[fmt.Sprint(g, ":", u)] = bytes.Clone(data)
	return nil
}
func (s *imageCheckpointFixture) Reset(_ context.Context, g, u int64) error {
	s.Lock()
	defer s.Unlock()
	delete(s.records, fmt.Sprint(g, ":", u))
	return nil
}

func TestGeneratedImageSurvivesChatRestartAndDifferentQuotedReference(t *testing.T) {
	old, oldPublic := config.Current(), httpclient.Public
	t.Cleanup(func() { config.Publish(old); httpclient.Public = oldPublic })
	makePNG := func(c color.NRGBA) []byte {
		img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
		img.SetNRGBA(1, 1, c)
		var out bytes.Buffer
		_ = png.Encode(&out, img)
		return out.Bytes()
	}
	white, pink := makePNG(color.NRGBA{R: 255, G: 255, B: 255, A: 255}), makePNG(color.NRGBA{R: 255, B: 128, A: 255})
	httpclient.Public = &httpclient.Client{Client: &http.Client{Transport: avatarTransport(func(r *http.Request) (*http.Response, error) {
		data := white
		if r.URL.Path == "/pink.png" {
			data = pink
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"image/png"}}, Body: io.NopCloser(bytes.NewReader(data))}, nil
	})}}
	var edits atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/images/edits" {
			edits.Add(1)
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
				return
			}
			defer r.MultipartForm.RemoveAll()
			files := r.MultipartForm.File["image[]"]
			if len(files) != 2 {
				t.Errorf("reference count=%d", len(files))
				return
			}
			for i, want := range [][]byte{white, pink} {
				f, err := files[i].Open()
				if err != nil {
					t.Error(err)
					return
				}
				data, _ := io.ReadAll(f)
				f.Close()
				if !bytes.Equal(data, want) {
					t.Errorf("reference %d confused generated character with quoted pose", i)
				}
			}
		} else if r.URL.Path != "/images/generations" {
			t.Errorf("unexpected provider path=%s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(white)}}})
	}))
	defer provider.Close()
	cfg := old
	cfg.AI = config.AIConfig{Connections: []config.AIConnection{{ID: "fixture", Kind: "openai", BaseURL: provider.URL}}, Profiles: []config.AIProfile{
		{ID: "chat", ConnectionID: "fixture", Model: "chat", Capabilities: []string{"chat", "tools"}},
		{ID: "image", ConnectionID: "fixture", Model: "drawing", Capabilities: []string{"image_generation", "image_edit"}},
	}, Bindings: map[string]string{"conversation": "chat", "image_generation": "image", "image_edit": "image"}}
	config.Publish(cfg)
	b := bot.New()
	t.Cleanup(func() { b.Stop(); b.Wait() })
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
		t.Fatal("OneBot fixture did not connect")
	}
	var sent [][]byte
	var sentMu sync.Mutex
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
			data := map[string]any{}
			switch req.Action {
			case "set_msg_emoji_like":
			case "get_msg":
				var id int32
				_ = json.Unmarshal(req.Params["message_id"], &id)
				url := "https://images.fixture/white.png"
				if id == 900 {
					url = "https://images.fixture/pink.png"
				} else if id != 456 {
					t.Errorf("unexpected referenced message=%d", id)
				}
				data["message"] = bot.Msg().Image(url).Build()
			case "send_group_msg":
				var message bot.Message
				_ = json.Unmarshal(req.Params["message"], &message)
				for _, segment := range message {
					if segment.Type == "image" {
						var v struct{ File string }
						_ = json.Unmarshal(segment.Data, &v)
						raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(v.File, "base64://"))
						if err != nil {
							t.Error(err)
						}
						sentMu.Lock()
						sent = append(sent, raw)
						data["message_id"] = 456 + (len(sent) - 1)
						sentMu.Unlock()
					}
				}
			default:
				t.Errorf("unexpected OneBot action=%s", req.Action)
			}
			if peer.WriteJSON(map[string]any{"echo": req.Echo, "status": "ok", "retcode": 0, "data": data}) != nil {
				return
			}
		}
	}()
	defer func() { peer.Close(); <-done }()
	store := &imageCheckpointFixture{records: map[string][]byte{}}
	calls := 0
	first := ai.NewEngine(ai.EngineDependencies{Conversations: store, ModelBackend: imageRequestModel(func(req openai.ChatCompletionRequest) openai.ChatCompletionResponse {
		calls++
		message := openai.ChatCompletionMessage{Role: "assistant", Content: "聊聊日常。"}
		if calls == 1 {
			message.Content = ""
			message.ToolCalls = []openai.ToolCall{{ID: "draw", Type: "function", Function: openai.FunctionCall{Name: "generate_image", Arguments: `{"prompt":"银白发Q版头像"}`}}}
		}
		if calls == 2 {
			if !strings.Contains(req.Messages[len(req.Messages)-1].Content, "image_id=image_1") {
				t.Error("generated output did not return a reusable identifier")
			}
			message.Content = "白发角色已发送。"
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: message, FinishReason: openai.FinishReasonStop}}}
	})})
	RegisterAll(first.Registry, Dependencies{})
	dispatch := func(engine *ai.Engine, id int32, message bot.Message) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		event := &bot.GroupMessageEvent{SelfID: 1, GroupID: 100, UserID: 42, MessageID: id, Message: message, Sender: bot.Sender{Role: "member"}}
		if err := engine.Dispatch(ctx, &bot.GroupContext{BotAPI: api, MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	dispatch(first, 1, bot.Msg().Text("生成银白发Q版头像").Build())
	for i := int32(2); i <= 6; i++ {
		dispatch(first, i, bot.Msg().Text("闲聊一个新话题").Build())
	}
	followupCalls := 0
	second := ai.NewEngine(ai.EngineDependencies{Conversations: store, ModelBackend: imageRequestModel(func(req openai.ChatCompletionRequest) openai.ChatCompletionResponse {
		followupCalls++
		message := openai.ChatCompletionMessage{Role: "assistant", Content: "动画已发送。"}
		if followupCalls == 1 {
			input := req.Messages[len(req.Messages)-1].Content
			if !strings.Contains(input, "image_1") || !strings.Contains(input, "image_2") || strings.Contains(input, "；历史生成描述") {
				t.Errorf("current reference should not import old generation instructions: %s", input)
			}
			message.Content = ""
			message.ToolCalls = []openai.ToolCall{{ID: "find", Type: "function", Function: openai.FunctionCall{Name: "get_chat_history", Arguments: `{"image_query":"白发","count":8}`}}}
		} else if followupCalls == 2 {
			if result := req.Messages[len(req.Messages)-1].Content; !strings.Contains(result, "image_1") || !strings.Contains(result, "银白发Q版") {
				t.Error("explicit image lookup lost the old character")
			}
			message.Content = ""
			message.ToolCalls = []openai.ToolCall{{ID: "animate", Type: "function", Function: openai.FunctionCall{Name: "edit_image", Arguments: `{"image_ids":["image_1","image_2"],"prompt":"保留第一张白发角色，第二张只参考姿势","animation_frames":["打哈欠","揉眼睛","闭眼睡觉"]}`}}}
		} else if !strings.Contains(req.Messages[len(req.Messages)-1].Content, "format=gif") {
			t.Error("model was not told that a completed GIF was sent")
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: message, FinishReason: openai.FinishReasonStop}}}
	})})
	RegisterAll(second.Registry, Dependencies{})
	dispatch(second, 7, bot.Msg().Reply(900).Text("把刚才生成的白发Q版做成动作GIF，引用这张只参考姿势").Build())
	sentMu.Lock()
	defer sentMu.Unlock()
	if len(sent) != 2 || edits.Load() != 3 || followupCalls != 3 {
		t.Fatalf("sent=%d edits=%d model_calls=%d", len(sent), edits.Load(), followupCalls)
	}
	animation, err := gif.DecodeAll(bytes.NewReader(sent[1]))
	if err != nil || len(animation.Image) != 3 {
		t.Fatalf("final message was not a three-frame GIF: %v", err)
	}
}
