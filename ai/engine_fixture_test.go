package ai

import (
	"net/http"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

var testEngine = NewEngine(EngineDependencies{})

func newTestModelClient(key, baseURL string) *openai.Client {
	cfg := openai.DefaultConfig(key)
	cfg.BaseURL = baseURL
	cfg.HTTPClient = &http.Client{Timeout: 45 * time.Second}
	return openai.NewClientWithConfig(cfg)
}
