package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
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

func TestAvatarReferenceToolIsAvailableForOrdinaryMembers(t *testing.T) {
	avatar, ok := testEngine.Registry.GetTool("get_user_avatar")
	if !ok {
		t.Fatal("AI cannot obtain a mentioned user's avatar: get_user_avatar is not registered")
	}
	if avatar.Permission != ai.PermMember || !avatar.ReadOnly {
		t.Fatal("reading an avatar should be available to ordinary members without side effects")
	}
	for _, name := range []string{"edit_image", "inspect_image"} {
		tool, ok := testEngine.Registry.GetTool(name)
		if !ok || !slices.Contains(tool.Requires, "get_user_avatar") {
			t.Errorf("discovering %s must also expose the avatar reference tool", name)
		}
	}
}

func TestAvatarTargetSelectionThroughDispatch(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mentions []int64
		args     string
		wantUser int64
	}{
		{name: "mention excludes bot and duplicates", mentions: []int64{1, 100, 100}, args: `{}`, wantUser: 100},
		{name: "bot mention defaults to speaker", mentions: []int64{1}, args: `{}`, wantUser: 42},
		{name: "no mention defaults to speaker", args: `{}`, wantUser: 42},
		{name: "multiple mentions require a target", mentions: []int64{1, 100, 200}, args: `{}`},
		{name: "explicit target resolves ambiguity", mentions: []int64{1, 100, 200}, args: `{"user_id":200}`, wantUser: 200},
		{name: "my avatar with another mention", mentions: []int64{1, 100}, args: `{"user_id":42}`, wantUser: 42},
		{name: "explicit QQ without mention", args: `{"user_id":300}`, wantUser: 300},
		{name: "explicit bot avatar", mentions: []int64{1, 100}, args: `{"user_id":1}`, wantUser: 1},
		{name: "invalid identity", args: `{"user_id":0}`},
		{name: "negative identity", args: `{"user_id":-1}`},
		{name: "fractional identity", args: `{"user_id":100.5}`},
		{name: "string identity", args: `{"user_id":"100"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := config.C
			t.Cleanup(func() { config.C = old })
			config.C.AI = config.AIConfig{Model: "fixture", DeepSeekKey: "fixture"}
			config.C.Bot.SuperUsers = nil
			calls := 0
			engine := ai.NewEngine(ai.EngineDependencies{ModelBackend: imageRequestModel(func(req openai.ChatCompletionRequest) openai.ChatCompletionResponse {
				calls++
				msg := openai.ChatCompletionMessage{Role: "assistant", Content: "已核对头像引用。"}
				if calls == 1 {
					input := req.Messages[len(req.Messages)-1].Content
					if !strings.Contains(input, `"speaker_id":42`) || !strings.Contains(input, `"mentioned_ids"`) {
						t.Error("model did not receive trusted speaker and mention identities")
					}
					args, _ := json.Marshal(map[string]string{"name": "get_user_avatar", "arguments": tc.args})
					msg.Content = ""
					msg.ToolCalls = []openai.ToolCall{{ID: "avatar", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "invoke_available_tool", Arguments: string(args)}}}
				} else {
					var result ai.ToolResult
					if err := json.Unmarshal([]byte(req.Messages[len(req.Messages)-1].Content), &result); err != nil {
						t.Fatal(err)
					}
					want := ai.ToolRejected
					if tc.wantUser != 0 {
						want = ai.ToolSucceeded
					}
					if result.Status != want || result.PossibleSideEffects {
						t.Errorf("avatar result = %+v, want %s without side effects", result, want)
					}
				}
				return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: msg, FinishReason: openai.FinishReasonStop}}}
			})})
			(&toolset{registry: engine.Registry}).registerContextTools()
			tool, _ := engine.Registry.GetTool("get_user_avatar")
			original := tool.ResultHandler
			tool.ResultHandler = func(c *ai.ToolContext) ai.ToolResult {
				result := original(c)
				if result.Status == ai.ToolSucceeded {
					urls, err := c.ImageURLs([]string{"image_1"})
					if err != nil || len(urls) != 1 {
						t.Fatalf("avatar was not registered as an image: %v %v", urls, err)
					}
					u, err := url.Parse(urls[0])
					if err != nil || u.Scheme != "https" || u.Host != "q.qlogo.cn" || u.Query().Get("dst_uin") != strconv.FormatInt(tc.wantUser, 10) {
						t.Errorf("wrong avatar source: %s", urls[0])
					}
				}
				return result
			}
			message := bot.Msg().Text("用头像生成一张水彩画")
			for _, id := range tc.mentions {
				message.At(id)
			}
			event := &bot.GroupMessageEvent{SelfID: 1, GroupID: 100, UserID: 42, MessageID: 123, Message: message.Build(), Sender: bot.Sender{Role: "member"}}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := engine.Dispatch(ctx, &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { cancel(); return nil }); err != nil {
				t.Fatal(err)
			}
			if calls != 2 {
				t.Fatalf("model calls = %d, want 2", calls)
			}
		})
	}
}

type avatarTransport func(*http.Request) (*http.Response, error)

func (f avatarTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMentionedAvatarIsSentAsImageEditReference(t *testing.T) {
	for _, count := range []int{1, 3} {
		t.Run(strconv.Itoa(count)+" references", func(t *testing.T) { testMentionedAvatarWithReferences(t, count) })
	}
}

func testMentionedAvatarWithReferences(t *testing.T, count int) {
	old, oldPublic := config.C, httpclient.Public
	t.Cleanup(func() { config.C, httpclient.Public = old, oldPublic })
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	sources := [][]byte{picture.Bytes()}
	for i := 1; i < count; i++ {
		img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
		img.SetNRGBA(0, 0, color.NRGBA{R: uint8(80 * i), A: 255})
		var data bytes.Buffer
		if err := png.Encode(&data, img); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, data.Bytes())
	}
	prompt := "画成水彩头像，保留人物特征"
	if count > 1 {
		prompt += "；参考图1取面部，参考图2和3取服装的正背面"
	}
	var reacted atomic.Bool
	var downloads, edits, sends atomic.Int32
	httpclient.Public = &httpclient.Client{Client: &http.Client{Transport: avatarTransport(func(r *http.Request) (*http.Response, error) {
		sourceIndex := 0
		if r.URL.Host == "references.example" {
			var err error
			sourceIndex, err = strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/"))
			if err != nil || sourceIndex <= 0 || sourceIndex >= count {
				t.Fatal("invalid attachment download")
			}
		} else if r.URL.Host != "q.qlogo.cn" || r.URL.Query().Get("dst_uin") != "100" {
			t.Errorf("wrong avatar target: %s", r.URL)
		}
		if !reacted.Load() {
			t.Errorf("wrong avatar download or order: %s", r.URL)
		}
		downloads.Add(1)
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"image/png"}}, Body: io.NopCloser(bytes.NewReader(sources[sourceIndex]))}, nil
	})}}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		edits.Add(1)
		if r.URL.Path != "/images/edits" || r.ParseMultipartForm(1<<20) != nil {
			t.Error("avatar request must use multipart image editing, not text-only generation")
			http.Error(w, "invalid edit", http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()
		field := "image"
		if count > 1 {
			field = "image[]"
		}
		files := r.MultipartForm.File[field]
		if len(files) != count {
			t.Errorf("provider got %d references, want %d", len(files), count)
			return
		}
		for i, file := range files {
			f, err := file.Open()
			if err != nil {
				t.Error(err)
				return
			}
			source, _ := io.ReadAll(f)
			f.Close()
			if !bytes.Equal(source, sources[i]) {
				t.Error("avatar and view image ordering changed")
			}
		}
		if !strings.HasPrefix(r.FormValue("prompt"), prompt) || r.FormValue("model") != "drawing" {
			t.Error("model or requested reference roles did not reach the image provider")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(picture.Bytes())}}})
	}))
	defer provider.Close()
	config.C.AI = config.AIConfig{Connections: []config.AIConnection{{ID: "fixture", Kind: "openai", BaseURL: provider.URL}}, Profiles: []config.AIProfile{
		{ID: "chat", ConnectionID: "fixture", Model: "chat", Capabilities: []string{"chat", "tools"}},
		{ID: "image", ConnectionID: "fixture", Model: "drawing", Capabilities: []string{"image_edit"}},
	}, ConversationProfile: "chat", Bindings: map[string]string{"image_edit": "image"}}
	config.C.Bot.SuperUsers = nil
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
				reacted.Store(true)
			case "send_group_msg":
				var message bot.Message
				_ = json.Unmarshal(req.Params["message"], &message)
				reply, _ := message.ReplyID()
				if !message.HasType("image") || reply != "123" || string(req.Params["group_id"]) != "100" {
					t.Error("generated image was not sent to the originating group and message")
				}
				sends.Add(1)
				data["message_id"] = 456
			default:
				t.Errorf("unexpected OneBot action %s", req.Action)
			}
			if peer.WriteJSON(map[string]any{"echo": req.Echo, "status": "ok", "retcode": 0, "data": data}) != nil {
				return
			}
		}
	}()
	defer func() { peer.Close(); <-done }()
	calls := 0
	avatarID := "image_" + strconv.Itoa(count)
	engine := ai.NewEngine(ai.EngineDependencies{ModelBackend: imageRequestModel(func(req openai.ChatCompletionRequest) openai.ChatCompletionResponse {
		calls++
		msg := openai.ChatCompletionMessage{Role: "assistant"}
		var result ai.ToolResult
		if calls > 1 {
			_ = json.Unmarshal([]byte(req.Messages[len(req.Messages)-1].Content), &result)
			if result.Status != ai.ToolSucceeded {
				t.Errorf("step %d failed: %+v", calls, result)
			}
		}
		name, args := "discover_available_tools", `{"names":["edit_image"]}`
		switch calls {
		case 1:
		case 2:
			avatarDefinitionAvailable := false
			for _, message := range req.Messages {
				avatarDefinitionAvailable = avatarDefinitionAvailable || strings.Contains(message.Content, `"name":"get_user_avatar"`)
			}
			if !avatarDefinitionAvailable {
				t.Error("edit_image requires an available avatar definition, either preloaded or newly discovered")
			}
			name, args = "invoke_available_tool", `{"name":"get_user_avatar","arguments":"{}"}`
		case 3:
			if !strings.Contains(result.Content, "image_id="+avatarID) || downloads.Load() != 0 {
				t.Error("avatar must first become a session reference, not an eager network request")
			}
			params := map[string]any{"image_id": avatarID, "prompt": prompt}
			if count > 1 {
				delete(params, "image_id")
				params["image_ids"] = []string{avatarID, "image_1", "image_2"}
			}
			arguments, _ := json.Marshal(params)
			invocation, _ := json.Marshal(map[string]string{"name": "edit_image", "arguments": string(arguments)})
			name, args = "invoke_available_tool", string(invocation)
		case 4:
			msg.Content = "已完成头像绘制。"
		default:
			t.Error("unexpected model call")
		}
		if msg.Content == "" {
			msg.ToolCalls = []openai.ToolCall{{ID: strconv.Itoa(calls), Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: name, Arguments: args}}}
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: msg, FinishReason: openai.FinishReasonStop}}}
	})})
	RegisterAll(engine.Registry, Dependencies{})
	message := bot.Msg().At(1).Text("用 ").At(100).Text(" 的头像生图，画成水彩头像，保留人物特征")
	for i := 1; i < count; i++ {
		message.Image("https://references.example/" + strconv.Itoa(i))
	}
	event := &bot.GroupMessageEvent{SelfID: 1, GroupID: 100, UserID: 42, MessageID: 123, Message: message.Build(), Sender: bot.Sender{Role: "member"}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := engine.Dispatch(ctx, &bot.GroupContext{BotAPI: api, MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { cancel(); return nil }); err != nil {
		t.Fatal(err)
	}
	if calls != 4 || downloads.Load() != int32(count) || edits.Load() != 1 || sends.Load() != 1 {
		t.Fatalf("calls=%d downloads=%d edits=%d sends=%d", calls, downloads.Load(), edits.Load(), sends.Load())
	}
}
