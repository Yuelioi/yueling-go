package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"github.com/Yuelioi/yueling-go/services/imaging"
	"github.com/gorilla/websocket"
	openai "github.com/sashabaranov/go-openai"
)

func TestImageOutputThroughDispatch(t *testing.T) {
	for _, scenario := range []string{"transparent cutout", "opaque cutout rejected", "AI GIF cutout"} {
		t.Run(scenario, func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, 4, 2))
			img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
			var encoded bytes.Buffer
			if err := png.Encode(&encoded, img); err != nil {
				t.Fatal(err)
			}
			source := encoded.Bytes()
			tool, args := "edit_image", `{"image_ids":["image_1"],"prompt":"抠出主体","transparent_background":true}`
			isGIF := strings.Contains(scenario, "GIF")
			if isGIF {
				other := image.NewNRGBA(img.Bounds())
				other.SetNRGBA(3, 0, color.NRGBA{B: 255, A: 255})
				var err error
				source, err = imaging.Encode(&imaging.Animation{Frames: []*image.NRGBA{img, other}, Delays: []int{4, 9}, LoopCount: 3}, imaging.DefaultLimits)
				if err != nil {
					t.Fatal(err)
				}
			}
			var providerCalls atomic.Int32
			result, sent := dispatchImageFixture(t, source, tool, args, func(w http.ResponseWriter, r *http.Request) {
				providerCalls.Add(1)
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					t.Error(err)
					return
				}
				defer r.MultipartForm.RemoveAll()
				if r.URL.Path != "/images/edits" || r.FormValue("background") != "transparent" || r.FormValue("output_format") != "png" {
					t.Errorf("missing transparent edit fields: %s", r.URL.Path)
				}
				file, _, err := r.FormFile("image")
				if err != nil {
					t.Error(err)
					return
				}
				data, _ := io.ReadAll(file)
				file.Close()
				frame, err := png.Decode(bytes.NewReader(data))
				if err != nil {
					t.Error("GIF must be sent as complete PNG display frames")
					return
				}
				if scenario == "opaque cutout rejected" {
					opaque := image.NewNRGBA(frame.Bounds())
					for y := 0; y < 2; y++ {
						for x := 0; x < 4; x++ {
							opaque.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
						}
					}
					var b bytes.Buffer
					_ = png.Encode(&b, opaque)
					data = b.Bytes()
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(data)}}})
			})
			if scenario == "opaque cutout rejected" {
				if result.Status != ai.ToolFailed || !result.PossibleSideEffects || !strings.Contains(result.Content, "透明") || len(sent) != 0 || providerCalls.Load() != 1 {
					t.Fatalf("opaque output accepted or retried: %+v sent=%d calls=%d", result, len(sent), providerCalls.Load())
				}
				return
			}
			if result.Status != ai.ToolSucceeded || len(sent) == 0 {
				t.Fatalf("image action failed: %+v", result)
			}
			if !strings.Contains(result.Content, "image_id=") {
				t.Fatal("sent image was not registered for subsequent edits")
			}
			wantCalls := int32(1)
			if isGIF {
				wantCalls = 2
			}
			if providerCalls.Load() != wantCalls {
				t.Fatalf("provider calls=%d want=%d", providerCalls.Load(), wantCalls)
			}
			if !isGIF {
				if !bytes.Equal(sent, source) {
					t.Fatal("transparent PNG changed during delivery")
				}
				return
			}
			animation, err := gif.DecodeAll(bytes.NewReader(sent))
			if err != nil || len(animation.Image) != 2 || animation.Delay[0] != 4 || animation.Delay[1] != 9 || animation.LoopCount != 3 {
				t.Fatalf("GIF timeline lost: %v %+v", err, animation)
			}
			for i, x := range []int{0, 3} {
				_, _, _, alpha := animation.Image[i].At(x, 0).RGBA()
				_, _, _, background := animation.Image[i].At(1, 1).RGBA()
				_, _, _, old := animation.Image[i].At(3-x, 0).RGBA()
				if alpha == 0 || background != 0 || old != 0 {
					t.Fatalf("GIF frame %d lost subject or transparency", i)
				}
			}
		})
	}
}

func TestStaticReferenceCreatesThreeFrameGIFInOneToolCall(t *testing.T) {
	frame := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	frame.SetNRGBA(1, 1, color.NRGBA{R: 255, A: 255})
	var source bytes.Buffer
	if err := png.Encode(&source, frame); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	result, sent := dispatchImageFixture(t, source.Bytes(), "edit_image", `{"image_ids":["image_1"],"prompt":"同一白发Q版角色，固定镜头","animation_frames":["掩嘴打哈欠","揉眼睛、眼角带泪花","闭眼睡觉，头顶ZZ"],"frame_duration_ms":500}`, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(source.Bytes())}}})
	})
	animation, err := gif.DecodeAll(bytes.NewReader(sent))
	if result.Status != ai.ToolSucceeded || err != nil || len(animation.Image) != 3 || calls.Load() != 3 {
		t.Fatalf("one animation task did not send three-frame GIF: result=%+v provider_calls=%d decode=%v", result, calls.Load(), err)
	}
	for _, delay := range animation.Delay {
		if delay != 50 {
			t.Fatalf("frame duration was not preserved: %v", animation.Delay)
		}
	}
}

func dispatchImageFixture(t *testing.T, source []byte, tool, args string, provider http.HandlerFunc) (ai.ToolResult, []byte) {
	t.Helper()
	old, oldPublic := config.C, httpclient.Public
	t.Cleanup(func() { config.C, httpclient.Public = old, oldPublic })
	httpclient.Public = &httpclient.Client{Client: &http.Client{Transport: avatarTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(source))}, nil
	})}}
	server := httptest.NewServer(provider)
	defer server.Close()
	config.C.AI = config.AIConfig{Connections: []config.AIConnection{{ID: "fixture", Kind: "openai", BaseURL: server.URL}}, Profiles: []config.AIProfile{
		{ID: "chat", ConnectionID: "fixture", Model: "chat", Capabilities: []string{"chat", "tools"}},
		{ID: "image", ConnectionID: "fixture", Model: "drawing", Capabilities: []string{"image_edit"}},
	}, Bindings: map[string]string{"conversation": "chat", "image_edit": "image"}}
	config.C.Bot.SuperUsers = nil
	config.C.Plugins.Disabled = nil
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
	if err = peer.WriteJSON(map[string]any{"post_type": "meta_event", "meta_event_type": "lifecycle", "self_id": 1}); err != nil {
		t.Fatal(err)
	}
	var api *bot.BotAPI
	select {
	case api = <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("OneBot did not connect")
	}
	var sent []byte
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
			case "send_group_msg":
				var message bot.Message
				_ = json.Unmarshal(req.Params["message"], &message)
				for _, seg := range message {
					if seg.Type == "image" {
						var err error
						var data struct {
							File string `json:"file"`
						}
						_ = json.Unmarshal(seg.Data, &data)
						sent, err = base64.StdEncoding.DecodeString(strings.TrimPrefix(data.File, "base64://"))
						if err != nil {
							t.Error(err)
						}
					}
				}
				reply, _ := message.ReplyID()
				if reply != "123" || string(req.Params["group_id"]) != "100" {
					t.Error("image sent to wrong message or group")
				}
				data["message_id"] = 456
			default:
				t.Errorf("unexpected action %s", req.Action)
			}
			if peer.WriteJSON(map[string]any{"echo": req.Echo, "status": "ok", "retcode": 0, "data": data}) != nil {
				return
			}
		}
	}()
	var result ai.ToolResult
	calls := 0
	engine := ai.NewEngine(ai.EngineDependencies{ModelBackend: imageRequestModel(func(req openai.ChatCompletionRequest) openai.ChatCompletionResponse {
		calls++
		message := openai.ChatCompletionMessage{Role: "assistant", Content: "处理结束。"}
		if calls == 1 {
			message.Content = ""
			message.ToolCalls = []openai.ToolCall{{ID: "edit", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: tool, Arguments: args}}}
		} else {
			if err := json.Unmarshal([]byte(req.Messages[len(req.Messages)-1].Content), &result); err != nil {
				t.Error(err)
			}
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: message, FinishReason: openai.FinishReasonStop}}}
	})})
	(&toolset{registry: engine.Registry}).registerAll()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	event := &bot.GroupMessageEvent{SelfID: 1, GroupID: 100, UserID: 42, MessageID: 123, Message: bot.Msg().At(1).Text("抠图 GIF 缩放 翻转").Image("https://image.fixture/source").Build(), Sender: bot.Sender{Role: "member"}}
	err = engine.Dispatch(ctx, &bot.GroupContext{BotAPI: api, MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { cancel(); return nil })
	peer.Close()
	<-done
	if err != nil || calls != 2 {
		t.Fatalf("dispatch calls=%d err=%v", calls, err)
	}
	return result, sent
}
