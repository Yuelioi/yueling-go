package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestIndependentConfigurationReaders(t *testing.T) {
	path := writeConfig(t, baseConfig(""))
	for i := range 20 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			name := fmt.Sprintf("reader-%d", i)
			lookup := func(key string) (string, bool) { return name, key == "YUELING_BOT_NAME" }
			c, err := readConfig(path, false, lookup)
			if err != nil || c.Bot.Name != name {
				t.Fatalf("independent reader: name=%q err=%v", c.Bot.Name, err)
			}
		})
	}
}

func TestConfigurationEnvironmentTypes(t *testing.T) {
	values := map[string]string{
		"YUELING_BOT_SUPERUSERS":                 "[123,456]",
		"YUELING_AI_PROFILES":                    `[{"id":"test","model":"fixture","api_key":"secret"}]`,
		"YUELING_AI_BINDINGS":                    `{"conversation":"test"}`,
		"YUELING_BOT_COMMAND_ARG_SPACE_REQUIRED": "false",
		"YUELING_DATABASE_MAX_OPEN":              "30",
	}
	c, err := readConfig(writeConfig(t, baseConfig("")), false, func(key string) (string, bool) { v, ok := values[key]; return v, ok })
	if err != nil {
		t.Fatal(err)
	}
	if c.Bot.CommandArgSpaceRequired || c.Database.MaxOpen != 30 || len(c.Bot.SuperUsers) != 2 || c.AI.Profiles[0].Model != "fixture" {
		t.Fatal("environment values not applied")
	}
}

func TestInvalidConfigurationDoesNotPublishOrExposeValues(t *testing.T) {
	original := Current()
	t.Cleanup(func() { Publish(original) })
	Publish(Config{Bot: BotConfig{Name: "keep"}})
	for _, content := range []string{
		"[ai]\nmax_tokens='credential-do-not-log'",
		"[ai]\nunknown_key='credential-do-not-log'",
		"[ai]\ndeepseek_key='credential-do-not-log",
	} {
		err := Load(writeConfig(t, content))
		if err == nil || strings.Contains(err.Error(), "credential-do-not-log") {
			t.Fatalf("unsafe error: %v", err)
		}
		if Current().Bot.Name != "keep" {
			t.Fatal("failed load published configuration")
		}
	}
	t.Setenv("YUELING_DATABASE_MAX_OPEN", "credential-do-not-log")
	err := Load(writeConfig(t, baseConfig("")))
	if err == nil || strings.Contains(err.Error(), "credential-do-not-log") || !strings.Contains(err.Error(), "YUELING_DATABASE_MAX_OPEN") {
		t.Fatalf("unsafe environment error: %v", err)
	}
}

func TestAIReaderResolvesProfiles(t *testing.T) {
	path := writeConfig(t, "[ai.bindings]\nconversation='chat'\n[[ai.connections]]\nid='account'\napi_key='fixture'\n[[ai.profiles]]\nid='chat'\nconnection_id='account'\ncapabilities=['chat','tools']\nmodel='fixture-model'\n")
	c, err := LoadAI(path)
	if err != nil || c.Profiles[0].Model != "fixture-model" || c.Connections[0].APIKey != "fixture" {
		t.Fatalf("profile resolution err=%v", err)
	}
}

func TestExampleConfigurationHasKnownFields(t *testing.T) {
	_, err := readConfig("../config.example.toml", false, func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}

}
