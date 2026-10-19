package runtimeconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/config"
)

var legacyAIPaths = map[string]bool{"ai.deepseek_key": true, "ai.base_url": true, "ai.model": true, "ai.vl.key": true, "ai.vl.base_url": true, "ai.vl.model": true}
var profileID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type profileView struct {
	config.AIProfile
	KeyConfigured bool `json:"key_configured"`
}

func profileViews(profiles []config.AIProfile) []profileView {
	result := make([]profileView, 0, len(profiles))
	for _, p := range profiles {
		configured := p.APIKey != ""
		p.APIKey = ""
		result = append(result, profileView{AIProfile: p, KeyConfigured: configured})
	}
	return result
}

type connectionView struct {
	config.AIConnection
	KeyConfigured bool `json:"key_configured"`
}

func connectionViews(connections []config.AIConnection) []connectionView {
	result := make([]connectionView, 0, len(connections))
	for _, c := range connections {
		configured := c.APIKey != ""
		c.APIKey = ""
		result = append(result, connectionView{c, configured})
	}
	return result
}

func decodeConnections(raw []byte, old []config.AIConnection) ([]config.AIConnection, error) {
	type input struct {
		config.AIConnection
		ClearKey bool `json:"clear_key"`
	}
	var inputs []input
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&inputs); err != nil || inputs == nil || len(inputs) > 32 {
		return nil, fmt.Errorf("连接配置必须是最多32项的有效列表")
	}
	keys := map[string]string{}
	for _, c := range old {
		keys[c.ID] = c.APIKey
	}
	result := make([]config.AIConnection, 0, len(inputs))
	for _, c := range inputs {
		if c.ClearKey && c.APIKey != "" {
			return nil, fmt.Errorf("不能同时替换和清除连接密钥")
		}
		if c.APIKey == "" && !c.ClearKey {
			c.APIKey = keys[c.ID]
		}
		result = append(result, c.AIConnection)
	}
	return result, nil
}

func decodeProfiles(raw []byte, old []config.AIProfile) ([]config.AIProfile, error) {
	type input struct {
		config.AIProfile
		ClearKey bool `json:"clear_key"`
	}
	var inputs []input
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&inputs); err != nil || inputs == nil {
		return nil, fmt.Errorf("模型配置必须为有效列表，不能含未知字段")
	}
	if len(inputs) > 32 {
		return nil, fmt.Errorf("最多保存 32 套模型配置")
	}
	keys := map[string]string{}
	for _, p := range old {
		keys[p.ID] = p.APIKey
	}
	result := make([]config.AIProfile, 0, len(inputs))
	for _, p := range inputs {
		if p.ClearKey && p.APIKey != "" {
			return nil, fmt.Errorf("不能同时替换和清除模型 Key")
		}
		if p.APIKey == "" && !p.ClearKey {
			p.APIKey = keys[p.ID]
		}
		result = append(result, p.AIProfile)
	}
	return result, nil
}

func validateProfiles(a config.AIConfig) error {
	if len(a.Profiles) > 32 {
		return fmt.Errorf("最多保存 32 套模型配置")
	}
	connections := map[string]config.AIConnection{}
	for _, c := range a.Connections {
		if !profileID.MatchString(c.ID) || c.ID == "off" || connections[c.ID].ID != "" || strings.TrimSpace(c.Name) == "" || len(c.Name) > 300 {
			return fmt.Errorf("连接 ID 或名称无效/重复")
		}
		if c.Kind != "openai" && c.Kind != "codex" {
			return fmt.Errorf("不支持的模型连接类型")
		}
		if c.Kind == "openai" {
			if err := validateModelURL(c.BaseURL); err != nil {
				return err
			}
		}
		if c.Kind == "codex" && (c.APIKey != "" || c.BaseURL != "") {
			return fmt.Errorf("Codex 使用独立登录态，无需 API 地址和密钥")
		}
		if len(c.APIKey) > 8192 || strings.ContainsAny(c.APIKey, "\r\n") {
			return fmt.Errorf("连接密钥无效")
		}
		connections[c.ID] = c
	}
	if len(connections) > 32 {
		return fmt.Errorf("最多保存32个连接")
	}
	seen := map[string]config.AIProfile{}
	for _, p := range a.Profiles {
		if !profileID.MatchString(p.ID) || p.ID == "off" {
			return fmt.Errorf("模型配置 ID 无效")
		}
		if _, ok := seen[p.ID]; ok {
			return fmt.Errorf("模型配置 ID 重复")
		}
		seen[p.ID] = p
		if strings.TrimSpace(p.Name) == "" || utf8.RuneCountInString(p.Name) > 100 {
			return fmt.Errorf("模型配置名称需为 1～100 字")
		}
		searchOnly := len(p.Capabilities) == 1 && p.Supports("web_search")
		if (!searchOnly && strings.TrimSpace(p.Model) == "") || len(p.Model) > 256 {
			return fmt.Errorf("模型名称不能为空且不能超过 256 字节")
		}
		if p.ConnectionID == "" {
			if err := validateModelURL(p.BaseURL); err != nil {
				return err
			}
		} else {
			c, ok := connections[p.ConnectionID]
			if !ok {
				return fmt.Errorf("模型 %s 引用的连接不存在", p.Name)
			}
			if p.APIKey != "" || p.BaseURL != "" {
				return fmt.Errorf("使用连接的模型不能重复填写地址和密钥")
			}
			if c.Kind == "codex" && p.Supports("structured_output") {
				return fmt.Errorf("Codex 接入暂不支持结构化输出约束")
			}
			if c.Kind == "codex" && p.Supports("web_search") {
				return fmt.Errorf("联网搜索需选择 DeepSeek API 账号，不能使用 Codex 订阅")
			}
		}
		if p.MaxTokens < 0 || p.MaxTokens > 200000 || p.TimeoutSeconds < 0 || p.TimeoutSeconds > 600 {
			return fmt.Errorf("模型参数超出范围")
		}
		if !slices.Contains([]string{"", "none", "minimal", "low", "medium", "high", "xhigh"}, p.ReasoningEffort) {
			return fmt.Errorf("推理强度无效")
		}
		for _, cap := range p.Capabilities {
			if !slices.Contains([]string{"chat", "vision", "tools", "structured_output", "image_generation", "image_edit", "web_search"}, cap) {
				return fmt.Errorf("未知模型能力")
			}
		}

		if len(p.APIKey) > 8192 || strings.ContainsAny(p.APIKey, "\r\n") {
			return fmt.Errorf("模型 Key 无效")
		}
	}
	if a.ConversationProfile != "" {
		if p, ok := seen[a.ConversationProfile]; !ok || p.Disabled || !p.Supports("chat") || !p.Supports("tools") {
			return fmt.Errorf("请选择存在的对话模型配置")
		}
	}
	if a.VisionProfile != "" {
		p, ok := seen[a.VisionProfile]
		if !ok || p.Disabled || !p.Supports("vision") || !p.Supports("chat") {
			return fmt.Errorf("视觉模型必须选择勾选了支持视觉的配置")
		}
	}
	for feature, id := range a.Bindings {
		known := false
		for _, f := range config.AIFeatures() {
			if f.ID == feature {
				known = true
			}
		}
		if !known {
			return fmt.Errorf("未知 AI 功能：%s", feature)
		}
		if id != "" && id != "off" {
			if _, _, err := a.Select(feature); err != nil {
				return fmt.Errorf("%s: %w", feature, err)
			}
		}
	}

	return nil
}

func validateProfileUpdateMix(req Update) error {
	legacy := false
	for path := range req.Values {
		legacy = legacy || legacyAIPaths[path]
	}
	for _, path := range req.ClearSecrets {
		legacy = legacy || legacyAIPaths[path]
	}
	if !legacy {
		return nil
	}
	for _, path := range []string{"ai.profiles", "ai.connections", "ai.bindings", "ai.conversation_profile", "ai.vision_profile"} {
		if _, ok := req.Values[path]; ok {
			return fmt.Errorf("不能混用旧模型字段与 Profile 字段")
		}
	}
	return nil
}

// Legacy API writes target the selected profile so older clients keep working.
func applyLegacyAIUpdate(a *config.AIConfig, req Update) error {
	if a.Profiles == nil {
		return nil
	}
	for path := range legacyAIPaths {
		raw, changed := req.Values[path]
		clear := slices.Contains(req.ClearSecrets, path)
		if !changed && !clear {
			continue
		}
		var value string
		if changed {
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("模型字段必须是文本")
			}
		}
		if secrets[path] && value == "" && !clear {
			continue
		}
		target := a.ConversationProfile
		if strings.HasPrefix(path, "ai.vl.") {
			target = a.VisionProfile
		}
		found := false
		for i := range a.Profiles {
			p := &a.Profiles[i]
			if p.ID != target {
				continue
			}
			found = true
			switch path {
			case "ai.deepseek_key", "ai.vl.key":
				if p.ConnectionID == "" {
					p.APIKey = value
				} else {
					for j := range a.Connections {
						if a.Connections[j].ID == p.ConnectionID {
							a.Connections[j].APIKey = value
						}
					}
				}
			case "ai.base_url", "ai.vl.base_url":
				if p.ConnectionID == "" {
					p.BaseURL = value
				} else {
					for j := range a.Connections {
						if a.Connections[j].ID == p.ConnectionID {
							a.Connections[j].BaseURL = value
						}
					}
				}
			case "ai.model", "ai.vl.model":
				p.Model = value
			}
		}
		if !found {
			return fmt.Errorf("请先选择对应的模型配置")
		}
	}
	return nil
}

func validateModelURL(value string) error {
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("模型 API 地址必须是 HTTP/HTTPS 地址，不能包含凭据、查询参数或片段")
	}
	return nil
}
