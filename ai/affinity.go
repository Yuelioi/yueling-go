package ai

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/services/logx"
)

type AffinityEvent struct {
	Message string
}

const (
	MaxAffinityTiers       = 8
	MaxAffinityTierName    = 24
	MaxAffinityPromptChars = 2000
)

func ClassifyAffinityDelta(event AffinityEvent) (int, string) {
	message := strings.ToLower(strings.TrimSpace(event.Message))
	if message == "" {
		return 0, ""
	}

	negativeKeywords := []string{
		"涩涩", "黄色", "色情", "色图", "裸照", "开车", "约炮", "操你", "傻逼",
		"去死", "越狱", "jailbreak", "忽略之前", "忽略以上", "system prompt",
	}
	for _, keyword := range negativeKeywords {
		if strings.Contains(message, keyword) {
			return -10, "包含冒犯、骚扰或越界内容"
		}
	}

	positiveKeywords := []string{"请", "谢谢", "麻烦", "帮我", "总结", "部署", "辛苦"}
	for _, keyword := range positiveKeywords {
		if strings.Contains(message, keyword) {
			return 2, "礼貌正常交流"
		}
	}

	if utf8.RuneCountInString(message) >= 8 {
		return 1, "正常交流"
	}
	return 0, ""
}

func NormalizeAffinityConfig(cfg config.AffinityConfig) config.AffinityConfig {
	usesDefaultRange := cfg.Min == 0 && cfg.Max == 0
	if usesDefaultRange {
		cfg.Min = 0
		cfg.Max = 100
	}
	if cfg.Initial == 0 && usesDefaultRange {
		cfg.Initial = 50
	}
	if cfg.BlockBelow == 0 && usesDefaultRange {
		cfg.BlockBelow = 10
	}
	if cfg.Max < cfg.Min {
		cfg.Max = cfg.Min
	}
	if cfg.Initial < cfg.Min {
		cfg.Initial = cfg.Min
	}
	if cfg.Initial > cfg.Max {
		cfg.Initial = cfg.Max
	}
	if cfg.BlockBelow < cfg.Min {
		cfg.BlockBelow = cfg.Min
	}
	if cfg.BlockBelow > cfg.Max {
		cfg.BlockBelow = cfg.Max
	}
	return cfg
}

func ApplyAffinityDelta(current, delta int, cfg config.AffinityConfig) int {
	cfg = NormalizeAffinityConfig(cfg)
	next := current + delta
	if next < cfg.Min {
		return cfg.Min
	}
	if next > cfg.Max {
		return cfg.Max
	}
	return next
}

func builtinAffinityTier(score int, cfg config.AffinityConfig) (string, string) {
	cfg = NormalizeAffinityConfig(cfg)
	switch {
	case score < cfg.BlockBelow:
		return "疏远", "回复要保持克制、简短，避免主动亲近。"
	case score < cfg.Initial:
		return "普通", "回复保持礼貌自然。"
	case score >= cfg.Max:
		return "亲近", "回复可以更熟悉，但仍需遵守边界。"
	default:
		return "友好", "回复自然、温和。"
	}
}

func formatAffinityTierPrompt(name, prompt string) string {
	name = strings.TrimSpace(name)
	prompt = strings.TrimSpace(prompt)
	return fmt.Sprintf("当前关系状态：%s。%s", name, prompt)
}

func AffinityPrompt(score int, cfg config.AffinityConfig) string {
	name, prompt := builtinAffinityTier(score, cfg)
	return formatAffinityTierPrompt(name, prompt)
}

// DefaultAffinityTiers exposes the built-in behavior as editable thresholds.
func DefaultAffinityTiers(cfg config.AffinityConfig) []db.AIAffinityTier {
	cfg = NormalizeAffinityConfig(cfg)
	candidates := []int{cfg.Min, cfg.BlockBelow, cfg.Initial, cfg.Max}
	sort.Ints(candidates)
	tiers := make([]db.AIAffinityTier, 0, len(candidates))
	lastName, lastPrompt := "", ""
	for _, threshold := range candidates {
		if threshold < cfg.Min || threshold > cfg.Max {
			continue
		}
		name, prompt := builtinAffinityTier(threshold, cfg)
		if name == lastName && prompt == lastPrompt {
			continue
		}
		if len(tiers) > 0 && tiers[len(tiers)-1].MinScore == threshold {
			tiers[len(tiers)-1].Name = name
			tiers[len(tiers)-1].Prompt = prompt
		} else {
			tiers = append(tiers, db.AIAffinityTier{MinScore: threshold, Name: name, Prompt: prompt})
		}
		lastName, lastPrompt = name, prompt
	}
	return tiers
}

// ValidateAffinityTiers normalizes administrator input and guarantees complete,
// non-overlapping coverage of the configured score range.
func ValidateAffinityTiers(cfg config.AffinityConfig, source []db.AIAffinityTier) ([]db.AIAffinityTier, error) {
	cfg = NormalizeAffinityConfig(cfg)
	if len(source) < 2 {
		return nil, fmt.Errorf("至少需要两个好感度阶梯")
	}
	if len(source) > MaxAffinityTiers {
		return nil, fmt.Errorf("好感度阶梯不能超过 %d 个", MaxAffinityTiers)
	}
	tiers := make([]db.AIAffinityTier, len(source))
	copy(tiers, source)
	for i := range tiers {
		tiers[i].Name = strings.TrimSpace(tiers[i].Name)
		tiers[i].Prompt = strings.TrimSpace(tiers[i].Prompt)
		if tiers[i].MinScore < cfg.Min || tiers[i].MinScore > cfg.Max {
			return nil, fmt.Errorf("阶梯起始分数必须在 %d 到 %d 之间", cfg.Min, cfg.Max)
		}
		if tiers[i].Name == "" {
			return nil, fmt.Errorf("阶梯名称不能为空")
		}
		if utf8.RuneCountInString(tiers[i].Name) > MaxAffinityTierName {
			return nil, fmt.Errorf("阶梯名称不能超过 %d 个字符", MaxAffinityTierName)
		}
		if tiers[i].Prompt == "" {
			return nil, fmt.Errorf("阶梯提示词不能为空")
		}
		if utf8.RuneCountInString(tiers[i].Prompt) > MaxAffinityPromptChars {
			return nil, fmt.Errorf("阶梯提示词不能超过 %d 个字符", MaxAffinityPromptChars)
		}
	}
	sort.Slice(tiers, func(i, j int) bool { return tiers[i].MinScore < tiers[j].MinScore })
	if tiers[0].MinScore != cfg.Min {
		return nil, fmt.Errorf("第一个阶梯必须从最低分 %d 开始", cfg.Min)
	}
	for i := 1; i < len(tiers); i++ {
		if tiers[i].MinScore == tiers[i-1].MinScore {
			return nil, fmt.Errorf("阶梯起始分数不能重复")
		}
	}
	return tiers, nil
}

func affinityPromptFromTiers(score int, tiers []db.AIAffinityTier) string {
	selected := tiers[0]
	for _, tier := range tiers[1:] {
		if score < tier.MinScore {
			break
		}
		selected = tier
	}
	return formatAffinityTierPrompt(selected.Name, selected.Prompt)
}

func ChatAffinityPrompt(score int, cfg config.AffinityConfig) string {
	if !cfg.Enabled {
		return ""
	}
	if db.DB != nil {
		tiers, err := db.ListAIAffinityTiers()
		if err != nil {
			logx.Warnf("[ai] affinity tiers load failed: %v", err)
		} else if len(tiers) > 0 {
			return affinityPromptFromTiers(score, tiers)
		}
	}
	return AffinityPrompt(score, cfg)
}

func UpdateChatAffinity(userID, groupID int64, nickname, text string) (int, bool) {
	cfg := NormalizeAffinityConfig(config.C.AI.Affinity)
	if !cfg.Enabled {
		return cfg.Initial, true
	}

	delta, reason := ClassifyAffinityDelta(AffinityEvent{Message: text})
	if db.DB == nil {
		logx.Warnf("[ai] affinity update skipped user=%d group=%d: database is not initialized", userID, groupID)
		return cfg.Initial, true
	}

	row, err := db.UpdateAIAffinity(userID, groupID, nickname, cfg.Initial, delta, cfg.Min, cfg.Max, reason)
	if err != nil {
		logx.Warnf("[ai] affinity update failed user=%d group=%d: %v", userID, groupID, err)
		return cfg.Initial, true
	}
	return row.Score, row.Score >= cfg.BlockBelow
}
