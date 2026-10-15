package tools

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services"
)

func (t *toolset) registerEmoticonSearch() {
	t.registry.Register(ai.ToolMeta{
		Name: "search_emoticons", Description: "查询本地已有表情包的名字，等同两个空格加关键词。按文件名包含关键词匹配，每页10个；用户问‘有哪些开心的表情’‘查猫猫表情’时使用。只列名字，不发送或生成图片；结果不代表看懂图片，无匹配时如实说明，不编造名称。",
		Triggers:   []string{"查询表情", "查表情", "找表情", "搜索表情", "表情包", "表情"},
		Permission: ai.PermMember, PluginID: catalog.PluginEmoticon, ReadOnly: true,
		Params: []ai.Param{
			{Name: "keyword", Type: "string", Description: "从用户请求提取的表情名字关键词；明确浏览全部时可留空"},
			{Name: "page", Type: "integer", Description: "页码，从1开始；翻页时沿用之前的关键词"},
		},
		ResultHandler: searchEmoticons,
	})
}

func searchEmoticons(c *ai.ToolContext) ai.ToolResult {
	keyword := strings.TrimSpace(c.String("keyword"))
	page := c.Int("page")
	if page == 0 {
		page = 1
	}
	if utf8.RuneCountInString(keyword) > 128 || page < 1 || page > 1000000 {
		return outcome(ai.ToolRejected, "关键词最多128字，页码须在1至1000000之间。", false)
	}
	if c.Context().Err() != nil {
		return outcome(ai.ToolFailed, "表情查询已取消。", false)
	}
	names, err := services.ListImageNames("表情", keyword)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return outcome(ai.ToolFailed, "暂时无法读取表情库，请稍后再试。", false)
	}
	if c.Context().Err() != nil {
		return outcome(ai.ToolFailed, "表情查询已取消。", false)
	}
	total := len(names)
	start := min(int(page-1)*10, total)
	preview := append([]string{}, names[start:min(start+10, total)]...)
	data, _ := json.Marshal(struct {
		Keyword string   `json:"keyword"`
		Names   []string `json:"names"`
		Total   int      `json:"total"`
		Page    int64    `json:"page"`
		HasMore bool     `json:"has_more"`
	}{keyword, preview, total, page, start+len(preview) < total})
	return outcome(ai.ToolSucceeded, string(data), false)
}
