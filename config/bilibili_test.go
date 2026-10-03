package config

import (
	"strings"
	"testing"
)

func TestBilibiliDefaultsAndValidation(t *testing.T) {
	if err := Load(writeConfig(t, baseConfig(""))); err != nil {
		t.Fatal(err)
	}
	if C.Bilibili.Enabled || C.Bilibili.ReplyMode != "off" || C.Bilibili.PollSeconds != 60 {
		t.Fatalf("defaults=%+v", C.Bilibili)
	}
	if err := Load(writeConfig(t, baseConfig("[bilibili]\nenabled=true\nuid=123\nnotify_users=[99]\n[[bilibili.keywords]]\nkeyword='你好'\nreply='hi'\nexact=true\n"))); err != nil {
		t.Fatal(err)
	}
	if C.Bilibili.UID != 123 || len(C.Bilibili.Keywords) != 1 || !C.Bilibili.Keywords[0].Exact {
		t.Fatalf("decoded config=%+v", C.Bilibili)
	}
	for _, fields := range []string{"poll_seconds=5", "recent_count=100", "reply_mode='unknown'", "reply_per_minute=0", "notify_users=[0]"} {
		if err := Load(writeConfig(t, baseConfig("[bilibili]\nenabled=true\n"+fields+"\n"))); err == nil || !strings.Contains(err.Error(), "bilibili") {
			t.Fatalf("invalid fields %q err=%v", fields, err)
		}
	}
}

func TestBilibiliCookieEnvironmentOverride(t *testing.T) {
	t.Setenv("YUELING_BILIBILI_COOKIE", "DedeUserID=10; SESSDATA=test")
	if err := Load(writeConfig(t, baseConfig("[bilibili]\nenabled=true\n"))); err != nil {
		t.Fatal(err)
	}
	if C.Bilibili.Cookie != "DedeUserID=10; SESSDATA=test" {
		t.Fatal("cookie environment override was not loaded")
	}
}
