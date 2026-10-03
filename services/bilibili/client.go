package bilibili

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	bpi "github.com/Yuelioi/bpi-go"
	core "github.com/Yuelioi/bpi-go/client"
	"github.com/Yuelioi/bpi-go/ids"
	"github.com/Yuelioi/bpi-go/message"
	"github.com/Yuelioi/bpi-go/user"
	"github.com/Yuelioi/bpi-go/video"
	"github.com/Yuelioi/yueling-go/config"
)

type Client struct {
	sdk         *core.Client
	uid         int64
	recentCount int
	accountID   int64
}

type VideoStats struct {
	View        video.View
	Part        int
	PartTitle   string
	Online      video.OnlineTotal
	OnlineError error
}

type RecentVideo struct {
	BVID  string
	Title string
	Stats VideoStats
	Error error
}

func New(cfg config.BilibiliConfig, options ...bpi.Option) (*Client, error) {
	if strings.TrimSpace(cfg.Cookie) != "" {
		options = append(options, bpi.WithCookie(cfg.Cookie))
	}
	sdk, err := core.NewClient(options...)
	if err != nil {
		return nil, err
	}
	uid := cfg.UID
	accountID := int64(0)
	req := &http.Request{Header: http.Header{"Cookie": {cfg.Cookie}}}
	for _, cookie := range req.Cookies() {
		if cookie.Name == "DedeUserID" {
			accountID, _ = strconv.ParseInt(cookie.Value, 10, 64)
		}
	}
	if uid == 0 {
		uid = accountID
	}
	count := cfg.RecentCount
	if count == 0 {
		count = 5
	}
	return &Client{sdk: sdk, uid: uid, recentCount: count, accountID: accountID}, nil
}

func (c *Client) UID() int64     { return c.uid }
func (c *Client) HasLogin() bool { return c.sdk.HasLoginCookies() }

func (c *Client) ContactName(ctx context.Context, uid int64) (string, error) {
	if uid <= 0 {
		return "", fmt.Errorf("无效的联系人")
	}
	mid, err := ids.NewMID(uint64(uid))
	if err != nil {
		return "", err
	}
	profile, err := user.NewClient(c.sdk).Card(ctx, user.NewCardParams(mid))
	return profile.Card.Name, err
}

func parseVideo(input string) (video.ViewParams, error) {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
		u, err := url.Parse(input)
		if err != nil || (u.Hostname() != "www.bilibili.com" && u.Hostname() != "bilibili.com" && u.Hostname() != "m.bilibili.com") {
			return video.ViewParams{}, fmt.Errorf("请提供 BV号、av号或 bilibili.com 视频链接")
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) != 2 || parts[0] != "video" {
			return video.ViewParams{}, fmt.Errorf("请提供完整的视频链接")
		}
		input = parts[1]
	}
	if strings.HasPrefix(input, "BV") {
		id, err := ids.ParseBVID(input)
		if err != nil {
			return video.ViewParams{}, fmt.Errorf("BV号格式不正确")
		}
		return video.ViewByBVID(id), nil
	}
	if strings.HasPrefix(strings.ToLower(input), "av") {
		id, err := ids.ParseAID(input[2:])
		if err != nil {
			return video.ViewParams{}, fmt.Errorf("av号格式不正确")
		}
		return video.ViewByAID(id), nil
	}
	return video.ViewParams{}, fmt.Errorf("请提供 BV号、av号或 bilibili.com 视频链接；暂不支持 b23 短链接")
}

func (c *Client) Video(ctx context.Context, input string, part int) (VideoStats, error) {
	params, err := parseVideo(input)
	if err != nil {
		return VideoStats{}, err
	}
	if part < 1 {
		return VideoStats{}, fmt.Errorf("分P序号必须大于零")
	}
	view, err := video.NewClient(c.sdk).View(ctx, params)
	if err != nil {
		return VideoStats{}, err
	}
	result := VideoStats{View: view, Part: part}
	cid := view.CID
	if len(view.Pages) > 0 {
		if part > len(view.Pages) {
			return VideoStats{}, fmt.Errorf("该视频只有 %d 个分P", len(view.Pages))
		}
		cid = view.Pages[part-1].CID
		result.PartTitle = view.Pages[part-1].Part
	} else if part != 1 {
		return VideoStats{}, fmt.Errorf("该视频没有此分P")
	}
	result.Online, result.OnlineError = video.NewClient(c.sdk).OnlineTotal(ctx, video.OnlineTotalByBVID(view.BVID, cid))
	return result, nil
}

func (c *Client) Recent(ctx context.Context, uid int64, count int) ([]RecentVideo, error) {
	if uid == 0 {
		uid = c.uid
	}
	if count == 0 {
		count = c.recentCount
	}
	if uid <= 0 {
		return nil, fmt.Errorf("请在 bilibili.uid 配置 UP主 UID，或使用 B站最近视频 <UID> [数量]")
	}
	if count < 1 || count > 10 {
		return nil, fmt.Errorf("视频数量必须在 1 到 10 之间")
	}
	mid, err := ids.NewMID(uint64(uid))
	if err != nil {
		return nil, err
	}
	params, err := user.NewUploadedVideosParams(mid).WithPageSize(30)
	if err != nil {
		return nil, err
	}
	list, err := user.NewClient(c.sdk).UploadedVideos(ctx, params)
	if err != nil {
		return nil, err
	}
	items := list.List.Videos
	if len(items) > count {
		items = items[:count]
	}
	results := make([]RecentVideo, 0, len(items))
	for _, item := range items {
		stats, err := c.Video(ctx, item.BVID.String(), 1)
		results = append(results, RecentVideo{BVID: item.BVID.String(), Title: item.Title, Stats: stats, Error: err})
	}
	return results, nil
}

func (c *Client) Unread(ctx context.Context) (message.SingleUnread, error) {
	if !c.HasLogin() {
		return message.SingleUnread{}, fmt.Errorf("请先配置 bilibili.cookie（需要 SESSDATA）")
	}
	return message.NewClient(c.sdk).SingleUnread(ctx, message.NewSingleUnreadParams().ShowUnfollowList(true))
}
