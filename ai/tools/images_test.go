package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
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
