// ai-eval compares model and routing variants using fictional tasks only.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func main() {
	path := flag.String("config", "config.toml", "model config; never printed")
	suite := flag.String("suite", "ai/testdata/conversations.json", "synthetic evaluation suite")
	models := flag.String("models", "", "comma separated model IDs on the configured provider; empty uses configured model")
	routing := flag.String("routing", "native", "native tools")
	limit := flag.Int("limit", 10, "max cases, 1-30")
	flag.Parse()
	if *limit < 1 || *limit > 30 || *routing != "native" {
		fmt.Fprintln(os.Stderr, "invalid evaluation options")
		os.Exit(2)
	}
	settings, err := config.LoadAI(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "model configuration unavailable")
		os.Exit(1)
	}
	data, err := os.ReadFile(*suite)
	if err != nil {
		fmt.Fprintln(os.Stderr, "suite unavailable")
		os.Exit(1)
	}
	var cases []ai.EvaluationCase
	if json.Unmarshal(data, &cases) != nil {
		fmt.Fprintln(os.Stderr, "invalid suite")
		os.Exit(1)
	}
	profile, connection, err := settings.Select("conversation")
	if err != nil {
		fmt.Fprintln(os.Stderr, "conversation not configured")
		os.Exit(1)
	}
	if connection.Kind != "openai" {
		fmt.Fprintln(os.Stderr, "this metered evaluation requires an OpenAI-compatible account; use ai-check for Codex")
		os.Exit(2)
	}
	names := []string{profile.Model}
	if *models != "" {
		names = strings.Split(*models, ",")
	}
	if len(names) > 3 {
		fmt.Fprintln(os.Stderr, "maximum three models")
		os.Exit(2)
	}
	modes := []bool{false}
	for _, name := range names {
		cfg := config.Current()
		cfg.AI = settings
		for i := range cfg.AI.Profiles {
			if cfg.AI.Profiles[i].ID == profile.ID {
				cfg.AI.Profiles[i].Model = strings.TrimSpace(name)
			}
		}
		config.Publish(cfg)
		c := openai.DefaultConfig(connection.APIKey)
		if connection.BaseURL != "" {
			c.BaseURL = connection.BaseURL
		}
		backend := openai.NewClientWithConfig(c)
		for _, mode := range modes {
			for _, test := range cases[:min(*limit, len(cases))] {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				result := ai.EvaluateConversation(ctx, test, backend, mode)
				cancel()
				json.NewEncoder(os.Stdout).Encode(result)
			}
		}
	}
}
