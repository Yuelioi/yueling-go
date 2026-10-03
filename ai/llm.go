package ai

import (
	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
	"sync"
)

var _client *openai.Client

var clientMu sync.Mutex
var clientKey, clientURL string

func llm() *openai.Client {
	client, _ := conversationClient()
	return client
}

func conversationClient() (*openai.Client, string) {
	clientMu.Lock()
	defer clientMu.Unlock()
	c := config.Current().AI
	if _client == nil || clientKey != c.DeepSeekKey || clientURL != c.BaseURL {
		cfg := openai.DefaultConfig(c.DeepSeekKey)
		cfg.BaseURL = c.BaseURL
		_client = openai.NewClientWithConfig(cfg)
		clientKey, clientURL = c.DeepSeekKey, c.BaseURL
	}
	return _client, c.Model
}

// NewClient creates a standalone openai.Client for one-off AI calls.
func NewClient(apiKey, baseURL string) *openai.Client {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	return openai.NewClientWithConfig(cfg)
}
