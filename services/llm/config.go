package llm

import (
	"path/filepath"
	"time"

	"github.com/Yuelioi/yueling-go/config"
)

func FromConfig(c config.AIConfig) Settings { return ForFeature(c, "conversation") }

func ForFeature(c config.AIConfig, feature string) Settings {
	p, conn, err := c.Select(feature)
	s := Settings{Model: p.Model, APIKey: conn.APIKey, BaseURL: conn.BaseURL, Kind: conn.Kind, ProfileID: p.ID, Feature: feature, MaxTokens: c.MaxTokens, ReasoningEffort: c.ReasoningEffort, configError: err}
	if p.MaxTokens > 0 {
		s.MaxTokens = p.MaxTokens
	}
	if p.TimeoutSeconds > 0 {
		s.Timeout = time.Duration(p.TimeoutSeconds) * time.Second
	}
	if p.ReasoningEffort != "" {
		s.ReasoningEffort = p.ReasoningEffort
	}
	if conn.Kind == "codex" {
		s.CodexHome = CodexHome(config.Current().Bot.DataDir, conn.ID)
		if s.Timeout == 0 {
			s.Timeout = 90 * time.Second
		}
	}
	return s
}

func CodexHome(dataDir, connectionID string) string {
	if dataDir == "" {
		dataDir = "data"
	}
	path, _ := filepath.Abs(filepath.Join(dataDir, "codex", connectionID))
	return path
}
