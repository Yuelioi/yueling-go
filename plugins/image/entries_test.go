package image

import (
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func TestValidateEntries(t *testing.T) {
	ok := []config.ImageEntry{
		{Folder: "龙图", Call: []string{"龙图"}, Add: "添加龙图"},
		{Folder: "吃的", Call: []string{"随机吃的"}, Add: "添加吃的", Kind: config.KindGrid},
		{Call: []string{"猫猫"}, Kind: config.KindExternal, URL: "http://x/"},
	}
	if err := validateEntries(ok); err != nil {
		t.Fatalf("valid set rejected: %v", err)
	}

	bad := []struct {
		name    string
		entries []config.ImageEntry
	}{
		{"bad kind", []config.ImageEntry{{Folder: "a", Call: []string{"a"}, Kind: "weird"}}},
		{"single no folder", []config.ImageEntry{{Call: []string{"a"}}}},
		{"single no call", []config.ImageEntry{{Folder: "a"}}},
		{"grid no add", []config.ImageEntry{{Folder: "a", Call: []string{"a"}, Kind: config.KindGrid}}},
		{"external no url", []config.ImageEntry{{Call: []string{"a"}, Kind: config.KindExternal}}},
		{"grid arg false", []config.ImageEntry{{Folder: "a", Call: []string{"a"}, Add: "添加a", Kind: config.KindGrid, Arg: boolPtr(false)}}},
		{"dup command", []config.ImageEntry{
			{Folder: "a", Call: []string{"x"}, Add: "添加a"},
			{Folder: "b", Call: []string{"x"}, Add: "添加b"},
		}},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			if err := validateEntries(c.entries); err == nil {
				t.Fatalf("expected error for %s", c.name)
			}
		})
	}
}

func TestNameFns(t *testing.T) {
	if got := nameByHash("HH", "ignored", 123); got != "HH" {
		t.Fatalf("nameByHash = %q", got)
	}
	if got := nameByArg("HH", "麻辣烫", 123); got != "麻辣烫_HH" {
		t.Fatalf("nameByArg = %q, want 麻辣烫_HH", got)
	}
	if got := nameByArg("HH", "", 123); got != "HH" {
		t.Fatalf("nameByArg empty arg = %q, want HH", got)
	}
}

func TestDefaultEntriesValid(t *testing.T) {
	if err := validateEntries(defaultEntries); err != nil {
		t.Fatalf("defaultEntries invalid: %v", err)
	}
}

func TestArgRequired(t *testing.T) {
	cases := []struct {
		name string
		e    config.ImageEntry
		want bool
	}{
		{"single default false", config.ImageEntry{Kind: config.KindSingle}, false},
		{"grid default true", config.ImageEntry{Kind: config.KindGrid}, true},
		{"empty kind default false", config.ImageEntry{}, false},
		{"single explicit true", config.ImageEntry{Kind: config.KindSingle, Arg: boolPtr(true)}, true},
		{"grid explicit true", config.ImageEntry{Kind: config.KindGrid, Arg: boolPtr(true)}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := argRequired(c.e); got != c.want {
				t.Fatalf("argRequired = %v, want %v", got, c.want)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }
