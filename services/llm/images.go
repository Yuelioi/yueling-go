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
	openai "github.com/sashabaranov/go-openai"
)

func (c *Client) Image(ctx context.Context, prompt string, source []byte) ([]byte, error) {
	if c.settings.configError != nil || c.settings.Kind == "codex" || c.settings.Model == "" {
		return nil, &Error{Kind: Configuration}
	}
	if strings.TrimSpace(prompt) == "" || len(prompt) > 16000 {
		return nil, &Error{Kind: Configuration}
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout(c.settings))
	defer cancel()
	endpoint := "/images/generations"
	contentType := "application/json"
	var body bytes.Buffer
	if source == nil {
		_ = json.NewEncoder(&body).Encode(map[string]any{"model": c.settings.Model, "prompt": prompt, "n": 1})
	} else {
		if !validGeneratedImage(source) {
			return nil, &Error{Kind: InvalidResponse, Detail: "invalid_source_image"}
		}
		endpoint = "/images/edits"
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("model", c.settings.Model)
		_ = writer.WriteField("prompt", prompt)
		_ = writer.WriteField("n", "1")
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
	return data, nil
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
