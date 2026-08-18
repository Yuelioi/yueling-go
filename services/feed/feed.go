package feed

import (
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/httpclient"
)

const (
	maxFeedBytes     = 2 * 1024 * 1024
	maxFeedItems     = 50
	maxItemLinkBytes = 512
)

type Item struct {
	Key       string
	Title     string
	Link      string
	Published time.Time
}

type Feed struct {
	Title string
	Items []Item
}

type rssDocument struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	GUID    string `xml:"guid"`
	PubDate string `xml:"pubDate"`
	Date    string `xml:"date"`
}

type atomDocument struct {
	Title   string      `xml:"title"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	ID        string     `xml:"id"`
	Title     string     `xml:"title"`
	Links     []atomLink `xml:"link"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

type rdfDocument struct {
	Channel rssChannel `xml:"channel"`
	Items   []rssItem  `xml:"item"`
}

// Fetch downloads and parses an untrusted public RSS/Atom URL.
func Fetch(rawURL string) (*Feed, error) {
	headers := []string{"Accept", "application/atom+xml, application/rss+xml, application/xml, text/xml;q=0.9, */*;q=0.1"}
	var body []byte
	var err error
	if isConfiguredRSSHubURL(rawURL, config.C.Feed.RSSHubBase) {
		body, err = httpclient.GetTrustedBaseBytesLimit(rawURL, config.C.Feed.RSSHubBase, maxFeedBytes, headers...)
	} else {
		body, err = httpclient.GetPublicBytesLimit(rawURL, maxFeedBytes, headers...)
	}
	if err != nil {
		if isConfiguredRSSHubURL(rawURL, config.C.Feed.RSSHubBase) {
			return nil, rssHubFetchError(err)
		}
		return nil, err
	}
	return Parse(body, rawURL)
}

func rssHubFetchError(err error) error {
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return fmt.Errorf("RSSHub 连接超时；请配置 tools.proxy，或将 feed.rsshub_base 改为自建实例")
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "HTTP 403"):
		return fmt.Errorf("RSSHub 拒绝访问（HTTP 403）；公共实例可能触发了反爬，请改用自建实例")
	case strings.Contains(message, "HTTP 429"):
		return fmt.Errorf("RSSHub 请求过于频繁（HTTP 429）；请稍后重试或改用自建实例")
	case strings.Contains(message, "重定向到了其他地址"):
		return fmt.Errorf("RSSHub 返回了异常重定向；请检查实例状态或改用自建实例")
	default:
		return fmt.Errorf("RSSHub 请求失败: %w", err)
	}
}

func isConfiguredRSSHubURL(rawURL, baseURL string) bool {
	target, targetErr := url.Parse(rawURL)
	base, baseErr := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	return targetErr == nil && baseErr == nil && target.User == nil && base.User == nil &&
		strings.EqualFold(target.Scheme, base.Scheme) && strings.EqualFold(target.Host, base.Host)
}

// Parse supports RSS 2.0, Atom, and RSS 1.0/RDF documents.
func Parse(body []byte, sourceURL string) (*Feed, error) {
	var root struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("解析订阅源失败: %w", err)
	}

	var title string
	var items []Item
	switch strings.ToLower(root.XMLName.Local) {
	case "rss":
		var document rssDocument
		if err := xml.Unmarshal(body, &document); err != nil {
			return nil, fmt.Errorf("解析 RSS 失败: %w", err)
		}
		title = document.Channel.Title
		items = convertRSSItems(document.Channel.Items)
	case "feed":
		var document atomDocument
		if err := xml.Unmarshal(body, &document); err != nil {
			return nil, fmt.Errorf("解析 Atom 失败: %w", err)
		}
		title = document.Title
		items = convertAtomItems(document.Entries)
	case "rdf":
		var document rdfDocument
		if err := xml.Unmarshal(body, &document); err != nil {
			return nil, fmt.Errorf("解析 RSS 1.0 失败: %w", err)
		}
		title = document.Channel.Title
		items = convertRSSItems(document.Items)
	default:
		return nil, fmt.Errorf("不支持的订阅格式: %s", root.XMLName.Local)
	}

	title = cleanFeedText(title, 64)
	if title == "" {
		if parsed, err := url.Parse(sourceURL); err == nil {
			title = parsed.Hostname()
		}
	}
	if title == "" {
		title = "未命名订阅"
	}
	items = normalizeItems(items)
	return &Feed{Title: title, Items: items}, nil
}

func convertRSSItems(source []rssItem) []Item {
	items := make([]Item, 0, len(source))
	for _, sourceItem := range source {
		published := parseFeedTime(firstNonEmpty(sourceItem.PubDate, sourceItem.Date))
		items = append(items, Item{
			Key:       itemKey(sourceItem.GUID, sourceItem.Link, sourceItem.Title, published),
			Title:     sourceItem.Title,
			Link:      sourceItem.Link,
			Published: published,
		})
	}
	return items
}

func convertAtomItems(source []atomEntry) []Item {
	items := make([]Item, 0, len(source))
	for _, sourceItem := range source {
		link := ""
		for _, candidate := range sourceItem.Links {
			if candidate.Rel == "" || candidate.Rel == "alternate" {
				link = candidate.Href
				break
			}
		}
		published := parseFeedTime(firstNonEmpty(sourceItem.Published, sourceItem.Updated))
		items = append(items, Item{
			Key:       itemKey(sourceItem.ID, link, sourceItem.Title, published),
			Title:     sourceItem.Title,
			Link:      link,
			Published: published,
		})
	}
	return items
}

func normalizeItems(items []Item) []Item {
	seen := map[string]bool{}
	normalized := make([]Item, 0, min(len(items), maxFeedItems))
	for _, item := range items {
		item.Title = cleanFeedText(item.Title, 160)
		if item.Title == "" {
			item.Title = "（无标题）"
		}
		item.Link = cleanFeedLink(item.Link)
		if item.Key == "" || seen[item.Key] {
			continue
		}
		seen[item.Key] = true
		normalized = append(normalized, item)
		if len(normalized) >= maxFeedItems {
			break
		}
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		left, right := normalized[i].Published, normalized[j].Published
		if left.IsZero() && right.IsZero() {
			return false
		}
		if left.IsZero() {
			return false
		}
		if right.IsZero() {
			return true
		}
		return left.After(right)
	})
	return normalized
}

func cleanFeedText(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) > maxRunes {
		if maxRunes == 1 {
			return "…"
		}
		return string([]rune(value)[:maxRunes-1]) + "…"
	}
	return value
}

func cleanFeedLink(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > maxItemLinkBytes {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return ""
	}
	parsed.Fragment = ""
	return parsed.String()
}

func itemKey(id, link, title string, published time.Time) string {
	seed := firstNonEmpty(strings.TrimSpace(id), strings.TrimSpace(link))
	if seed == "" {
		seed = strings.TrimSpace(title) + "|" + published.UTC().Format(time.RFC3339Nano)
	}
	if seed == "|0001-01-01T00:00:00Z" || seed == "" {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(seed)))
}

func parseFeedTime(value string) time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC850,
		time.ANSIC,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"2006-01-02 15:04:05 -0700",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
