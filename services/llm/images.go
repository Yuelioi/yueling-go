package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/services/imageassets"
	"github.com/Yuelioi/yueling-go/services/imaging"
	openai "github.com/sashabaranov/go-openai"
)

type ImageOptions struct {
	TransparentBackground bool
}

func (c *Client) Image(ctx context.Context, prompt string, source []byte, options ...ImageOptions) ([]byte, error) {
	if c.settings.configError != nil || c.settings.Model == "" || strings.TrimSpace(prompt) == "" || len(prompt) > 16000 {
		return nil, &Error{Kind: Configuration}
	}
	if err := ctx.Err(); err != nil {
		return nil, classify(err)
	}
	var opts ImageOptions
	if len(options) > 1 {
		return nil, &Error{Kind: Configuration}
	}
	if len(options) == 1 {
		opts = options[0]
	}
	if source != nil {
		if !validGeneratedImage(source) {
			return nil, &Error{Kind: InvalidResponse, Detail: "invalid_source_image"}
		}
		_, format, _ := image.DecodeConfig(bytes.NewReader(source))
		if format == "gif" {
			animation, err := imaging.Run(ctx, func() (*imaging.Animation, error) { return imaging.Decode(source, gifEditLimits()) })
			if err != nil {
				return nil, &Error{Kind: Configuration, Detail: "gif_edit_limits"}
			}
			if animation.Animated() {
				return c.editAnimation(ctx, prompt, animation, opts)
			}
			source, err = imaging.EncodePNG(animation.Frames[0], gifEditLimits())
			if err != nil {
				return nil, &Error{Kind: InvalidResponse, Detail: "invalid_source_image"}
			}
		}
	}
	return c.imageSingle(ctx, prompt, source, opts)
}

func (c *Client) imageSingle(ctx context.Context, prompt string, source []byte, opts ImageOptions) ([]byte, error) {
	if c.settings.configError != nil || c.settings.Model == "" {
		return nil, &Error{Kind: Configuration}
	}
	if opts.TransparentBackground {
		prompt += "\n输出真正带透明Alpha通道的PNG，背景必须透明；保留主体，不能绘制棋盘格、白底或纯色背景来代替透明。"
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout(c.settings))
	defer cancel()
	if c.settings.Kind == "codex" {
		if source != nil && !validGeneratedImage(source) {
			return nil, &Error{Kind: InvalidResponse, Detail: "invalid_source_image"}
		}
		message := openai.ChatCompletionMessage{Role: "user", Content: prompt}
		if source != nil {
			message.Content = ""
			message.MultiContent = []openai.ChatMessagePart{{Type: openai.ChatMessagePartTypeText, Text: prompt}, {Type: openai.ChatMessagePartTypeImageURL, ImageURL: &openai.ChatMessageImageURL{URL: "data:" + http.DetectContentType(source) + ";base64," + base64.StdEncoding.EncodeToString(source)}}}
		}
		var imageData []byte
		started := time.Now()
		response, err := (&codexBackend{settings: c.settings}).complete(ctx, c.prepare(openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{message}}), &imageData)
		if err != nil {
			return nil, classify(err)
		}
		observeUsage(withUsageSource(ctx, c.settings), c.settings.Model, response.Usage, time.Since(started))
		return checkedImageResult(ctx, imageData, opts)
	}
	endpoint := "/images/generations"
	contentType := "application/json"
	var body bytes.Buffer
	if source == nil {
		fields := map[string]any{"model": c.settings.Model, "prompt": prompt, "n": 1}
		if opts.TransparentBackground {
			fields["background"] = "transparent"
			fields["output_format"] = "png"
		}
		_ = json.NewEncoder(&body).Encode(fields)
	} else {
		if !validGeneratedImage(source) {
			return nil, &Error{Kind: InvalidResponse, Detail: "invalid_source_image"}
		}
		endpoint = "/images/edits"
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("model", c.settings.Model)
		_ = writer.WriteField("prompt", prompt)
		_ = writer.WriteField("n", "1")
		if opts.TransparentBackground {
			_ = writer.WriteField("background", "transparent")
			_ = writer.WriteField("output_format", "png")
		}
		_, format, _ := image.DecodeConfig(bytes.NewReader(source))
		part, err := writer.CreateFormFile("image", "source."+format)
		if err != nil {
			return nil, &Error{Kind: Configuration}
		}
		if _, err = part.Write(source); err != nil {
			return nil, &Error{Kind: Configuration}
		}
		_ = writer.Close()
		contentType = writer.FormDataContentType()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.settings.BaseURL, "/")+endpoint, &body)
	if err != nil {
		return nil, &Error{Kind: Configuration}
	}
	req.Header.Set("Content-Type", contentType)
	if c.settings.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.settings.APIKey)
	}
	started := time.Now()
	client := &http.Client{Timeout: requestTimeout(c.settings), CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return nil, classify(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, classify(&openai.APIError{HTTPStatusCode: res.StatusCode})
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 24<<20))
	if err != nil || len(data) == 24<<20 {
		return nil, &Error{Kind: InvalidResponse}
	}
	var result struct {
		Data []struct {
			Base64 string `json:"b64_json"`
			URL    string `json:"url"`
		} `json:"data"`
		Usage struct {
			Input  int `json:"input_tokens"`
			Output int `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(data, &result) != nil || len(result.Data) != 1 {
		return nil, &Error{Kind: InvalidResponse}
	}
	observeUsage(withUsageSource(ctx, c.settings), c.settings.Model, openai.Usage{PromptTokens: result.Usage.Input, CompletionTokens: result.Usage.Output}, time.Since(started))
	item := result.Data[0]
	if item.Base64 != "" {
		data, err = base64.StdEncoding.DecodeString(item.Base64)
	} else if item.URL != "" {
		data, err = imageassets.Fetch(ctx, item.URL)
	} else {
		return nil, &Error{Kind: InvalidResponse}
	}
	if err != nil || !validGeneratedImage(data) {
		return nil, &Error{Kind: InvalidResponse, Detail: "invalid_image"}
	}
	return checkedImageResult(ctx, data, opts)
}

func validGeneratedImage(data []byte) bool {
	if len(data) == 0 || len(data) > imageassets.MaxBytes {
		return false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
		return false
	}
	_, _, err = image.Decode(bytes.NewReader(data))
	return err == nil
}

func checkedImageResult(ctx context.Context, data []byte, opts ImageOptions) ([]byte, error) {
	if !validGeneratedImage(data) {
		return nil, &Error{Kind: InvalidResponse, Detail: "invalid_image"}
	}
	return imaging.Run(ctx, func() ([]byte, error) {
		img, format, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, &Error{Kind: InvalidResponse, Detail: "invalid_image"}
		}
		if format == "gif" {
			animation, err := imaging.Decode(data, gifEditLimits())
			if err != nil || animation.Animated() {
				return nil, &Error{Kind: InvalidResponse, Detail: "unexpected_animation"}
			}
		}
		if !opts.TransparentBackground {
			return data, nil
		}
		transparent, visible := false, false
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				_, _, _, alpha := img.At(x, y).RGBA()
				transparent = transparent || alpha < 0xffff
				visible = visible || alpha > 0
			}
		}
		if !transparent || !visible {
			return nil, &Error{Kind: InvalidResponse, Detail: "transparency_missing"}
		}
		return imaging.EncodePNG(img, imaging.DefaultLimits)
	})
}
