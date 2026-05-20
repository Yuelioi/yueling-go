package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func init() {
	registerDateCalc()
	registerMakeDecision()
	registerSummarizeText()
	registerCodeHelper()
	registerConvertCurrency()
}

// ── 日期计算 ──────────────────────────────────────────────────────────────────

func registerDateCalc() {
	ai.Register(ai.ToolMeta{
		Name:        "date_calc",
		Description: "日期计算：倒计时、推算日期、星期查询",
		Tags:        []string{"数学", "信息"},
		Triggers:    []string{"倒计时", "距离"},
		Patterns:    []string{`\d+天(后|前)是`, `距离.+还有`},
		Slots:       []string{"日期计算", "星期几", "多少天"},
		Params: []ai.Param{
			{Name: "query", Type: "string", Description: "日期问题，如「距离2025-01-01还有多少天」", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			query := strings.TrimSpace(ctx.String("query"))
			now := time.Now()
			cfg := config.C.AI
			client := ai.NewClient(cfg.DeepSeekKey, cfg.BaseURL)
			resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
				Model: cfg.Model,
				Messages: []openai.ChatCompletionMessage{
					{
						Role: openai.ChatMessageRoleSystem,
						Content: fmt.Sprintf(
							"你是日期计算器。当前时间: %s。直接回答结果，一句话，不要废话。",
							now.Format("2006-01-02 15:04"),
						),
					},
					{Role: openai.ChatMessageRoleUser, Content: query},
				},
				MaxTokens:   100,
				Temperature: 0,
			})
			if err != nil || len(resp.Choices) == 0 {
				return "计算失败", nil
			}
			return strings.TrimSpace(resp.Choices[0].Message.Content), nil
		},
	})
}

// ── 帮我做选择 ────────────────────────────────────────────────────────────────

func registerMakeDecision() {
	reasons := []string{
		"命运的齿轮如此转动",
		"月灵掐指一算",
		"直觉告诉我",
		"量子力学的选择",
		"塔罗牌指引",
		"第六感",
		"风水学分析后",
	}

	ai.Register(ai.ToolMeta{
		Name:        "make_decision",
		Description: "帮用户做选择，解决选择困难症",
		Tags:        []string{"娱乐"},
		Triggers:    []string{"帮我选"},
		Patterns:    []string{`选择困难`},
		Slots:       []string{"帮我做决定", "选A还是B"},
		Params: []ai.Param{
			{Name: "options", Type: "string", Description: "用空格或逗号分隔的选项，如「A B C」", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			raw := strings.NewReplacer("，", " ", ",", " ").Replace(strings.TrimSpace(ctx.String("options")))
			var items []string
			for _, o := range strings.Fields(raw) {
				if o != "" {
					items = append(items, o)
				}
			}
			if len(items) < 2 {
				return "至少给我两个选项！", nil
			}
			if len(items) > 10 {
				return "选项太多了，最多10个", nil
			}
			chosen := items[rand.Intn(len(items))]
			reason := reasons[rand.Intn(len(reasons))]
			return fmt.Sprintf("在 %s 中...\n%s，选「%s」！", strings.Join(items, " / "), reason, chosen), nil
		},
	})
}

// ── 文本摘要 ──────────────────────────────────────────────────────────────────

func registerSummarizeText() {
	ai.Register(ai.ToolMeta{
		Name:        "summarize_text",
		Description: "将长文本压缩为简短摘要，保留要点",
		Tags:        []string{"语言"},
		Triggers:    []string{"摘要"},
		Slots:       []string{"文本总结", "太长不看", "概括"},
		Params: []ai.Param{
			{Name: "text", Type: "string", Description: "需要摘要的文本", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			text := strings.TrimSpace(ctx.String("text"))
			if len([]rune(text)) < 20 {
				return "文本太短，不需要摘要", nil
			}
			if len(text) > 3000 {
				text = text[:3000]
			}
			cfg := config.C.AI
			client := ai.NewClient(cfg.DeepSeekKey, cfg.BaseURL)
			resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
				Model: cfg.Model,
				Messages: []openai.ChatCompletionMessage{
					{Role: openai.ChatMessageRoleSystem, Content: "用3-5个要点总结以下内容，每个要点一句话，简洁直接。"},
					{Role: openai.ChatMessageRoleUser, Content: text},
				},
				MaxTokens:   300,
				Temperature: 0.2,
			})
			if err != nil || len(resp.Choices) == 0 {
				return "摘要失败", nil
			}
			return strings.TrimSpace(resp.Choices[0].Message.Content), nil
		},
	})
}

// ── 编程助手 ──────────────────────────────────────────────────────────────────

func registerCodeHelper() {
	ai.Register(ai.ToolMeta{
		Name:        "code_helper",
		Description: "回答编程相关问题，支持代码解释、生成和调试",
		Tags:        []string{"信息"},
		Triggers:    []string{"代码", "编程"},
		Patterns:    []string{`(写个|帮我写).*(正则|代码|脚本)`},
		Slots:       []string{"编程帮助", "代码解释", "写代码"},
		Params: []ai.Param{
			{Name: "question", Type: "string", Description: "编程问题", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			q := strings.TrimSpace(ctx.String("question"))
			cfg := config.C.AI
			client := ai.NewClient(cfg.DeepSeekKey, cfg.BaseURL)
			resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
				Model: cfg.Model,
				Messages: []openai.ChatCompletionMessage{
					{
						Role:    openai.ChatMessageRoleSystem,
						Content: "你是编程助手。回答简洁，代码不超过30行，用中文解释，代码用英文。直接给答案。",
					},
					{Role: openai.ChatMessageRoleUser, Content: q},
				},
				MaxTokens:   500,
				Temperature: 0.2,
			})
			if err != nil || len(resp.Choices) == 0 {
				return "回答失败", nil
			}
			return strings.TrimSpace(resp.Choices[0].Message.Content), nil
		},
	})
}

// ── 汇率换算 ──────────────────────────────────────────────────────────────────

func registerConvertCurrency() {
	ai.Register(ai.ToolMeta{
		Name:        "convert_currency",
		Description: "汇率换算，支持主要货币，可指定金额",
		Tags:        []string{"数学", "信息"},
		Triggers:    []string{"汇率", "换算"},
		Patterns:    []string{`\d+.*(美元|日元|欧元|韩元|英镑|港币)`},
		Slots:       []string{"货币转换", "外币兑换"},
		Params: []ai.Param{
			{Name: "amount", Type: "number", Description: "金额", Required: true},
			{Name: "source", Type: "string", Description: "源货币代码（如USD/JPY/EUR）", Required: true},
			{Name: "target", Type: "string", Description: "目标货币代码，默认CNY", Required: false},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			amount := ctx.Float("amount")
			source := strings.ToUpper(strings.TrimSpace(ctx.String("source")))
			target := strings.ToUpper(strings.TrimSpace(ctx.String("target")))
			if target == "" {
				target = "CNY"
			}
			resp, err := httpClient.Get(fmt.Sprintf("https://open.er-api.com/v6/latest/%s", source))
			if err != nil {
				return "汇率查询失败", nil
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			var data struct {
				Rates map[string]float64 `json:"rates"`
			}
			if json.Unmarshal(body, &data) != nil {
				return "汇率查询失败", nil
			}
			rate, ok := data.Rates[target]
			if !ok {
				return fmt.Sprintf("不支持的货币: %s", target), nil
			}
			result := amount * rate
			return fmt.Sprintf("%.2f %s = %.2f %s（汇率: 1 %s = %.4f %s）", amount, source, result, target, source, rate, target), nil
		},
	})
}
