package config

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestPublishedConfigurationOwnsNestedValues(t *testing.T) {
	old := Current()
	t.Cleanup(func() { Publish(old) })
	arg := true
	original := Config{Bot: BotConfig{SuperUsers: []int64{1}, Blacklist: []int64{4}}, Plugins: PluginsConfig{Disabled: []int{2}}, Image: ImageConfig{Entry: []ImageEntry{{Call: []string{"image"}, Arg: &arg}}}, Bilibili: BilibiliConfig{NotifyUsers: []int64{3}, Keywords: []BilibiliKeyword{{Keyword: "hello"}}}}
	Publish(original)
	baseline, _ := json.Marshal(Current())
	mutate := func(c Config) {
		c.Bot.SuperUsers[0] = 9
		c.Bot.Blacklist[0] = 9
		c.Plugins.Disabled[0] = 9
		c.Image.Entry[0].Call[0] = "changed"
		*c.Image.Entry[0].Arg = false
		c.Bilibili.NotifyUsers[0] = 9
		c.Bilibili.Keywords[0].Keyword = "changed"
	}
	mutate(original)
	if got := Current(); got.Bot.SuperUsers[0] != 1 || got.Image.Entry[0].Call[0] != "image" || !*got.Image.Entry[0].Arg {
		t.Fatal("publish retained caller-owned values")
	}
	mutate(Current())
	if got, _ := json.Marshal(Current()); !bytes.Equal(got, baseline) {
		t.Fatal("reader changed published configuration")
	}
}
