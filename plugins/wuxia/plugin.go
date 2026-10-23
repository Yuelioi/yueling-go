package wuxia

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services/logx"
	game "github.com/Yuelioi/yueling-go/services/wuxia"
	"github.com/Yuelioi/yueling-go/services/wuxia/cards"
)

func Register(b *bot.Bot, service *game.Service) {
	b.OnCommand("江湖赌场帮助").Plugin(catalog.PluginWuxia).Priority(60).Block().Handle(func(c *bot.CommandContext) error { return c.Reply(game.CasinoRules) })
	for _, spec := range []struct {
		name, action string
		aliases      []string
	}{
		{"江湖", "daily", nil}, {"江湖签到", "sign", nil}, {"江湖历练", "adventure", []string{"历练"}}, {"江湖练武", "train", nil},
		{"江湖赌场", "casino", []string{"赌场"}}, {"江湖寻宝", "draw", []string{"江湖抽奖"}}, {"江湖装备", "equip", nil}, {"江湖学艺", "learn", nil}, {"江湖武学", "skill", nil},
		{"江湖转职", "profession", nil}, {"江湖武器", "weapon", nil}, {"江湖形象", "portrait", nil}, {"切磋", "duel", []string{"江湖切磋"}},
	} {
		spec := spec
		b.OnCommand(spec.name, spec.aliases...).Plugin(catalog.PluginWuxia).Priority(60).Block().Handle(func(c *bot.CommandContext) error {
			if (spec.action == "weapon" || spec.action == "profession" || spec.action == "portrait" || spec.action == "equip" || spec.action == "learn" || spec.action == "skill") && len(c.Args) == 0 {
				return showOptions(c, service, spec.action)
			}
			argument := strings.TrimSpace(strings.Join(c.Args, " "))
			if spec.action == "casino" && argument == "帮助" {
				return c.Reply(game.CasinoRules)
			}
			if spec.action != "casino" && spec.action != "duel" && spec.action != "weapon" && spec.action != "profession" && spec.action != "portrait" && spec.action != "draw" && spec.action != "equip" && spec.action != "learn" && spec.action != "skill" && argument != "" {
				return c.Reply("该命令无需参数；玩法见「江湖帮助」")
			}
			target := int64(0)
			targetName := ""
			if spec.action == "duel" {
				var err error
				target, err = duelTarget(c)
				if err != nil {
					return c.Reply(err.Error())
				}
				member, err := c.GetGroupMemberInfo(c.GroupID(), target)
				if err != nil || member == nil || member.UserID != target {
					return c.Reply("暂时无法获取这位群友的信息，请稍后重试切磋")
				}
				targetName = strings.TrimSpace(member.Card)
				if targetName == "" {
					targetName = member.Nickname
				}
			}
			ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
			defer cancel()
			command, err := game.ParseCommand(spec.action, argument, target)
			if err != nil {
				return replyError(c, err)
			}
			r, err := service.Execute(ctx, game.Envelope{BotID: c.Event.SelfID, GroupID: c.GroupID(), UserID: c.UserID(), MessageID: c.MessageID(), Name: c.Nickname(), TargetName: targetName}, command)
			if err != nil {
				return replyError(c, err)
			}
			return sendResult(c, ctx, r)
		})
	}
	b.OnCommand("江湖背包").Plugin(catalog.PluginWuxia).Priority(60).Block().Handle(func(c *bot.CommandContext) error { return showInventory(c, service, "bag") })
	b.OnCommand("江湖奖池").Plugin(catalog.PluginWuxia).Priority(60).Block().Handle(func(c *bot.CommandContext) error {
		v, _, err := service.Profile(c.Context(), c.GroupID(), c.UserID())
		if err != nil {
			return replyError(c, err)
		}

		var text strings.Builder
		for _, pool := range service.Catalog().Pools {
			currency := "银两"
			if pool.CostItem != "silver" {
				item, _ := service.Catalog().Item(pool.CostItem)
				currency = item.Name
			}
			fmt.Fprintf(&text, "%s · 每次%d%s\n", pool.Name, pool.Cost, currency)
			total := 0
			for _, entry := range service.Catalog().EligibleEntries(pool, v.Profession.ID, v.Level) {
				total += entry.Weight
			}
			for _, entry := range service.Catalog().EligibleEntries(pool, v.Profession.ID, v.Level) {
				item, _ := service.Catalog().Item(entry.Reward.ItemID)
				fmt.Fprintf(&text, "%s ×%d · %.1f%%\n", item.Name, entry.Reward.Quantity, 100*float64(entry.Weight)/float64(total))
			}
		}
		fmt.Fprintf(&text, "以上为%s · %d级的实际奖励概率。\n", v.Profession.Name, v.Level)
		text.WriteString("江湖寻宝：抽一次；江湖寻宝 5：抽五次；可加奖池名。重复物品收入背包，更高阶可用装备自动换上。")
		return c.Reply(text.String())
	})
	b.OnCommand("江湖战报").Plugin(catalog.PluginWuxia).Priority(60).Block().Handle(func(c *bot.CommandContext) error {
		ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
		defer cancel()
		r, err := service.LatestResult(ctx, c.Event.SelfID, c.GroupID(), c.UserID())
		if err != nil {
			return replyError(c, err)
		}
		return sendResult(c, ctx, r)
	})
	b.OnCommand("我的侠客").Plugin(catalog.PluginWuxia).Priority(60).Block().Handle(func(c *bot.CommandContext) error {
		v, d, err := service.Profile(c.Context(), c.GroupID(), c.UserID())
		if err != nil {
			return replyError(c, err)
		}
		settings, err := service.Settings(c.Context(), c.GroupID())
		if err != nil {
			return replyError(c, err)
		}
		data, err := cards.Profile(c.Context(), v, d, settings.DailyRuns)
		if err != nil {
			return c.Reply(fmt.Sprintf("%s · %s · 等级%d · 战力%d\n银两%d · 兵刃%s\n角色卡绘制失败，属性与资产已保留。", v.Character.Name, v.Profession.Name, v.Level, v.Power, v.Character.Silver, v.Weapon.Name))
		}
		return c.SendMsg(bot.Msg().Reply(c.MessageID()).ImageBytes(data).Build())
	})
	b.OnCommand("江湖榜").Plugin(catalog.PluginWuxia).Priority(60).Block().Handle(func(c *bot.CommandContext) error {
		rows, err := service.Ranking(c.Context(), c.GroupID(), 10)
		if err != nil {
			return replyError(c, err)
		}
		if len(rows) == 0 {
			return c.Reply("本群江湖尚未开张，发送「江湖」即可加入")
		}
		var text strings.Builder
		text.WriteString("本群江湖战力榜\n")
		for i, v := range rows {
			fmt.Fprintf(&text, "%d. %s · %s · %d战力\n", i+1, v.Character.Name, v.Profession.Name, v.Power)
		}
		return c.Reply(strings.TrimSpace(text.String()))
	})
	b.OnCommand("江湖帮助").Plugin(catalog.PluginWuxia).Priority(60).Block().Handle(func(c *bot.CommandContext) error {
		return c.Reply("月灵江湖 · 先发「江湖」即可开玩\n江湖：一键建角、签到、历练、升级、换装\n我的侠客 / 江湖榜\n切磋 @群友：双方自动建角，兵刃对招，招架、卸力、破防，自动生成战报\n江湖练武：按显示价格提升一次武学\n江湖转职 [职业名]：免费换职业，保留成长\n江湖武器 [剑|刀|拳套|武器名]\n江湖形象 [编号或名称，0跟随职业]\n江湖背包 / 江湖装备 物品名\n江湖学艺 秘籍名 / 江湖武学 武学名\n江湖奖池 / 江湖寻宝 [奖池名] [1或5]\n江湖赌场 [赌金]：默认20、最低20，直接开奖，豹子最高10倍；江湖赌场帮助查看规则\n江湖战报：重看最近一次结果，不重复领奖\n江湖签到 / 江湖历练：仅做单项日常\n本期等级上限10级；寻宝按职业与当前等级筛选。每日零点重置日常；各群成长独立。图片中的指令请直接发送到群聊。")
	})
}

func sendResult(c *bot.CommandContext, ctx context.Context, r game.Result) error {
	data, err := cards.Report(ctx, r)
	if err != nil {
		logx.Warnf("[wuxia] render group=%d user=%d: %v", c.GroupID(), c.UserID(), err)
		return c.Reply(cards.Summary(r) + "\n结果已保存，可发送「江湖战报」重新查看。")
	}
	message := bot.Msg().Reply(c.MessageID())
	if r.Joined && !r.Replayed {
		message.Text("欢迎入江湖！已自动创建侠客。\n发「我的侠客」看属性；发「江湖转职」可免费选职业。图中指令直接发到群聊即可。\n")
	}
	if r.TargetJoined && !r.Replayed {
		message.Text("已为" + r.Battle.Right.Name + "创建初始侠客；签到和历练奖励留给本人领取。\n")
	}
	return c.SendMsg(message.ImageBytes(data).Build())
}
func duelTarget(c *bot.CommandContext) (int64, error) {
	var targets []int64
	for _, raw := range c.Message().AtTargets() {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && id > 0 && id != c.Event.SelfID {
			targets = append(targets, id)
		}
	}
	if len(targets) != 1 || targets[0] == c.UserID() {
		return 0, fmt.Errorf("用法：切磋 @另一位群友")
	}
	return targets[0], nil
}
func replyError(c *bot.CommandContext, err error) error {
	var rule *game.RuleError
	if errors.As(err, &rule) || errors.Is(err, game.ErrNotJoined) {
		return c.Reply(err.Error())
	}
	logx.Errorf("[wuxia] group=%d user=%d: %v", c.GroupID(), c.UserID(), err)
	return c.Reply("江湖数据处理失败，请稍后重试；已结算的同一条消息不会重复领奖。")
}
func showOptions(c *bot.CommandContext, s *game.Service, action string) error {
	if action == "equip" || action == "learn" || action == "skill" {
		return showInventory(c, s, action)
	}
	if action == "profession" {
		var text strings.Builder
		for _, p := range s.Catalog().Professions {
			fmt.Fprintf(&text, "%s：%s\n专属%s；每级 气血+%d / 攻击+%d / 防御+%d / 身法+%d\n", p.Name, p.Description, p.WeaponKind, p.PerLevel.HP, p.PerLevel.Attack, p.PerLevel.Defense, p.PerLevel.Speed)
		}
		return c.Reply(text.String() + "发送「江湖转职 职业名」即可免费切换")
	}
	if action == "portrait" {
		var text strings.Builder
		for i, a := range s.Catalog().Appearances {
			fmt.Fprintf(&text, "%d：%s\n", i+1, a.Name)
		}
		return c.Reply(text.String() + "江湖形象 编号或名称：切换外观；0跟随职业。外观不改变属性。")
	}
	v, _, err := s.Profile(c.Context(), c.GroupID(), c.UserID())
	if err != nil {
		return replyError(c, err)
	}
	var text strings.Builder
	text.WriteString("已收集兵刃\n")
	for _, w := range s.Catalog().Weapons {
		if v.Character.Inventory[w.ID] > 0 {
			fmt.Fprintf(&text, "%s · %s · 攻击+%d\n", w.Kind, w.Name, w.Attack)
		}
	}
	return c.Reply(text.String() + "发送「江湖武器 类型或名字」装备；只能装备当前职业专属且等级满足的兵刃")
}

func showInventory(c *bot.CommandContext, s *game.Service, action string) error {
	v, _, err := s.Profile(c.Context(), c.GroupID(), c.UserID())
	if err != nil {
		return replyError(c, err)
	}
	var text strings.Builder
	catalog := s.Catalog()
	if action == "skill" {
		text.WriteString("已学武学\n")
		for _, skill := range catalog.Skills {
			if v.Character.LearnedSkills[skill.ID] {
				mark := ""
				if v.Skill != nil && v.Skill.ID == skill.ID {
					mark = " · 使用中"
				}
				fmt.Fprintf(&text, "%s · %s%s\n", skill.Name, skill.WeaponKind, mark)
			}
		}
		text.WriteString("江湖武学 武学名：装备；江湖武学 卸下。秘籍用「江湖学艺 秘籍名」学习。")
	} else {
		text.WriteString("行囊\n")
		ids := make([]string, 0, len(v.Character.Inventory))
		for id, n := range v.Character.Inventory {
			if n > 0 {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		for _, id := range ids {
			item, ok := catalog.Item(id)
			if !ok {
				continue
			}
			if action == "learn" && item.Kind != "manual" {
				continue
			}
			if action == "equip" && item.Kind != "weapon" && item.Kind != "armor" && item.Kind != "charm" {
				continue
			}
			fmt.Fprintf(&text, "%s ×%d · %d级可用\n", item.Name, v.Character.Inventory[id], item.MinLevel)
		}
		text.WriteString("江湖装备 物品名：换装；江湖学艺 秘籍名：学武；江湖奖池：查看寻宝奖励。")
	}
	return c.Reply(text.String())
}
