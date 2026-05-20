package ai

import (
	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

var _client *openai.Client

func llm() *openai.Client {
	if _client == nil {
		c := config.C.AI
		cfg := openai.DefaultConfig(c.DeepSeekKey)
		cfg.BaseURL = c.BaseURL
		_client = openai.NewClientWithConfig(cfg)
	}
	return _client
}

// NewClient creates a standalone openai.Client for one-off calls (e.g. translate tool).
func NewClient(apiKey, baseURL string) *openai.Client {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	return openai.NewClientWithConfig(cfg)
}
