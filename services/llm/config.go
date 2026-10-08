package llm

import "github.com/Yuelioi/yueling-go/config"

func FromConfig(c config.AIConfig) Settings {
	return Settings{APIKey: c.DeepSeekKey, BaseURL: c.BaseURL, Model: c.Model, MaxTokens: c.MaxTokens, ReasoningEffort: c.ReasoningEffort}
}
