package tools

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/httpclient"
)

// ── URL pattern matchers ─────────────────────────────────────────────────────

var (
	reBV      = regexp.MustCompile(`(?i)BV[a-zA-Z0-9]{10}`)
	reAV      = regexp.MustCompile(`(?i)\bav(\d+)\b`)
	reB23     = regexp.MustCompile(`https?://b23\.tv/[a-zA-Z0-9]+`)
	reBangumi = regexp.MustCompile(`bilibili\.com/bangumi/play/(ep|ss)(\d+)`)
	reLive    = regexp.MustCompile(`live\.bilibili\.com/(?:blanc/|h5/)?(\d+)`)

	reZhihu   = regexp.MustCompile(`zhuanlan\.zhihu\.com/p/(\d+)`)
	reCSDN    = regexp.MustCompile(`blog\.csdn\.net/`)
	reWeibo   = regexp.MustCompile(`weibo\.(?:com|cn)/`)
	reTwitter = regexp.MustCompile(`(?:x|twitter)\.com/[0-9A-Za-z_]+/status/(\d+)`)
	reBehance = regexp.MustCompile(`behance\.net/gallery/`)
)

var biliClient = &http.Client{Timeout: 8 * time.Second}

func RegisterLinkAnalysis(b *bot.Bot) {
	b.OnGroupMessage().Handle(func(ctx *bot.GroupContext) error {
		return analyzeBiliLink(ctx, ctx.Text())
	})
}

func analyzeBiliLink(ctx *bot.GroupContext, text string) error {
	if m := reB23.FindString(text); m != "" {
		if resolved, err := resolveShortLink(m); err == nil {
			text = strings.ReplaceAll(text, m, resolved)
		}
	}

	switch {
	case reBangumi.MatchString(text):
		m := reBangumi.FindStringSubmatch(text)
		return handleBangumi(ctx, m[1], m[2])
	case reLive.MatchString(text):
		return handleLive(ctx, reLive.FindStringSubmatch(text)[1])
	case reBV.MatchString(text):
		return handleVideo(ctx, reBV.FindString(text), "")
	case reAV.MatchString(text):
		return handleVideo(ctx, "", reAV.FindStringSubmatch(text)[1])
	case reZhihu.MatchString(text):
		return handleZhihu(ctx, text)
	case reCSDN.MatchString(text):
		return handleHTMLPage(ctx, extractURL(text), "csdn")
	case reWeibo.MatchString(text):
		return handleWeibo(ctx, text)
	case reTwitter.MatchString(text):
		return handleTwitter(ctx, reTwitter.FindStringSubmatch(text))
	case reBehance.MatchString(text):
		return handleBehance(ctx, extractURL(text))
	}
	return nil
}

// ── Bilibili ─────────────────────────────────────────────────────────────────

func resolveShortLink(url string) (string, error) {
	noRedir := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := noRedir.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("no redirect")
	}
	return loc, nil
}

func fetchCover(picURL string) string {
	if picURL == "" {
		return ""
	}
	req, _ := http.NewRequest("GET", picURL, nil)
	req.Header.Set("Referer", "https://www.bilibili.com")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := biliClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil || len(data) == 0 {
		return ""
	}
	return "base64://" + base64.StdEncoding.EncodeToString(data)
}

func handleVideo(ctx *bot.GroupContext, bvid, avid string) error {
	var apiURL string
	if bvid != "" {
		apiURL = "https://api.bilibili.com/x/web-interface/view?bvid=" + bvid
	} else {
		apiURL = "https://api.bilibili.com/x/web-interface/view?aid=" + avid
	}
	var result struct {
		Code int `json:"code"`
		Data struct {
			Title    string `json:"title"`
			Desc     string `json:"desc"`
			Pic      string `json:"pic"`
			BVID     string `json:"bvid"`
			Duration int    `json:"duration"`
			Stat     struct {
				View    int `json:"view"`
				Like    int `json:"like"`
				Coin    int `json:"coin"`
				Danmaku int `json:"danmaku"`
			} `json:"stat"`
			Owner struct {
				Name string `json:"name"`
			} `json:"owner"`
		} `json:"data"`
	}
	if err := biliGet(apiURL, &result); err != nil || result.Code != 0 {
		return nil
	}
	d := result.Data
	dur := fmt.Sprintf("%d:%02d", d.Duration/60, d.Duration%60)
	desc := d.Desc
	if len([]rune(desc)) > 60 {
		desc = string([]rune(desc)[:60]) + "..."
	}
	lines := []string{
		d.Title,
		fmt.Sprintf("UP: %s  时长: %s", d.Owner.Name, dur),
		fmt.Sprintf("播放: %d  点赞: %d  投币: %d  弹幕: %d", d.Stat.View, d.Stat.Like, d.Stat.Coin, d.Stat.Danmaku),
	}
	if desc != "" && desc != "-" {
		lines = append(lines, "简介: "+desc)
	}
	lines = append(lines, "https://www.bilibili.com/video/"+d.BVID)
	msg := bot.Msg()
	if cover := fetchCover(d.Pic); cover != "" {
		msg = msg.Image(cover)
	}
	_, err := ctx.SendGroupMsg(ctx.GroupID(), msg.Text(strings.Join(lines, "\n")).Build())
	return err
}

func handleBangumi(ctx *bot.GroupContext, epType, id string) error {
	apiURL := fmt.Sprintf("https://api.bilibili.com/pgc/view/web/season?ep_id=%s", id)
	if epType == "ss" {
		apiURL = fmt.Sprintf("https://api.bilibili.com/pgc/view/web/season?season_id=%s", id)
	}
	var result struct {
		Code   int    `json:"code"`
		Result struct {
			Title    string `json:"title"`
			Cover    string `json:"cover"`
			Evaluate string `json:"evaluate"`
			Episodes []struct{ EpID int `json:"ep_id"` } `json:"episodes"`
		} `json:"result"`
	}
	if err := biliGet(apiURL, &result); err != nil || result.Code != 0 {
		return nil
	}
	r := result.Result
	desc := r.Evaluate
	if len([]rune(desc)) > 60 {
		desc = string([]rune(desc)[:60]) + "..."
	}
	text := fmt.Sprintf("%s\n共 %d 集\n简介: %s", r.Title, len(r.Episodes), desc)
	msg := bot.Msg()
	if cover := fetchCover(r.Cover); cover != "" {
		msg = msg.Image(cover)
	}
	_, err := ctx.SendGroupMsg(ctx.GroupID(), msg.Text(text).Build())
	return err
}

func handleLive(ctx *bot.GroupContext, roomID string) error {
	apiURL := "https://api.live.bilibili.com/room/v1/Room/get_info?room_id=" + roomID
	var result struct {
		Code int `json:"code"`
		Data struct {
			Title      string `json:"title"`
			LiveStatus int    `json:"live_status"`
			Online     int    `json:"online"`
			UserCover  string `json:"user_cover"`
		} `json:"data"`
	}
	if err := biliGet(apiURL, &result); err != nil || result.Code != 0 {
		return nil
	}
	d := result.Data
	status := "未开播"
	if d.LiveStatus == 1 {
		status = fmt.Sprintf("直播中  在线: %d", d.Online)
	}
	msg := bot.Msg()
	if cover := fetchCover(d.UserCover); cover != "" {
		msg = msg.Image(cover)
	}
	_, err := ctx.SendGroupMsg(ctx.GroupID(), msg.Text(fmt.Sprintf("%s\n状态: %s", d.Title, status)).Build())
	return err
}

func biliGet(url string, v any) error {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")
	resp, err := biliClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

// ── 知乎专栏 ─────────────────────────────────────────────────────────────────
// Mirror trick from Python: replace zhihu → fxzhihu to bypass login wall.

func handleZhihu(ctx *bot.GroupContext, text string) error {
	u := extractURL(text)
	if u == "" {
		return nil
	}
	u = strings.ReplaceAll(u, "zhihu.com", "fxzhihu.com")
	title, summary, err := fetchTitleSummary(u, httpclient.Direct)
	if err != nil || title == "" {
		return nil
	}
	if len([]rune(summary)) > 75 {
		summary = string([]rune(summary)[:75]) + "..."
	}
	return ctx.Reply(fmt.Sprintf("标题：%s\n摘要：%s", title, summary))
}

// ── CSDN ─────────────────────────────────────────────────────────────────────

func handleHTMLPage(ctx *bot.GroupContext, u, _ string) error {
	if u == "" {
		return nil
	}
	title, summary, err := fetchTitleSummary(u, httpclient.Direct)
	if err != nil || title == "" {
		return nil
	}
	if len([]rune(summary)) > 75 {
		summary = string([]rune(summary)[:75]) + "..."
	}
	return ctx.Reply(fmt.Sprintf("标题：%s\n摘要：%s", title, summary))
}

// ── 微博 ─────────────────────────────────────────────────────────────────────

const weiboAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func base62Encode(n int64) string {
	if n == 0 {
		return "0"
	}
	var r []byte
	for n > 0 {
		r = append([]byte{weiboAlphabet[n%62]}, r...)
		n /= 62
	}
	return string(r)
}

func mid2id(mid string) string {
	runes := []rune(mid)
	// reverse
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	size := int(math.Ceil(float64(len(runes)) / 7))
	parts := make([]string, size)
	for i := 0; i < size; i++ {
		end := (i + 1) * 7
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[i*7 : end])
		// reverse chunk back
		cr := []rune(chunk)
		for a, b := 0, len(cr)-1; a < b; a, b = a+1, b-1 {
			cr[a], cr[b] = cr[b], cr[a]
		}
		n, _ := strconv.ParseInt(string(cr), 10, 64)
		enc := base62Encode(n)
		if i < size-1 && len(enc) < 4 {
			enc = strings.Repeat("0", 4-len(enc)) + enc
		}
		parts[size-1-i] = enc
	}
	return strings.Join(parts, "")
}

var reWeiboID = []*regexp.Regexp{
	regexp.MustCompile(`m\.weibo\.cn(?:/detail|/status)?/([A-Za-z\d]+)`),
	regexp.MustCompile(`mid=([A-Za-z\d]+)`),
	regexp.MustCompile(`(?:weibo\.com)/[A-Za-z\d]+/([A-Za-z\d]+)`),
}

func handleWeibo(ctx *bot.GroupContext, text string) error {
	var weiboID string
	for i, re := range reWeiboID {
		if m := re.FindStringSubmatch(text); m != nil {
			if i == 1 {
				weiboID = mid2id(m[1])
			} else {
				weiboID = m[1]
			}
			break
		}
	}
	if weiboID == "" {
		return nil
	}

	// Generate visitor SUB cookie
	cookie, err := weiboVisitorCookie()
	if err != nil {
		return nil
	}

	apiURL := fmt.Sprintf("https://weibo.com/ajax/statuses/show?id=%s&locale=zh-CN&isGetLongText=true", weiboID)
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Cookie", "SUB="+cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := httpclient.Direct.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil
	}
	rawText, _ := data["text_raw"].(string)
	if rawText == "" {
		return nil
	}
	title := strings.SplitN(rawText, "\n", 2)[0]
	if len([]rune(title)) > 80 {
		title = string([]rune(title)[:80]) + "..."
	}
	return ctx.Reply("微博: " + title)
}

func weiboVisitorCookie() (string, error) {
	req, _ := http.NewRequest("POST", "https://passport.weibo.com/visitor/genvisitor2", strings.NewReader("cb=visitor_gray_callback&tid=&from=weibo"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := httpclient.Direct.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	re := regexp.MustCompile(`\{.*\}`)
	raw := re.Find(body)
	if raw == nil {
		return "", fmt.Errorf("no json")
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	data, _ := result["data"].(map[string]any)
	sub, _ := data["sub"].(string)
	if sub == "" {
		return "", fmt.Errorf("no sub")
	}
	return sub, nil
}

// ── Twitter/X ─────────────────────────────────────────────────────────────────
// Uses fxtwitter.com public API (no auth needed).

func handleTwitter(ctx *bot.GroupContext, m []string) error {
	if len(m) < 2 {
		return nil
	}
	tweetID := m[1]
	// extract username from original URL
	re := regexp.MustCompile(`(?:x|twitter)\.com/([0-9A-Za-z_]+)/status/`)
	um := re.FindStringSubmatch(ctx.Text())
	if um == nil {
		return nil
	}
	username := um[1]
	apiURL := fmt.Sprintf("https://api.fxtwitter.com/%s/status/%s", username, tweetID)

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := httpclient.Proxy.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var data struct {
		Tweet struct {
			Text   string `json:"text"`
			Author struct {
				Name       string `json:"name"`
				ScreenName string `json:"screen_name"`
			} `json:"author"`
			Media struct {
				Photos []struct{ URL string `json:"url"` } `json:"photos"`
			} `json:"media"`
		} `json:"tweet"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil
	}
	t := data.Tweet
	header := fmt.Sprintf("@%s (%s)\n%s", t.Author.ScreenName, t.Author.Name, t.Text)
	msg := bot.Msg().Text(header)
	for _, p := range t.Media.Photos {
		if p.URL != "" {
			msg = msg.Image(p.URL)
		}
	}
	_, err = ctx.SendGroupMsg(ctx.GroupID(), msg.Build())
	return err
}

// ── Behance ─────────────────────────────────────────────────────────────────

func handleBehance(ctx *bot.GroupContext, u string) error {
	if u == "" {
		return nil
	}
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Cookie", "ilo0=1")
	resp, err := httpclient.Proxy.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	title := htmlMeta(body, "og:title", "property")
	if title == "" {
		title = htmlTitle(body)
	}
	desc := htmlMeta(body, "og:description", "property")
	imgURL := htmlMeta(body, "og:image", "property")

	if title == "" {
		return nil
	}
	if len([]rune(desc)) > 100 {
		desc = string([]rune(desc)[:100]) + "..."
	}
	msg := bot.Msg()
	if imgURL != "" {
		if cover := fetchCover(imgURL); cover != "" {
			msg = msg.Image(cover)
		}
	}
	text := fmt.Sprintf("标题: %s", title)
	if desc != "" {
		text += "\n概述: " + desc
	}
	_, err = ctx.SendGroupMsg(ctx.GroupID(), msg.Text(text).Build())
	return err
}

// ── HTML helpers ─────────────────────────────────────────────────────────────

func fetchTitleSummary(u string, client *http.Client) (title, summary string, err error) {
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return htmlTitle(body), htmlMeta(body, "description", "name"), nil
}

func htmlTitle(body []byte) string {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return ""
	}
	var find func(*html.Node) string
	find = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
			return strings.TrimSpace(n.FirstChild.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if v := find(c); v != "" {
				return v
			}
		}
		return ""
	}
	return find(doc)
}

func htmlMeta(body []byte, key, attr string) string {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return ""
	}
	var find func(*html.Node) string
	find = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "meta" {
			var matchKey, content bool
			var contentVal string
			for _, a := range n.Attr {
				if strings.EqualFold(a.Key, attr) && strings.EqualFold(a.Val, key) {
					matchKey = true
				}
				if strings.EqualFold(a.Key, "content") {
					content = true
					contentVal = a.Val
				}
			}
			if matchKey && content {
				return strings.TrimSpace(contentVal)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if v := find(c); v != "" {
				return v
			}
		}
		return ""
	}
	return find(doc)
}

// extractURL pulls the first http(s) URL out of a message string.
func extractURL(text string) string {
	re := regexp.MustCompile(`https?://[^\s]+`)
	return re.FindString(text)
}
