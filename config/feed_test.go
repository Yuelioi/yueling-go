package config

import (
	"strings"
	"testing"
)

func TestLoadFeedRSSHubBaseDefaultsAndValidates(t *testing.T) {
	path := writeConfig(t, baseConfig(""))
	if err := Load(path); err != nil {
		t.Fatal(err)
	}
	if C.Feed.RSSHubBase != "https://rsshub.app" {
		t.Fatalf("default RSSHub base = %q", C.Feed.RSSHubBase)
	}

	badPath := writeConfig(t, baseConfig(`
[feed]
rsshub_base = "file:///tmp/rsshub"
`))
	if err := Load(badPath); err == nil || !strings.Contains(err.Error(), "feed.rsshub_base") {
		t.Fatalf("invalid RSSHub base error = %v", err)
	}
}
