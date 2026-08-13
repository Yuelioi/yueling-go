package tools

import (
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	gameplugin "github.com/Yuelioi/yueling-go/plugins/game"
)

func init() {
	ai.Register(ai.ToolMeta{
		Name:        "query_game_deals",
		Description: "查询 Epic 本周限免，或 Steam 国区当前价、折扣和参考史低",
		Tags:        []string{"游戏", "折扣"},
		Triggers:    []string{"Epic限免", "喜加一", "免费游戏", "史低", "Steam价格", "查游戏价格"},
		Patterns:    []string{`(查|看看).{0,8}(史低|游戏价格|限免)`, `steam.{0,12}(多少钱|折扣|史低)`},
		Slots:       []string{"Epic免费", "Steam查价", "游戏折扣"},
		PluginID:    catalog.PluginGameDeals,
		Params: []ai.Param{
			{Name: "action", Type: "string", Description: "查询类型", Required: true, Enum: []string{"epic_free", "steam_price"}},
			{Name: "query", Type: "string", Description: "steam_price 的游戏名、Steam链接或appid", Required: false},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			switch strings.TrimSpace(ctx.String("action")) {
			case "epic_free":
				result, err := gameplugin.QueryEpicDeals()
				if err != nil {
					return "Epic 限免读取失败，稍后再试", nil
				}
				return result, nil
			case "steam_price":
				result, err := gameplugin.QuerySteamDeal(ctx.String("query"))
				if err != nil {
					return "没查到这个游戏，试试 Steam 商店完整名称或 appid", nil
				}
				return result, nil
			}
			return "未知查询类型", nil
		},
	})
}
