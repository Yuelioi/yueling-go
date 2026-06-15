package tools

import (
	"encoding/json"

	"github.com/Yuelioi/yueling-go/bot"
)

const packMaxDepth = 5

// collectImages 递归收集一条消息里的图片 url（含展开的合并转发）。
// getForward 注入以便单测；maxImages 张数上限；visited 记已展开 forward id 防循环。
func collectImages(msg bot.Message, getForward func(string) ([]bot.Message, error), depth, maxImages int, visited map[string]bool, out *[]string) {
	if depth > packMaxDepth {
		return
	}
	for _, s := range msg {
		if len(*out) >= maxImages {
			return
		}
		switch s.Type {
		case "image":
			var d struct {
				File string `json:"file"`
				URL  string `json:"url"`
			}
			if json.Unmarshal(s.Data, &d) == nil {
				if d.URL != "" {
					*out = append(*out, d.URL)
				} else if d.File != "" {
					*out = append(*out, d.File)
				}
			}
		case "forward":
			var d struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(s.Data, &d) == nil && d.ID != "" && !visited[d.ID] {
				visited[d.ID] = true
				if inner, err := getForward(d.ID); err == nil {
					for _, im := range inner {
						collectImages(im, getForward, depth+1, maxImages, visited, out)
					}
				}
			}
		}
	}
}
