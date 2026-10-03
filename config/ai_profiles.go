package config

import "slices"

type AIProfile struct {
	ID      string `mapstructure:"id" json:"id"`
	Name    string `mapstructure:"name" json:"name"`
	BaseURL string `mapstructure:"base_url" json:"base_url"`
	Model   string `mapstructure:"model" json:"model"`
	APIKey  string `mapstructure:"api_key" json:"api_key"`
	Vision  bool   `mapstructure:"vision" json:"vision"`
}

// Nil profiles preserve legacy bootstrap input; an empty list explicitly disables both roles.
func (a AIConfig) Resolved() AIConfig {
	if a.Profiles == nil {
		return a
	}
	a.Profiles = slices.Clone(a.Profiles)
	a.DeepSeekKey, a.BaseURL, a.Model = "", "", ""
	a.VL = VLConfig{}
	for _, p := range a.Profiles {
		if p.ID == a.ConversationProfile {
			a.DeepSeekKey, a.BaseURL, a.Model = p.APIKey, p.BaseURL, p.Model
		}
		if p.ID == a.VisionProfile && p.Vision {
			a.VL = VLConfig{Key: p.APIKey, BaseURL: p.BaseURL, Model: p.Model}
		}
	}
	return a
}

func (a *AIConfig) ImportLegacyProfiles() bool {
	if a.Profiles != nil {
		return false
	}
	a.Profiles = []AIProfile{}
	if a.BaseURL != "" || a.Model != "" || a.DeepSeekKey != "" {
		a.Profiles = append(a.Profiles, AIProfile{ID: "legacy-chat", Name: "对话模型", BaseURL: a.BaseURL, Model: a.Model, APIKey: a.DeepSeekKey})
		a.ConversationProfile = "legacy-chat"
	}
	if a.VL.BaseURL != "" || a.VL.Model != "" || a.VL.Key != "" {
		a.Profiles = append(a.Profiles, AIProfile{ID: "legacy-vision", Name: "视觉模型", BaseURL: a.VL.BaseURL, Model: a.VL.Model, APIKey: a.VL.Key, Vision: true})
		a.VisionProfile = "legacy-vision"
	}
	*a = a.Resolved()
	return true
}
