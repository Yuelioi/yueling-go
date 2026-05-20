package funny

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
)

var hotClient = &http.Client{
	Timeout: 8 * time.Second,
}

const hotUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

func RegisterHot(b *bot.Bot) {
	b.OnCommand("查热搜", "热搜").Handle(func(ctx *bot.CommandContext) error {
		type result struct {
			name  string
			items []string
		}
		sources := []struct {
			name string
			fn   func() []string
		}{
			{"微博", fetchWeiboHot},
			{"B站", fetchBilibiliHot},
			{"百度", fetchBaiduHot},
			{"抖音", fetchDouyinHot},
		}

		results := make([]result, len(sources))
		var wg sync.WaitGroup
		for i, s := range sources {
			wg.Add(1)
			go func(idx int, name string, fn func() []string) {
				defer wg.Done()
				results[idx] = result{name: name, items: fn()}
			}(i, s.name, s.fn)
		}
		wg.Wait()

		var sb strings.Builder
		for _, r := range results {
			sb.WriteString(fmt.Sprintf("── %s热搜 ──\n", r.name))
			if len(r.items) == 0 {
				sb.WriteString("获取失败\n")
			} else {
				for i, item := range r.items {
					sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, item))
				}
			}
			sb.WriteString("\n")
		}
		return ctx.Reply(strings.TrimRight(sb.String(), "\n"))
	})
}

func hotGet(url string, extraHeaders ...string) ([]byte, bool) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("User-Agent", hotUA)
	for i := 0; i+1 < len(extraHeaders); i += 2 {
		req.Header.Set(extraHeaders[i], extraHeaders[i+1])
	}
	resp, err := hotClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return nil, false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, err == nil
}

func fetchWeiboHot() []string {
	body, ok := hotGet(
		"https://weibo.com/ajax/side/hotSearch",
		"Referer", "https://weibo.com/hot/search",
	)
	if !ok {
		return nil
	}
	var data struct {
		Data struct {
			Realtime []struct {
				Word string `json:"word"`
			} `json:"realtime"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &data) != nil {
		return nil
	}
	var out []string
	for _, item := range data.Data.Realtime {
		if item.Word != "" {
			out = append(out, item.Word)
			if len(out) >= 10 {
				break
			}
		}
	}
	return out
}

func fetchBilibiliHot() []string {
	body, ok := hotGet("https://s.search.bilibili.com/main/hotword")
	if !ok {
		return nil
	}
	var data struct {
		List []struct {
			ShowName string `json:"show_name"`
		} `json:"list"`
	}
	if json.Unmarshal(body, &data) != nil {
		return nil
	}
	var out []string
	for _, item := range data.List {
		if item.ShowName != "" {
			out = append(out, item.ShowName)
			if len(out) >= 10 {
				break
			}
		}
	}
	return out
}

func fetchBaiduHot() []string {
	body, ok := hotGet("https://top.baidu.com/api/board?platform=wise&tab=realtime")
	if !ok {
		return nil
	}
	var data struct {
		Data struct {
			Cards []struct {
				Content []struct {
					Word string `json:"word"`
				} `json:"content"`
			} `json:"cards"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &data) != nil || len(data.Data.Cards) == 0 {
		return nil
	}
	var out []string
	for _, item := range data.Data.Cards[0].Content {
		if item.Word != "" {
			out = append(out, item.Word)
			if len(out) >= 10 {
				break
			}
		}
	}
	return out
}

func fetchDouyinHot() []string {
	body, ok := hotGet(
		"https://www.iesdouyin.com/web/api/v2/hotsearch/billboard/word/",
		"User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15",
	)
	if !ok {
		return nil
	}
	var data struct {
		WordList []struct {
			Word string `json:"word"`
		} `json:"word_list"`
	}
	if json.Unmarshal(body, &data) != nil {
		return nil
	}
	var out []string
	for _, item := range data.WordList {
		if item.Word != "" {
			out = append(out, item.Word)
			if len(out) >= 10 {
				break
			}
		}
	}
	return out
}
