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
		{PlatformXiaohongshuUser, "593032945e87e77791e03696", "https://rss.example/xiaohongshu/user/593032945e87e77791e03696/notes"},
		{PlatformXiaohongshuUser, "https://www.xiaohongshu.com/user/profile/593032945e87e77791e03696?xsec_token=example", "https://rss.example/xiaohongshu/user/593032945e87e77791e03696/notes"},
		{PlatformGitHubReleases, "DIYgod/RSSHub", "https://github.com/DIYgod/RSSHub/releases.atom"},
		{PlatformGitHubReleases, "https://github.com/DIYgod/RSSHub/releases", "https://github.com/DIYgod/RSSHub/releases.atom"},
		{PlatformGitHubIssues, "https://github.com/DIYgod/RSSHub.git", "https://rss.example/github/issue/DIYgod/RSSHub/all"},
		{PlatformGitHubIssues, "DIYgod/RSSHub", "https://rss.example/github/issue/DIYgod/RSSHub/all"},
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
		{PlatformXiaohongshuUser, "https://evil.example/user/profile/593032945e87e77791e03696"},
		{PlatformXiaohongshuUser, "https://www.xiaohongshu.com/explore/593032945e87e77791e03696"},
		{PlatformXiaohongshuUser, "123"},
		{PlatformGitHubIssues, "https://github.com.evil.example/owner/repo"},
		{PlatformGitHubIssues, "owner/.."},
		{PlatformGitHubIssues, "owner/repo?token=secret"},
		{PlatformGitHubReleases, "owner"},
		{PlatformGitHubReleases, "owner/repo/extra"},
	} {
		if _, err := BuildPlatformURL("https://rss.example", test.kind, test.target); err == nil {
			t.Fatalf("bad target accepted: %s %q", test.kind, test.target)
		}
	}
}
