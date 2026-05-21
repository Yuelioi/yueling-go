package game

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/httpclient"
)

const jw3UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

func RegisterJW3(b *bot.Bot) {
	b.OnCommand("物价").Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("用法：/物价 <商品名>")
		}
		keyword := strings.Join(ctx.Args, " ")
		result, err := jw3Query(keyword)
		if err != nil || result == "" {
			return ctx.Reply("未找到相关商品价格信息")
		}
		return ctx.Reply(result)
	})
}

func jw3Get(rawURL string, params map[string]string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return httpclient.Direct.GetBytes(u.String(), "User-Agent", jw3UA, "Accept", "application/json")
}

func jw3Appearance(keyword string) []string {
	body, err := jw3Get(
		"https://trade-api.seasunwbl.com/api/platform/setting/goods_appearance_name_search",
		map[string]string{"game_id": "jx3", "keyword": keyword},
	)
	if err != nil {
		return nil
	}
	var data struct {
		Data struct {
			List []struct {
				Name string `json:"name"`
			} `json:"list"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &data) != nil {
		return nil
	}
	var names []string
	for _, item := range data.Data.List {
		if item.Name != "" {
			names = append(names, item.Name)
			if len(names) >= 5 {
				break
			}
		}
	}
	return names
}

func jw3Price(keyword string) string {
	body, err := jw3Get(
		"https://trade-api.seasunwbl.com/m_api/buyer/goods/list",
		map[string]string{
			"goods_type":              "3",
			"game":                    "jx3",
			"game_id":                 "jx3",
			"sort[price]":             "1",
			"filter[state]":           "0",
			"filter[appearance_type]": "",
			"filter[role_appearance]": "",
			"filter[price]":           "0",
			"size":                    "10",
			"page":                    "1",
			"keyword":                 keyword,
		},
	)
	if err != nil {
		return ""
	}
	var data struct {
		Data struct {
			List []struct {
				Info            string  `json:"info"`
				SingleUnitPrice float64 `json:"single_unit_price"`
			} `json:"list"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &data) != nil || len(data.Data.List) == 0 {
		return ""
	}
	prices := data.Data.List
	if len(prices) > 3 {
		prices = prices[:3]
	}
	var parts []string
	for _, p := range prices {
		parts = append(parts, fmt.Sprintf("%.2f金", p.SingleUnitPrice/100))
	}
	name := prices[0].Info
	if name == "" {
		name = keyword
	}
	return fmt.Sprintf("%s\n参考价: %s", name, strings.Join(parts, " / "))
}

func jw3Query(keyword string) (string, error) {
	names := jw3Appearance(keyword)
	if len(names) == 0 {
		result := jw3Price(keyword)
		return result, nil
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("「%s」相关物价:", keyword))
	for _, name := range names {
		if p := jw3Price(name); p != "" {
			lines = append(lines, p)
		}
		if len(lines) >= 4 {
			break
		}
	}
	if len(lines) <= 1 {
		return "", nil
	}
	return strings.Join(lines, "\n\n"), nil
}
