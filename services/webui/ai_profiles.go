package webui

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
	"strings"
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/codexauth"
	"github.com/Yuelioi/yueling-go/services/llm"
	"github.com/gin-gonic/gin"
	openai "github.com/sashabaranov/go-openai"
)

var profileTests sync.Map

func (s *Server) handleAIFeatures(c *gin.Context) {
	c.JSON(200, gin.H{"features": config.AIFeatures()})
}
func codexConnection(c *gin.Context) (config.Config, bool) {
	cfg := config.Current()
	for _, conn := range cfg.AI.Connections {
		if conn.ID == c.Param("id") && conn.Kind == "codex" {
			return cfg, true
		}
	}
	jsonError(c, 404, "Codex 连接不存在")
	return cfg, false
}
func (s *Server) handleCodexStatus(c *gin.Context) {
	cfg, ok := codexConnection(c)
	if !ok {
		return
	}
	c.JSON(200, codexauth.ReadStatus(cfg.Bot.DataDir, c.Param("id")))
}
func (s *Server) handleCodexImport(c *gin.Context) {
	cfg, ok := codexConnection(c)
	if !ok {
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10))
	if err != nil {
		jsonError(c, 400, "登录文件过大或读取失败")
		return
	}
	if err = llm.ImportCodexAuth(cfg.Bot.DataDir, c.Param("id"), data, c.Query("replace") == "true"); err != nil {
		jsonError(c, 400, err.Error())
		return
	}
	c.JSON(200, codexauth.ReadStatus(cfg.Bot.DataDir, c.Param("id")))
}

func testImage() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

func (s *Server) handleAIProfileTest(c *gin.Context) {
	id := c.Param("id")
	if _, busy := profileTests.LoadOrStore(id, true); busy {
		jsonError(c, 409, "该模型正在测试")
		return
	}
	defer profileTests.Delete(id)
	var input struct {
		Capability string `json:"capability"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		jsonError(c, 400, "请选择要测试的能力")
		return
	}
	cfg := config.Current().AI
	var profile *config.AIProfile
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ID == id {
			profile = &cfg.Profiles[i]
			break
		}
	}
	if profile == nil {
		jsonError(c, 404, "模型配置不存在，请先保存")
		return
	}
	if !profile.Supports(input.Capability) {
		jsonError(c, 400, "该配置没有声明此能力")
		return
	}
	feature := map[string]string{"chat": "summary", "vision": "vision", "tools": "conversation", "structured_output": "summary", "image_generation": "image_generation", "image_edit": "image_edit"}[input.Capability]
	if feature == "" {
		jsonError(c, 400, "未知能力")
		return
	}
	cfg = cfg.Clone()
	if cfg.Bindings == nil {
		cfg.Bindings = map[string]string{}
	}
	cfg.Bindings[feature] = id
	settings := llm.ForFeature(cfg, feature)
	// Return a useful test result before the reverse proxy stops waiting.
	imageTest := feature == "image_generation" || feature == "image_edit"
	if !imageTest && settings.Kind == "codex" && settings.Timeout > 45*time.Second {
		settings.Timeout = 45 * time.Second
	}
	client := llm.New(settings)
	if !imageTest {
		c.JSON(200, runAIProfileTest(c.Request.Context(), client, input.Capability, feature))
		return
	}
	result := make(chan gin.H, 1)
	ctx := c.Request.Context()
	go func() { result <- runAIProfileTest(ctx, client, input.Capability, feature) }()
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Header("X-Accel-Buffering", "no")
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case body := <-result:
			c.JSON(200, body)
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := c.Writer.Write([]byte("\n")); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}

func runAIProfileTest(ctx context.Context, client *llm.Client, capability, feature string) gin.H {
	started := time.Now()
	var imageData []byte
	var err error
	if feature == "image_generation" || feature == "image_edit" {
		var source []byte
		if feature == "image_edit" {
			source = testImage()
		}
		imageData, err = client.Image(ctx, "A plain blue square, no text.", source)
	} else {
		req := openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Reply with OK."}}, MaxTokens: 128}
		switch capability {
		case "vision":
			req.Messages = []openai.ChatCompletionMessage{{Role: "user", MultiContent: []openai.ChatMessagePart{{Type: openai.ChatMessagePartTypeText, Text: "What color is this square?"}, {Type: openai.ChatMessagePartTypeImageURL, ImageURL: &openai.ChatMessageImageURL{URL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(testImage())}}}}}
		case "tools":
			req.Messages[0].Content = "Call capability_probe with value OK. Do not reply with text."
			req.Tools = []openai.Tool{{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "capability_probe", Description: "A test that performs no action", Parameters: map[string]any{"type": "object", "properties": map[string]any{"value": map[string]string{"type": "string"}}, "required": []string{"value"}}}}}
		case "structured_output":
			req.Messages[0].Content = "Return JSON: {\"ok\":true}"
			req.ResponseFormat = &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject}
		}
		var response openai.ChatCompletionResponse
		response, err = client.Complete(ctx, req)
		if err == nil {
			if len(response.Choices) == 0 {
				err = &llm.Error{Kind: llm.InvalidResponse}
			} else {
				choice := response.Choices[0]
				err = llm.ValidateChoice(choice)
				if err == nil && capability == "chat" && strings.TrimSpace(choice.Message.Content) != "OK" {
					err = &llm.Error{Kind: llm.Compatibility, Detail: "probe_text_mismatch"}
				}
				if err == nil && capability == "tools" {
					if len(choice.Message.ToolCalls) != 1 || choice.Message.ToolCalls[0].Function.Name != "capability_probe" {
						err = &llm.Error{Kind: llm.Compatibility, Detail: "probe_tool_missing"}
					}
				}
				if err == nil && capability == "structured_output" && !json.Valid([]byte(choice.Message.Content)) {
					err = &llm.Error{Kind: llm.Compatibility}
				}
			}
		}
	}
	status, message := "verified", "本次请求通过；可用性仍取决于模型和账号状态。"
	if err != nil {
		status = "failed"
		message = llm.UserMessage(err)
	}
	result := gin.H{"status": status, "message": message, "capability": capability, "duration_ms": time.Since(started).Milliseconds()}
	if err == nil && len(imageData) > 0 {
		result["image_url"] = "data:" + http.DetectContentType(imageData) + ";base64," + base64.StdEncoding.EncodeToString(imageData)
	}
	return result
}
