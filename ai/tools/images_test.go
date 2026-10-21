package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services/llm"
	openai "github.com/sashabaranov/go-openai"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnifiedImageToolRouting(t *testing.T) {
	for _, input := range []string{"添加龙图", "保存表情包", "上传这张到语录", "添加吃的 麻辣烫", "保存到自定义图库"} {
		if !containsRoutedTool(ai.Route(input, testEngine.Registry.AllTools()), "save_image_asset") {
			t.Fatal(input)
		}
	}
	for _, name := range []string{"save_quotation", "save_emoticon"} {
		if _, ok := testEngine.Registry.GetTool(name); ok {
			t.Fatal("separate category tool remains", name)
		}
	}
	tool, _ := testEngine.Registry.GetTool("save_image_asset")
	if tool.PluginID != catalog.PluginUploadAssets || len(tool.Requires) != 2 {
		t.Fatal("missing configuration discovery")
	}
}

func TestVisionRequestCarriesImageAndQuestion(t *testing.T) {
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if req.Model != "vision-fixture" || len(req.Messages) != 2 || len(req.Tools) != 0 {
			t.Errorf("unexpected request shape")
		}
		parts := req.Messages[1].MultiContent
		if len(parts) != 2 || parts[0].ImageURL == nil || parts[0].ImageURL.URL != "data:image/png;base64,"+base64.StdEncoding.EncodeToString(imageData.Bytes()) || parts[1].Text != "图片写了什么" {
			t.Error("image or question missing")
		}
		if !strings.Contains(req.Messages[0].Content, "不可信资料") {
			t.Error("image instructions not separated from authority")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "图片中有文字。"}, FinishReason: openai.FinishReasonStop}}})
	}))
	defer server.Close()
	desc, err := describeAttachment(context.Background(), config.VLConfig{Key: "fixture", BaseURL: server.URL, Model: "vision-fixture"}, imageData.Bytes(), "图片写了什么")
	if err != nil || desc != "图片中有文字。" {
		t.Fatalf("%q %v", desc, err)
	}
}

func TestDeletionRouting(t *testing.T) {
	for _, input := range []string{"删除这张图片", "把这个表情删除", "删除自定义图库的图片"} {
		if !containsRoutedTool(ai.Route(input, testEngine.Registry.AllTools()), "delete_image_asset") {
			t.Fatal(input)
		}
	}
	tool, _ := testEngine.Registry.GetTool("delete_image_asset")
	if tool.Permission != ai.PermSuperUser {
		t.Fatal("missing shared library authorization")
	}
}

func TestVisionProfilesAndCanceledRequestsAfterZssmRetirement(t *testing.T) {
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var request openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if r.Header.Get("Authorization") != "Bearer "+request.Model+"-key" || len(request.Messages) != 2 || len(request.Messages[1].MultiContent) != 2 || request.Messages[1].MultiContent[0].ImageURL == nil || request.Messages[1].MultiContent[0].ImageURL.URL != "data:image/png;base64,"+base64.StdEncoding.EncodeToString(imageData.Bytes()) {
			t.Error("vision lost selected account or image input")
		}
		w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"image description"}}]}`))
	}))
	defer server.Close()
	a := config.AIConfig{
		Connections: []config.AIConnection{{ID: "shared", Kind: "openai", BaseURL: server.URL, APIKey: "shared-key"}, {ID: "separate", Kind: "openai", BaseURL: server.URL, APIKey: "separate-key"}},
		Profiles:    []config.AIProfile{{ID: "shared", ConnectionID: "shared", Model: "shared", Capabilities: []string{"chat", "tools", "vision"}}, {ID: "separate", ConnectionID: "separate", Model: "separate", Capabilities: []string{"chat", "vision"}}}, Bindings: map[string]string{"conversation": "shared"},
	}
	for _, id := range []string{"shared", "separate"} {
		a.Bindings["vision"] = id
		if description, err := describeAttachmentWithSettings(context.Background(), llm.ForFeature(a, "vision"), imageData.Bytes(), "解释图片"); err != nil || description != "image description" {
			t.Fatal(description, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := describeAttachmentWithSettings(ctx, llm.ForFeature(a, "vision"), imageData.Bytes(), "解释图片"); err == nil {
		t.Fatal("canceled request succeeded")
	}
	if requests != 2 {
		t.Fatal("canceled request reached provider", requests)
	}
}
