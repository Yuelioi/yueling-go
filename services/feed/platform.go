package feed

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type PlatformKind string

const (
	PlatformBilibiliDynamic PlatformKind = "bilibili_dynamic"
	PlatformBilibiliVideo   PlatformKind = "bilibili_video"
	PlatformBilibiliLive    PlatformKind = "bilibili_live"
	PlatformXUser           PlatformKind = "x_user"
)

var (
	digitsPattern  = regexp.MustCompile(`^\d{1,20}$`)
	xHandlePattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,15}$`)
)

func BuildPlatformURL(baseURL string, kind PlatformKind, target string) (string, error) {
	base, err := normalizeRSSHubBase(baseURL)
	if err != nil {
		return "", err
	}
	identifier, err := platformIdentifier(kind, target)
	if err != nil {
		return "", err
	}

	var route string
	switch kind {
	case PlatformBilibiliDynamic:
		route = "/bilibili/user/dynamic/" + identifier + "/embed=0"
	case PlatformBilibiliVideo:
		route = "/bilibili/user/video/" + identifier + "/1"
	case PlatformBilibiliLive:
		route = "/bilibili/live/room/" + identifier
	case PlatformXUser:
		route = "/twitter/user/" + identifier + "/excludeReplies=1&includeRts=0&forceWebApi=1"
	default:
		return "", fmt.Errorf("不支持的平台订阅类型")
	}
	return base + route, nil
}

func PlatformLabel(kind PlatformKind) string {
	switch kind {
	case PlatformBilibiliDynamic:
		return "B站动态"
	case PlatformBilibiliVideo:
		return "B站投稿"
	case PlatformBilibiliLive:
		return "B站直播"
	case PlatformXUser:
		return "X 动态"
	default:
		return "平台订阅"
	}
}

func normalizeRSSHubBase(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return "", fmt.Errorf("RSSHub 地址配置无效")
	}
	parsed.Fragment = ""
	parsed.RawQuery = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func platformIdentifier(kind PlatformKind, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if parsed, err := url.Parse(raw); err == nil && parsed.Hostname() != "" {
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		switch kind {
		case PlatformBilibiliDynamic, PlatformBilibiliVideo:
			host := strings.ToLower(parsed.Hostname())
			if (host != "bilibili.com" && !strings.HasSuffix(host, ".bilibili.com")) || len(parts) == 0 {
				return "", fmt.Errorf("请输入 B站 UP 主 UID 或主页链接")
			}
			if parts[0] == "space" && len(parts) > 1 {
				raw = parts[1]
			} else {
				raw = parts[0]
			}
		case PlatformBilibiliLive:
			if parsed.Hostname() != "live.bilibili.com" || len(parts) == 0 {
				return "", fmt.Errorf("请输入 B站直播间号或链接")
			}
			raw = parts[0]
		case PlatformXUser:
			host := strings.ToLower(parsed.Hostname())
			if host != "x.com" && host != "www.x.com" && host != "twitter.com" && host != "www.twitter.com" || len(parts) == 0 {
				return "", fmt.Errorf("请输入 X 用户名或主页链接")
			}
			raw = parts[0]
		}
	}

	switch kind {
	case PlatformBilibiliDynamic, PlatformBilibiliVideo, PlatformBilibiliLive:
		if !digitsPattern.MatchString(raw) {
			return "", fmt.Errorf("B站 UID/房间号格式错误")
		}
	case PlatformXUser:
		raw = strings.TrimPrefix(raw, "@")
		if !xHandlePattern.MatchString(raw) {
			return "", fmt.Errorf("X 用户名格式错误")
		}
	}
	return raw, nil
}
