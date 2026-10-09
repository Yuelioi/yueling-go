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
	routing := flag.String("routing", "both", "keyword, discovery, or both")
	limit := flag.Int("limit", 10, "max cases, 1-30")
	flag.Parse()
	if *limit < 1 || *limit > 30 || (*routing != "both" && *routing != "keyword" && *routing != "discovery") {
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
	names := []string{settings.Model}
	if *models != "" {
		names = strings.Split(*models, ",")
	}
	if len(names) > 3 {
		fmt.Fprintln(os.Stderr, "maximum three models")
		os.Exit(2)
	}
	modes := []bool{false, true}
	if *routing == "keyword" {
		modes = []bool{false}
	}
	if *routing == "discovery" {
		modes = []bool{true}
	}
	for _, name := range names {
		cfg := config.Current()
		cfg.AI = settings
		cfg.AI.Profiles = nil
		cfg.AI.Model = strings.TrimSpace(name)
		config.Publish(cfg)
		c := openai.DefaultConfig(settings.DeepSeekKey)
		if settings.BaseURL != "" {
			c.BaseURL = settings.BaseURL
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
