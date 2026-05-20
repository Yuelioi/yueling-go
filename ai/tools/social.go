package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	openai "github.com/sashabaranov/go-openai"
)

func init() {
	registerHoroscope()
	registerDailyFortune()
	registerIdiomChain()
	registerAnonymousMessage()
	registerAffinityRanking()
}

// ── 星座运势 ──────────────────────────────────────────────────────────────────

var signMap = map[string]string{
	"白羊座": "aries", "金牛座": "taurus", "双子座": "gemini",
	"巨蟹座": "cancer", "狮子座": "leo", "处女座": "virgo",
	"天秤座": "libra", "天蝎座": "scorpio", "射手座": "sagittarius",
	"摩羯座": "capricorn", "水瓶座": "aquarius", "双鱼座": "pisces",
}

func registerHoroscope() {
	signs := make([]string, 0, len(signMap))
	for k := range signMap {
		signs = append(signs, k)
	}
	ai.Register(ai.ToolMeta{
		Name:        "horoscope",
		Description: "查询星座今日运势",
		Tags:        []string{"娱乐", "运势"},
		Triggers:    []string{"星座", "运势"},
		Patterns:    []string{`(白羊|金牛|双子|巨蟹|狮子|处女|天秤|天蝎|射手|摩羯|水瓶|双鱼).{0,3}运势`},
		Slots:       []string{"星座", "运势"},
		Params: []ai.Param{
			{Name: "sign", Type: "string", Description: "星座名，如白羊座、天蝎座", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			sign := strings.TrimSpace(ctx.String("sign"))
			if _, ok := signMap[sign]; !ok {
				for k := range signMap {
					if strings.HasPrefix(k, sign) || strings.Contains(k, sign) {
						sign = k
						break
					}
				}
			}
			eng, ok := signMap[sign]
			if !ok {
				return fmt.Sprintf("不认识的星座「%s」，支持：%s", sign, strings.Join(signs, "/")), nil
			}

			resp, err := httpClient.Get(fmt.Sprintf("https://api.vvhan.com/api/horoscope?type=%s&time=today", eng))
			if err != nil {
				return "查询失败：网络错误", nil
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			var data struct {
				Success bool `json:"success"`
				Data    struct {
					ShortComment string `json:"shortcomment"`
					Fortune      []struct {
						Name string `json:"name"`
						Star string `json:"star"`
						Text string `json:"text"`
					} `json:"fortune"`
					LuckyNum   string `json:"luckynum"`
					LuckyColor string `json:"luckycolor"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &data); err != nil || !data.Success {
				return "查询失败", nil
			}
			d := data.Data
			var sb strings.Builder
			sb.WriteString(sign + "今日运势\n")
			if d.ShortComment != "" {
				sb.WriteString(d.ShortComment + "\n")
			}
			for _, f := range d.Fortune[:min(4, len(d.Fortune))] {
				sb.WriteString(fmt.Sprintf("%s: %s %s\n", f.Name, f.Star, f.Text))
			}
			if d.LuckyNum != "" || d.LuckyColor != "" {
				sb.WriteString(fmt.Sprintf("幸运数字: %s  幸运颜色: %s", d.LuckyNum, d.LuckyColor))
			}
			return strings.TrimRight(sb.String(), "\n"), nil
		},
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── 今日宜忌 ──────────────────────────────────────────────────────────────────

func registerDailyFortune() {
	goodThings := []string{
		"摸鱼", "写代码", "吃火锅", "打游戏", "看番", "逛街", "告白",
		"学习", "健身", "睡懒觉", "出门旅行", "网购", "做饭", "追剧",
		"约朋友", "喝奶茶", "拍照", "画画", "弹琴", "唱歌", "写日记",
	}
	badThings := []string{
		"熬夜", "吵架", "剁手", "迟到", "说谎", "偷懒", "发脾气",
		"翘课", "玩手机到半夜", "不吃早饭", "忘带钥匙", "踩水坑",
		"忘记保存", "开黑连败", "修电脑", "相亲", "体检", "考试",
	}
	lucks := []string{"大吉", "中吉", "小吉", "吉", "末吉", "凶", "小凶"}

	ai.Register(ai.ToolMeta{
		Name:        "daily_fortune",
		Description: "查看今日宜忌（老黄历风格）",
		Tags:        []string{"娱乐", "运势"},
		Triggers:    []string{"老黄历", "宜忌", "今天适合"},
		Patterns:    []string{`今天(适合|宜)`},
		Slots:       []string{"今日运势", "老黄历"},
		Params:      []ai.Param{},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			yi := rand.Perm(len(goodThings))[:3]
			ji := rand.Perm(len(badThings))[:3]
			luck := lucks[rand.Intn(len(lucks))]
			yiStr := fmt.Sprintf("%s、%s、%s", goodThings[yi[0]], goodThings[yi[1]], goodThings[yi[2]])
			jiStr := fmt.Sprintf("%s、%s、%s", badThings[ji[0]], badThings[ji[1]], badThings[ji[2]])
			return fmt.Sprintf("今日运势: %s\n宜: %s\n忌: %s", luck, yiStr, jiStr), nil
		},
	})
}

// ── 成语接龙 ──────────────────────────────────────────────────────────────────

func registerIdiomChain() {
	ai.Register(ai.ToolMeta{
		Name:        "idiom_chain",
		Description: "成语接龙，给出一个成语，月灵接下一个",
		Tags:        []string{"娱乐", "文字游戏"},
		Triggers:    []string{"接龙", "成语接龙"},
		Slots:       []string{"成语接龙", "文字游戏"},
		Params: []ai.Param{
			{Name: "idiom", Type: "string", Description: "上一个成语", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			idiom := strings.TrimSpace(ctx.String("idiom"))
			if len([]rune(idiom)) < 3 {
				return "请给一个成语（至少3个字）", nil
			}
			cfg := config.C.AI
			client := ai.NewClient(cfg.DeepSeekKey, cfg.BaseURL)
			resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
				Model: cfg.Model,
				Messages: []openai.ChatCompletionMessage{
					{Role: openai.ChatMessageRoleSystem, Content: "你是成语接龙高手。规则：用对方成语的最后一个字（同音也可以）作为开头，接一个新成语。只回复一个成语（4个字），不要加任何解释。如果实在接不上就说「接不上了，你赢了！」"},
					{Role: openai.ChatMessageRoleUser, Content: idiom},
				},
				MaxTokens:   20,
				Temperature: 0.8,
			})
			if err != nil {
				return "接不上了，你赢了！", nil
			}
			if len(resp.Choices) == 0 {
				return "接不上了，你赢了！", nil
			}
			return strings.TrimSpace(resp.Choices[0].Message.Content), nil
		},
	})
}

// ── 匿名消息 ──────────────────────────────────────────────────────────────────

func registerAnonymousMessage() {
	prefixes := []string{"有人想说:", "匿名消息:", "有位群友想说:", "收到一条匿名消息:"}
	ai.Register(ai.ToolMeta{
		Name:        "anonymous_message",
		Description: "以月灵名义匿名转发一条消息到群里",
		Tags:        []string{"娱乐", "匿名"},
		Triggers:    []string{"匿名"},
		Patterns:    []string{`匿名(说|吐槽|表白)`},
		Slots:       []string{"匿名消息", "匿名留言"},
		Params: []ai.Param{
			{Name: "message", Type: "string", Description: "要匿名发送的内容", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			msg := strings.TrimSpace(ctx.String("message"))
			if len([]rune(msg)) < 2 {
				return "消息太短了", nil
			}
			if len([]rune(msg)) > 200 {
				return "消息太长了，200字以内", nil
			}
			text := fmt.Sprintf("%s\n%s", prefixes[rand.Intn(len(prefixes))], msg)
			ctx.BotAPI().SendGroupText(ctx.GroupID(), text)
			return "已匿名发送", nil
		},
	})
}

// ── Epic 免费游戏 ─────────────────────────────────────────────────────────────

func init() {
	registerEpicFreeGames()
}

func registerEpicFreeGames() {
	ai.Register(ai.ToolMeta{
		Name:        "epic_free_games",
		Description: "查询当前 Epic Games 商店的免费游戏",
		Tags:        []string{"游戏", "信息"},
		Triggers:    []string{"Epic", "免费游戏", "白嫖游戏"},
		Slots:       []string{"白嫖游戏", "Epic免费"},
		Params:      []ai.Param{},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			resp, err := httpClient.Get("https://store-site-backend-static-ipv4.ak.epicgames.com/freeGamesPromotions?locale=zh-CN&country=CN")
			if err != nil {
				return "查询失败：网络错误", nil
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			var data struct {
				Data struct {
					Catalog struct {
						SearchStore struct {
							Elements []struct {
								Title      string `json:"title"`
								Promotions *struct {
									PromotionalOffers []struct {
										PromotionalOffers []struct {
											DiscountSetting struct {
												DiscountPercentage int `json:"discountPercentage"`
											} `json:"discountSetting"`
										} `json:"promotionalOffers"`
									} `json:"promotionalOffers"`
								} `json:"promotions"`
							} `json:"elements"`
						} `json:"searchStore"`
					} `json:"Catalog"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &data); err != nil {
				return "解析失败", nil
			}

			var free []string
			for _, g := range data.Data.Catalog.SearchStore.Elements {
				if g.Promotions == nil {
					continue
				}
				for _, og := range g.Promotions.PromotionalOffers {
					for _, o := range og.PromotionalOffers {
						if o.DiscountSetting.DiscountPercentage == 0 {
							free = append(free, g.Title)
						}
					}
				}
			}
			if len(free) == 0 {
				return "当前没有免费游戏，请关注后续活动", nil
			}
			return "Epic 当前免费游戏:\n" + strings.Join(free, "\n"), nil
		},
	})
}

// ── 好感度排行 ────────────────────────────────────────────────────────────────

func registerAffinityRanking() {
	ai.Register(ai.ToolMeta{
		Name:        "affinity_ranking",
		Description: "显示当前群的好感度排行榜（基于积分）",
		Tags:        []string{"群聊", "娱乐"},
		Triggers:    []string{"好感", "排行"},
		Slots:       []string{"好感度", "关系排名", "谁最喜欢"},
		Params:      []ai.Param{},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			rows, err := db.GetTopScores(ctx.GroupID(), 10)
			if err != nil || len(rows) == 0 {
				return "暂无好感数据", nil
			}
			medals := []string{"🥇", "🥈", "🥉"}
			var sb strings.Builder
			sb.WriteString("好感度排行:\n")
			for i, r := range rows {
				name := r.Nickname
				if name == "" {
					name = fmt.Sprintf("%d", r.UserID)
				}
				prefix := fmt.Sprintf("%d.", i+1)
				if i < len(medals) {
					prefix = medals[i]
				}
				sb.WriteString(fmt.Sprintf("%s %s: %d\n", prefix, name, r.Score))
			}
			return strings.TrimRight(sb.String(), "\n"), nil
		},
	})
}

// ── 匿名消息需要 bytes ────────────────────────────────────────────────────────
var _ = bytes.NewReader // suppress unused import
