package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
)

func init() {
	registerHoroscope()
	registerDailyFortune()
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
		PluginID:    catalog.PluginFortune,
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
		PluginID:    catalog.PluginFortune,
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
