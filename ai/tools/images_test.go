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

func TestImageToolRoutingAndGuards(t *testing.T) {
	for _, input := range []string{"帮我上传这张图片到语录，名字是小明", "帮我看图中写了什么"} {
		routes := ai.Route(input, testEngine.Registry.AllTools())
		want := "save_quotation"
		if input == "帮我看图中写了什么" {
			want = "inspect_image"
		}
		if !containsRoutedTool(routes, want) {
			t.Fatalf("%s did not route to %s", input, want)
		}
	}
	tool, _ := testEngine.Registry.GetTool("save_quotation")
	if tool.PluginID != catalog.PluginUploadAssets || len(tool.Requires) != 1 || tool.Requires[0] != "inspect_image" {
		t.Fatal("missing permission or vision dependency")
	}
	for _, name := range []string{"", "../escape", "good"} {
		r := saveQuotation(&ai.ToolContext{Params: map[string]any{"name": name, "image_ids": []string{"image_1"}}})
		if r.Status != ai.ToolRejected || r.PossibleSideEffects {
			t.Fatal(r)
		}
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
