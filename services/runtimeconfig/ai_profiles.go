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
	ID            string `json:"id"`
	Name          string `json:"name"`
	BaseURL       string `json:"base_url"`
	Model         string `json:"model"`
	APIKey        string `json:"api_key"`
	Vision        bool   `json:"vision"`
	KeyConfigured bool   `json:"key_configured"`
}

func profileViews(profiles []config.AIProfile) []profileView {
	result := make([]profileView, 0, len(profiles))
	for _, p := range profiles {
		result = append(result, profileView{ID: p.ID, Name: p.Name, BaseURL: p.BaseURL, Model: p.Model, Vision: p.Vision, KeyConfigured: p.APIKey != ""})
	}
	return result
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
	if a.Profiles == nil {
		return nil
	}
	if len(a.Profiles) > 32 {
		return fmt.Errorf("最多保存 32 套模型配置")
	}
	seen := map[string]config.AIProfile{}
	for _, p := range a.Profiles {
		if !profileID.MatchString(p.ID) {
			return fmt.Errorf("模型配置 ID 无效")
		}
		if _, ok := seen[p.ID]; ok {
			return fmt.Errorf("模型配置 ID 重复")
		}
		seen[p.ID] = p
		if strings.TrimSpace(p.Name) == "" || utf8.RuneCountInString(p.Name) > 100 {
			return fmt.Errorf("模型配置名称需为 1～100 字")
		}
		if strings.TrimSpace(p.Model) == "" || len(p.Model) > 256 {
			return fmt.Errorf("模型名称不能为空且不能超过 256 字节")
		}
		u, err := url.Parse(p.BaseURL)
		if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("模型 API 地址必须是 HTTP/HTTPS 地址，不能包含凭据、查询参数或片段")
		}
		if len(p.APIKey) > 8192 || strings.ContainsAny(p.APIKey, "\r\n") {
			return fmt.Errorf("模型 Key 无效")
		}
	}
	if a.ConversationProfile != "" {
		if _, ok := seen[a.ConversationProfile]; !ok {
			return fmt.Errorf("请选择存在的对话模型配置")
		}
	}
	if a.VisionProfile != "" {
		p, ok := seen[a.VisionProfile]
		if !ok || !p.Vision {
			return fmt.Errorf("视觉模型必须选择勾选了支持视觉的配置")
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
	for _, path := range []string{"ai.profiles", "ai.conversation_profile", "ai.vision_profile"} {
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
				p.APIKey = value
			case "ai.base_url", "ai.vl.base_url":
				p.BaseURL = value
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
