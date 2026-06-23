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
folder = "吃的"
call   = ["随机吃的"]
add    = "添加吃的"
kind   = "grid"
arg    = true

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
	if len(c.Image.Entry) != 3 {
		t.Fatalf("want 3 entries, got %d", len(c.Image.Entry))
	}
	e0 := c.Image.Entry[0]
	if e0.Folder != "龙图" || len(e0.Call) != 2 || e0.Add != "添加龙图" || e0.Arg != nil {
		t.Fatalf("entry0 mismatch: %+v", e0)
	}
	e1 := c.Image.Entry[1]
	if e1.Kind != KindGrid || e1.Arg == nil || !*e1.Arg {
		t.Fatalf("entry1 (grid arg) mismatch: %+v (arg=%v)", e1, e1.Arg)
	}
	e2 := c.Image.Entry[2]
	if e2.Kind != KindExternal || e2.URL != "http://edgecats.net/" || e2.Pick != "data.url" {
		t.Fatalf("entry2 mismatch: %+v", e2)
	}
}
