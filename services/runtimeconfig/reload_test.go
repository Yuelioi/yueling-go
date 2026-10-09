package runtimeconfig

import (
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func TestReloadPolicyMatchesEditableFields(t *testing.T) {
	for _, tt := range []struct {
		path   string
		reload bool
		change func(*config.Config)
	}{
		{"napcat.token", true, func(c *config.Config) { c.NapCat.Token = "changed" }},
		{"http_api.key", true, func(c *config.Config) { c.HTTPAPI.Key = "changed" }},
		{"webui.password", true, func(c *config.Config) { c.WebUI.Password = "changed" }},
		{"tools.meme_server", true, func(c *config.Config) { c.Tools.MemeServer = "changed" }},
		{"bot.timezone", true, func(c *config.Config) { c.Bot.Timezone = "UTC" }},
		{"image.entry", true, func(c *config.Config) { c.Image.Entry = []config.ImageEntry{{Call: []string{"image"}}} }},
		{"bilibili.cookie", true, func(c *config.Config) { c.Bilibili.Cookie = "changed" }},
		{"bot.name", false, func(c *config.Config) { c.Bot.Name = "changed" }},
		{"ai.max_tokens", false, func(c *config.Config) { c.AI.MaxTokens = 8192 }},
		{"plugins.disabled", false, func(c *config.Config) { c.Plugins.Disabled = []int{1} }},
		{"database.max_open", false, func(c *config.Config) { c.Database.MaxOpen = 8 }},
	} {
		t.Run(tt.path, func(t *testing.T) {
			var before config.Config
			next := before.Clone()
			tt.change(&next)
			if NeedsReload(before, next) != tt.reload {
				t.Fatal("incorrect apply policy")
			}
			found := false
			for _, section := range fields(next) {
				for _, field := range section.Fields {
					if field.Path == tt.path {
						found = true
						if field.Reload != tt.reload {
							t.Fatal("incorrect field policy")
						}
					}
				}
			}
			if !found {
				t.Fatal("missing field")
			}
			if NeedsReload(next, next.Clone()) {
				t.Fatal("unchanged settings require reload")
			}
		})
	}
}
