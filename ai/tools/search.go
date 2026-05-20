package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
)

func init() {
	registerSearchFlights()
	registerSearchTrains()
	ai.Register(ai.ToolMeta{
		Name:        "web_search",
		Description: "搜索互联网上的最新信息，适合时事、新闻、不确定的事实查询",
		Tags:        []string{"搜索", "search", "查询", "新闻"},
		Triggers:    []string{"搜索", "search", "查一下", "最新", "新闻", "是什么"},
		Slots:       []string{"query"},
		Params: []ai.Param{
			{Name: "query", Type: "string", Description: "搜索关键词", Required: true},
			{Name: "count", Type: "integer", Description: "返回结果数量，默认5", Required: false},
		},
		Handler: searchHandler,
	})
}

func searchHandler(ctx *ai.ToolContext) (string, error) {
	query := ctx.String("query")
	if query == "" {
		return "请提供搜索关键词", nil
	}
	count := int(ctx.Int("count"))
	if count <= 0 {
		count = 5
	}

	key := config.C.Tools.TavilyKey
	if key == "" {
		return "搜索功能未配置（缺少 tavily_key）", nil
	}
	return tavilySearch(query, count, key)
}

func registerSearchFlights() {
	ai.Register(ai.ToolMeta{
		Name:        "search_flights",
		Description: "查询机票/航班信息",
		Tags:        []string{"搜索", "出行"},
		Triggers:    []string{"机票", "航班", "飞机票"},
		Patterns:    []string{`.+到.+机票`, `.+到.+航班`},
		Slots:       []string{"机票", "航班"},
		Params: []ai.Param{
			{Name: "departure", Type: "string", Description: "出发城市", Required: true},
			{Name: "arrival", Type: "string", Description: "到达城市", Required: true},
			{Name: "date", Type: "string", Description: "出发日期(YYYY-MM-DD)，不填=今天", Required: false},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			dep := strings.TrimSpace(ctx.String("departure"))
			arr := strings.TrimSpace(ctx.String("arrival"))
			date := strings.TrimSpace(ctx.String("date"))
			query := strings.TrimSpace(dep + "到" + arr + "机票 " + date)
			key := config.C.Tools.TavilyKey
			if key == "" {
				return "搜索功能未配置（缺少 tavily_key）", nil
			}
			return tavilySearch(query, 5, key)
		},
	})
}

func registerSearchTrains() {
	ai.Register(ai.ToolMeta{
		Name:        "search_trains",
		Description: "查询火车票/高铁票信息",
		Tags:        []string{"搜索", "出行"},
		Triggers:    []string{"火车", "高铁", "车票"},
		Patterns:    []string{`.+到.+火车`, `.+到.+高铁`},
		Slots:       []string{"火车票", "高铁票"},
		Params: []ai.Param{
			{Name: "departure", Type: "string", Description: "出发城市/车站", Required: true},
			{Name: "arrival", Type: "string", Description: "到达城市/车站", Required: true},
			{Name: "date", Type: "string", Description: "出发日期(YYYY-MM-DD)，不填=今天", Required: false},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			dep := strings.TrimSpace(ctx.String("departure"))
			arr := strings.TrimSpace(ctx.String("arrival"))
			date := strings.TrimSpace(ctx.String("date"))
			query := strings.TrimSpace(dep + "到" + arr + "高铁火车票 " + date)
			key := config.C.Tools.TavilyKey
			if key == "" {
				return "搜索功能未配置（缺少 tavily_key）", nil
			}
			return tavilySearch(query, 5, key)
		},
	})
}

func tavilySearch(query string, count int, key string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"api_key":     key,
		"query":       query,
		"max_results": count,
	})

	resp, err := httpClient.Post("https://api.tavily.com/search", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var result struct {
		Results []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("search parse error: %w", err)
	}
	if len(result.Results) == 0 {
		return fmt.Sprintf("没有找到关于【%s】的结果", query), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("搜索【%s】的结果：\n", query))
	for i, r := range result.Results {
		content := r.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		sb.WriteString(fmt.Sprintf("\n[%d] %s\n%s\n%s", i+1, r.Title, content, r.URL))
	}
	return sb.String(), nil
}
