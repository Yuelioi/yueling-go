package config

import (
	"fmt"
	"maps"
	"slices"
)

type AIConnection struct {
	ID      string `toml:"id" json:"id"`
	Name    string `toml:"name" json:"name"`
	Kind    string `toml:"kind" json:"kind"`
	BaseURL string `toml:"base_url" json:"base_url"`
	APIKey  string `toml:"api_key" json:"api_key"`
}

type AIProfile struct {
	ConnectionID    string   `toml:"connection_id" json:"connection_id"`
	Capabilities    []string `toml:"capabilities" json:"capabilities"`
	Disabled        bool     `toml:"disabled" json:"disabled"`
	MaxTokens       int      `toml:"max_tokens" json:"max_tokens"`
	TimeoutSeconds  int      `toml:"timeout_seconds" json:"timeout_seconds"`
	ReasoningEffort string   `toml:"reasoning_effort" json:"reasoning_effort"`
	ID              string   `toml:"id" json:"id"`
	Name            string   `toml:"name" json:"name"`
	BaseURL         string   `toml:"base_url" json:"base_url"`
	Model           string   `toml:"model" json:"model"`
	APIKey          string   `toml:"api_key" json:"api_key"`
	Vision          bool     `toml:"vision" json:"vision"`
}

// Nil profiles preserve legacy bootstrap input; an empty list explicitly disables both roles.
func (a AIConfig) Resolved() AIConfig {
	if a.Profiles == nil {
		return a
	}
	a = a.Clone()
	a.DeepSeekKey, a.BaseURL, a.Model = "", "", ""
	a.VL = VLConfig{}
	for _, p := range a.Profiles {
		if p.ConnectionID != "" {
			for _, conn := range a.Connections {
				if conn.ID == p.ConnectionID {
					p.APIKey, p.BaseURL = conn.APIKey, conn.BaseURL
				}
			}
		}
		if p.ID == a.ConversationProfile && !p.Disabled {
			a.DeepSeekKey, a.BaseURL, a.Model = p.APIKey, p.BaseURL, p.Model
		}
		if p.ID == a.VisionProfile && p.Supports("vision") && !p.Disabled {
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

func (a AIConfig) Clone() AIConfig {
	a.Connections = slices.Clone(a.Connections)
	a.Bindings = maps.Clone(a.Bindings)
	a.Profiles = slices.Clone(a.Profiles)
	for i := range a.Profiles {
		a.Profiles[i].Capabilities = slices.Clone(a.Profiles[i].Capabilities)
	}
	return a
}

func (p AIProfile) Supports(capability string) bool {
	if p.Capabilities == nil {
		return capability == "chat" || capability == "tools" || capability == "vision" && p.Vision
	}
	return slices.Contains(p.Capabilities, capability)
}

type AIFeature struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
}

func AIFeatures() []AIFeature {
	return []AIFeature{
		{"conversation", "Bot 对话与操作", []string{"chat", "tools"}},
		{"vision", "图片理解", []string{"chat", "vision"}},
		{"image_generation", "生成图片", []string{"image_generation"}},
		{"image_edit", "编辑图片", []string{"image_edit"}},
		{"summary", "群聊总结", []string{"chat"}},
		{"digest", "群聊日报", []string{"chat"}},
		{"memory", "记忆提取", []string{"chat"}},
		{"compression", "上下文压缩", []string{"chat"}},
		{"knowledge", "知识整理", []string{"chat"}},
		{"proactive", "主动发言", []string{"chat"}},
		{"translation", "订阅翻译", []string{"chat"}},
		{"bilibili", "B站私信回复", []string{"chat"}},
		{"zssm", "zssm 回复", []string{"chat"}},
	}
}

func (a AIConfig) Select(feature string) (AIProfile, AIConnection, error) {
	var required []string
	for _, f := range AIFeatures() {
		if f.ID == feature {
			required = f.Capabilities
		}
	}
	if required == nil {
		return AIProfile{}, AIConnection{}, fmt.Errorf("未知 AI 功能")
	}
	id := a.Bindings[feature]
	if id == "" {
		switch feature {
		case "vision":
			id = a.VisionProfile
		case "image_generation", "image_edit":
			id = "off"
		default:
			id = a.ConversationProfile
		}
	}
	if id == "off" {
		return AIProfile{}, AIConnection{}, fmt.Errorf("AI 功能已关闭")
	}
	if a.Profiles == nil {
		p := AIProfile{Model: a.Model, APIKey: a.DeepSeekKey, BaseURL: a.BaseURL}
		if feature == "vision" {
			p.Model, p.APIKey, p.BaseURL, p.Vision = a.VL.Model, a.VL.Key, a.VL.BaseURL, true
		}
		return p, AIConnection{Kind: "openai", APIKey: p.APIKey, BaseURL: p.BaseURL}, nil
	}
	for _, p := range a.Profiles {
		if p.ID != id {
			continue
		}
		if p.Disabled {
			return AIProfile{}, AIConnection{}, fmt.Errorf("模型配置已停用")
		}
		for _, c := range required {
			if !p.Supports(c) {
				return AIProfile{}, AIConnection{}, fmt.Errorf("模型缺少所需能力：%s", c)
			}
		}
		if p.ConnectionID == "" {
			return p, AIConnection{Kind: "openai", BaseURL: p.BaseURL, APIKey: p.APIKey}, nil
		}
		for _, conn := range a.Connections {
			if conn.ID == p.ConnectionID {
				return p, conn, nil
			}
		}
		return AIProfile{}, AIConnection{}, fmt.Errorf("模型连接不存在")
	}
	return AIProfile{}, AIConnection{}, fmt.Errorf("尚未选择模型配置")
}

func (a AIConfig) Available(feature string) bool {
	p, _, err := a.Select(feature)
	return err == nil && p.Model != ""
}

func (a *AIConfig) ImportConnections() bool {
	if a.Connections != nil {
		return false
	}
	a.Connections = []AIConnection{}
	for i := range a.Profiles {
		p := &a.Profiles[i]
		if p.ConnectionID != "" {
			continue
		}
		p.ConnectionID = p.ID
		a.Connections = append(a.Connections, AIConnection{ID: p.ID, Name: p.Name, Kind: "openai", BaseURL: p.BaseURL, APIKey: p.APIKey})
		p.APIKey, p.BaseURL = "", ""
	}
	return true
}
