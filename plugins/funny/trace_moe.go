package funny

import (
	"fmt"
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
		ctx.React(bot.EmojiProcessing)
		imgData, err := httpclient.Direct.GetBytes(urls[0])
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
	var out struct {
		Result []traceResult `json:"result"`
	}
	if err := httpclient.Proxy.Post("https://api.trace.moe/search?anilistInfo", "image/jpeg", imgData, &out); err != nil {
		return nil, fmt.Errorf("trace.moe: %w", err)
	}
	return out.Result, nil
}

func formatTraceResults(results []traceResult) string {
	var lines []string
	n := len(results)
	if n > 3 {
		n = 3
	}
	for _, item := range results[:n] {
		title := "未知"
		if t, ok := item.Anilist["title"].(map[string]any); ok {
			if native, ok := t["native"].(string); ok && native != "" {
				title = native
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
