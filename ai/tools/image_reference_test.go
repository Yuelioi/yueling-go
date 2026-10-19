package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"github.com/gorilla/websocket"
	openai "github.com/sashabaranov/go-openai"
)

func TestQuotedImageRenameThroughDispatch(t *testing.T) {
	oldConfig, oldRoot, oldPublic := config.C, services.DataDir, httpclient.Public
	t.Cleanup(func() { config.C, services.DataDir, httpclient.Public = oldConfig, oldRoot, oldPublic })
	services.DataDir = t.TempDir()
	config.C.AI = config.AIConfig{Model: "fixture", DeepSeekKey: "fixture"}
	config.C.Bot.SuperUsers = []int64{42}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data.Bytes()) }))
	defer imageServer.Close()
	httpclient.Public = &httpclient.Client{Client: imageServer.Client()}
	dir := filepath.Join(services.DataDir, "images", "表情")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "不知道旧名字.png")
	if err := os.WriteFile(file, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
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
	done := make(chan struct{})
	go func() {
		defer close(done)
		var req struct{ Action, Echo string }
		if err := peer.ReadJSON(&req); err != nil {
			t.Error(err)
			return
		}
		if req.Action != "get_msg" {
			t.Errorf("unexpected action %s", req.Action)
		}
		_ = peer.WriteJSON(map[string]any{"echo": req.Echo, "status": "ok", "retcode": 0, "data": map[string]any{"message": bot.Msg().Image(imageServer.URL + "/opaque-qq-url").Build()}})
	}()
	defer func() { peer.Close(); <-done }()
	var match struct {
		Assets []struct{ Category, ID, Name string }
		Total  int
	}
	calls := 0
	engine := ai.NewEngine(ai.EngineDependencies{ModelBackend: imageRequestModel(func(req openai.ChatCompletionRequest) openai.ChatCompletionResponse {
		calls++
		msg := openai.ChatCompletionMessage{Role: "assistant", Content: "处理完毕"}
		switch calls {
		case 1:
			if !strings.Contains(req.Messages[len(req.Messages)-1].Content, "image_1：引用消息第1张") {
				t.Error("missing quoted attachment")
			}
			msg.ToolCalls = []openai.ToolCall{{ID: "find", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "list_image_assets", Arguments: `{"image_id":"image_1"}`}}}
		case 2:
			var result ai.ToolResult
			_ = json.Unmarshal([]byte(req.Messages[len(req.Messages)-1].Content), &result)
			if err := json.Unmarshal([]byte(result.Content), &match); err != nil || result.Status != ai.ToolSucceeded || match.Total != 1 || len(match.Assets) != 1 {
				t.Errorf("quoted image could not locate stored asset: %+v (%v)", result, err)
				break
			}
			args, _ := json.Marshal(map[string]string{"category": match.Assets[0].Category, "asset_id": match.Assets[0].ID, "name": "新名字"})
			msg.ToolCalls = []openai.ToolCall{{ID: "rename", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "rename_image_asset", Arguments: string(args)}}}
		case 3:
			var result ai.ToolResult
			_ = json.Unmarshal([]byte(req.Messages[len(req.Messages)-1].Content), &result)
			if result.Status != ai.ToolSucceeded {
				t.Errorf("rename after attachment lookup failed: %+v", result)
			}
		default:
			t.Error("unexpected model call")
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: msg, FinishReason: openai.FinishReasonStop}}}
	})})
	(&toolset{registry: engine.Registry}).registerImageLibraryTools()
	event := &bot.GroupMessageEvent{SelfID: 1, GroupID: 100, UserID: 42, MessageID: 123, Message: bot.Msg().Reply(321).Text("把这张图片改名为新名字").Build(), Sender: bot.Sender{Role: "member"}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := engine.Dispatch(ctx, &bot.GroupContext{BotAPI: api, MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { cancel(); return nil }); err != nil {
		t.Fatal(err)
	}
	rows, err := currentImageLibrary().List(context.Background(), "表情", "新名字", 1)
	if err != nil || rows.Total != 1 || calls != 3 {
		t.Fatalf("quoted image was not renamed: calls=%d rows=%+v err=%v", calls, rows, err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("old asset remained active: %v", err)
	}
	trash, _ := filepath.Glob(filepath.Join(services.DataDir, "trash", "images", "表情", "*", "不知道旧名字.png"))
	if len(trash) != 0 {
		t.Fatal("rename should not recycle the original")
	}
}
