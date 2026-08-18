package feed

import "testing"

func TestBuildPlatformURL(t *testing.T) {
	tests := []struct {
		kind   PlatformKind
		target string
		want   string
	}{
		{PlatformBilibiliDynamic, "https://space.bilibili.com/2267573", "https://rss.example/bilibili/user/dynamic/2267573/embed=0"},
		{PlatformBilibiliDynamic, "https://www.bilibili.com/space/2267573", "https://rss.example/bilibili/user/dynamic/2267573/embed=0"},
		{PlatformBilibiliVideo, "2267573", "https://rss.example/bilibili/user/video/2267573/1"},
		{PlatformBilibiliLive, "https://live.bilibili.com/3", "https://rss.example/bilibili/live/room/3"},
		{PlatformXUser, "@DIYgod", "https://rss.example/twitter/user/DIYgod/excludeReplies=1&includeRts=0&forceWebApi=1"},
	}
	for _, test := range tests {
		got, err := BuildPlatformURL("https://rss.example/", test.kind, test.target)
		if err != nil || got != test.want {
			t.Fatalf("BuildPlatformURL(%s, %q) = %q, %v", test.kind, test.target, got, err)
		}
	}
}

func TestBuildPlatformURLRejectsBadTargets(t *testing.T) {
	for _, test := range []struct {
		kind   PlatformKind
		target string
	}{
		{PlatformBilibiliVideo, "https://evil.example/123"},
		{PlatformBilibiliLive, "room"},
		{PlatformXUser, "bad handle!"},
	} {
		if _, err := BuildPlatformURL("https://rss.example", test.kind, test.target); err == nil {
			t.Fatalf("bad target accepted: %s %q", test.kind, test.target)
		}
	}
}
