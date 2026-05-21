package funny

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/httpclient"
)

func RegisterTraceMoe(b *bot.Bot) {
	b.OnCommand("场景识别").Handle(func(ctx *bot.CommandContext) error {
		urls := ctx.Message().ImageURLs()
		if len(urls) == 0 {
			return ctx.Reply("请附上图片")
		}
		imgData, err := fetchImageBytes(urls[0])
		if err != nil || len(imgData) == 0 {
			return ctx.Reply("图片下载失败")
		}
		results, err := traceSearch(imgData)
		if err != nil || len(results) == 0 {
			return ctx.Reply("未识别到动漫场景")
		}
		return ctx.Reply(formatTraceResults(results))
	})
}

type traceResult struct {
	Anilist    map[string]any `json:"anilist"`
	Episode    any            `json:"episode"`
	From       float64        `json:"from"`
	Similarity float64        `json:"similarity"`
}

func traceSearch(imgData []byte) ([]traceResult, error) {
	req, err := http.NewRequest("POST", "https://api.trace.moe/search?anilistInfo", bytes.NewReader(imgData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "image/jpeg")
	resp, err := httpclient.Proxy.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("trace.moe status %d", resp.StatusCode)
	}
	var out struct {
		Result []traceResult `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

func formatTraceResults(results []traceResult) string {
	var lines []string
	for _, item := range results[:min(3, len(results))] {
		title := "未知"
		if t, ok := item.Anilist["title"].(map[string]any); ok {
			if n, ok := t["native"].(string); ok && n != "" {
				title = n
			}
		}
		ep := fmt.Sprintf("%v", item.Episode)
		sim := fmt.Sprintf("%.1f", item.Similarity*100)
		lines = append(lines,
			fmt.Sprintf("动漫: %s", title),
			fmt.Sprintf("集数: EP%s  时间: %.0fs  相似度: %s%%", ep, item.From, sim),
			"---",
		)
	}
	return strings.Join(lines, "\n")
}

func fetchImageBytes(url string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := httpclient.Direct.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
