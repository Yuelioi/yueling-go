// Package testconfig creates explicit model bindings for deterministic fixtures.
package testconfig

import "github.com/Yuelioi/yueling-go/config"

func AI(model, endpoint, key string) config.AIConfig {
	a := config.AIConfig{MaxTokens: 4096, ReplyMaxChars: 200, Context: config.ContextConfig{ChatHistory: 15, Summary: 50}, Affinity: config.AffinityConfig{Max: 100, Initial: 50}, Profiles: []config.AIProfile{{ID: "fixture", Name: "fixture", ConnectionID: "fixture", Model: model, Capabilities: []string{"chat", "tools", "vision"}}}, Connections: []config.AIConnection{{ID: "fixture", Name: "fixture", Kind: "openai", BaseURL: endpoint, APIKey: key}}, Bindings: map[string]string{}}
	for _, f := range config.AIFeatures() {
		a.Bindings[f.ID] = "fixture"
	}
	return a
}
func Set(a *config.AIConfig, field, value string) {
	if len(a.Profiles) == 0 || len(a.Connections) == 0 {
		*a = AI("fixture", "http://fixture.invalid", "fixture")
	}
	switch field {
	case "Model":
		a.Profiles[0].Model = value
	case "BaseURL":
		a.Connections[0].BaseURL = value
	case "DeepSeekKey":
		a.Connections[0].APIKey = value
	}
}
