package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestImageEntryUnmarshal(t *testing.T) {
	viper.Reset()
	viper.SetConfigType("toml")
	toml := `
[[image.entry]]
folder = "龙图"
call   = ["龙图", "龙图攻击"]
add    = "添加龙图"

[[image.entry]]
call = ["随机猫猫"]
kind = "external"
url  = "http://edgecats.net/"
pick = "data.url"
`
	if err := viper.ReadConfig(strings.NewReader(toml)); err != nil {
		t.Fatal(err)
	}
	var c Config
	if err := viper.Unmarshal(&c); err != nil {
		t.Fatal(err)
	}
	if len(c.Image.Entry) != 2 {
		t.Fatalf("want 2 entries, got %d", len(c.Image.Entry))
	}
	e0 := c.Image.Entry[0]
	if e0.Folder != "龙图" || len(e0.Call) != 2 || e0.Add != "添加龙图" {
		t.Fatalf("entry0 mismatch: %+v", e0)
	}
	e1 := c.Image.Entry[1]
	if e1.Kind != KindExternal || e1.URL != "http://edgecats.net/" || e1.Pick != "data.url" {
		t.Fatalf("entry1 mismatch: %+v", e1)
	}
}
