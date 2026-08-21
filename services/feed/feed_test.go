package feed

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestRSSHubFetchErrorExplainsTimeoutRecovery(t *testing.T) {
	err := rssHubFetchError(&url.Error{Op: "Get", URL: "https://rsshub.app/feed", Err: context.DeadlineExceeded}, "https://rsshub.app")
	if !strings.Contains(err.Error(), "tools.proxy") || !strings.Contains(err.Error(), "自建实例") {
		t.Fatalf("rssHubFetchError() = %q", err)
	}
}

func TestRSSHubFetchErrorExplainsPrivateInstanceTimeout(t *testing.T) {
	err := rssHubFetchError(&url.Error{Op: "Get", URL: "http://rsshub:1200/feed", Err: context.DeadlineExceeded}, "http://rsshub:1200")
	if !strings.Contains(err.Error(), "自建 RSSHub") || !strings.Contains(err.Error(), "Docker 网络") {
		t.Fatalf("rssHubFetchError() = %q", err)
	}
}

func TestParseRSSOrdersNewestFirstAndDeduplicates(t *testing.T) {
	body := []byte(`<?xml version="1.0"?>
<rss version="2.0"><channel><title> 示例 RSS </title>
  <item><guid>old</guid><title>旧消息</title><link>https://example.com/old</link><pubDate>Wed, 12 Aug 2026 08:00:00 +0000</pubDate></item>
  <item><guid>new</guid><title>新消息</title><link>https://example.com/new#part</link><pubDate>Thu, 13 Aug 2026 08:00:00 +0000</pubDate></item>
  <item><guid>new</guid><title>重复消息</title></item>
</channel></rss>`)
	parsed, err := Parse(body, "https://example.com/feed")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Title != "示例 RSS" || len(parsed.Items) != 2 {
		t.Fatalf("parsed = %+v", parsed)
	}
	if parsed.Items[0].Title != "新消息" || parsed.Items[0].Link != "https://example.com/new" {
		t.Fatalf("newest item = %+v", parsed.Items[0])
	}
}

func TestParseAtomUsesAlternateLinkAndUpdatedTime(t *testing.T) {
	body := []byte(`<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom"><title>Atom Feed</title>
  <entry><id>one</id><title>第一条</title><updated>2026-08-13T08:00:00Z</updated>
    <link rel="self" href="https://example.com/api/one"/><link rel="alternate" href="https://example.com/one"/>
  </entry>
</feed>`)
	parsed, err := Parse(body, "https://example.com/atom")
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Items) != 1 || parsed.Items[0].Link != "https://example.com/one" || parsed.Items[0].Published.IsZero() {
		t.Fatalf("parsed = %+v", parsed)
	}
}

func TestParseRejectsUnsupportedDocument(t *testing.T) {
	if _, err := Parse([]byte(`<html><title>not a feed</title></html>`), "https://example.com"); err == nil {
		t.Fatal("unsupported document was accepted")
	}
}

func TestFeedItemTextIsPreservedForDeliveryPolicy(t *testing.T) {
	longTitle := strings.Repeat("长", 200)
	body := []byte(`<rss><channel><item><guid>x</guid><title>` + longTitle + `</title><link>javascript:alert(1)</link></item></channel></rss>`)
	parsed, err := Parse(body, "https://example.com/feed")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Items[0].Title != longTitle || parsed.Items[0].Link != "" {
		t.Fatalf("normalized item = %+v", parsed.Items[0])
	}
}

func TestFeedItemTextHonorsSafetyBound(t *testing.T) {
	longTitle := strings.Repeat("长", MaxItemMaxChars+100)
	body := []byte(`<rss><channel><item><guid>x</guid><title>` + longTitle + `</title></item></channel></rss>`)
	parsed, err := Parse(body, "https://example.com/feed")
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.Items[0].Title; len([]rune(got)) != MaxItemMaxChars || !strings.HasSuffix(got, "…") {
		t.Fatalf("bounded title length=%d suffix=%q", len([]rune(got)), got[len(got)-3:])
	}
}

func TestFeedLinkLengthIsBounded(t *testing.T) {
	longLink := "https://example.com/" + strings.Repeat("a", maxItemLinkBytes)
	body := []byte(`<feed><entry><id>x</id><title>entry</title><link href="` + longLink + `"/></entry></feed>`)
	parsed, err := Parse(body, "https://example.com/feed")
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Items) != 1 || parsed.Items[0].Link != "" {
		t.Fatalf("oversized link was retained: %+v", parsed.Items)
	}
}

func TestCleanFeedTextKeepsRequestedMaximum(t *testing.T) {
	if got := cleanFeedText("一二三四", 3); got != "一二…" || len([]rune(got)) != 3 {
		t.Fatalf("cleanFeedText = %q", got)
	}
}
