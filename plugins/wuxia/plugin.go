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
	b.OnCommand("江湖赌场帮助").Plugin(catalog.PluginWuxia).Priority(60).Block().Handle(func(c *bot.CommandContext) error { return replyCard(c, game.CasinoRules) })
	for _, spec := range []struct {
		name, action string
		aliases      []string
	}{
		{"江湖", "daily", nil}, {"江湖历练", "adventure", []string{"历练"}}, {"江湖练武", "train", nil},
		{"江湖赌场", "casino", []string{"赌场"}}, {"江湖寻宝", "draw", []string{"江湖抽奖"}}, {"江湖装备", "equip", nil}, {"江湖学艺", "learn", nil}, {"江湖武技", "skill", []string{"江湖武学"}}, {"江湖心法", "mind", nil},
		{"江湖转职", "profession", nil}, {"江湖武器", "weapon", nil}, {"江湖形象", "portrait", nil}, {"切磋", "duel", []string{"江湖切磋"}},
	} {
		spec := spec
		b.OnCommand(spec.name, spec.aliases...).Plugin(catalog.PluginWuxia).Priority(60).Block().Handle(func(c *bot.CommandContext) error {
			if (spec.action == "weapon" || spec.action == "profession" || spec.action == "portrait" || spec.action == "equip" || spec.action == "learn" || spec.action == "skill" || spec.action == "mind") && len(c.Args) == 0 {
				return showOptions(c, service, spec.action)
			}
			argument := strings.TrimSpace(strings.Join(c.Args, " "))
			if spec.action == "casino" && argument == "帮助" {
				return replyCard(c, game.CasinoRules)
			}
			if spec.action != "casino" && spec.action != "duel" && spec.action != "weapon" && spec.action != "profession" && spec.action != "portrait" && spec.action != "draw" && spec.action != "equip" && spec.action != "learn" && spec.action != "skill" && spec.action != "mind" && argument != "" {
				return replyCard(c, "该命令无需参数；玩法见「江湖帮助」")
			}
			target := int64(0)
			targetName := ""
			if spec.action == "duel" {
				var err error
				target, err = duelTarget(c)
				if err != nil {
					return replyCard(c, err.Error())
				}
				member, err := c.GetGroupMemberInfo(c.GroupID(), target)
				if err != nil || member == nil || member.UserID != target {
					return replyCard(c, "暂时无法获取这位群友的信息，请稍后重试切磋")
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

		var pages [][]byte
		for _, pool := range service.Catalog().Pools {
			entries := service.Catalog().EligibleEntries(pool, v.Profession.ID, v.Level)
			total := 0
			for _, e := range entries {
				total += e.Weight
			}
			rows := []cards.ListEntry{}
			for _, e := range entries {
				item, _ := service.Catalog().Item(e.Reward.ItemID)
				chance := 0.0
				if total > 0 {
					chance = 100 * float64(e.Weight) / float64(total)
				}
				rows = append(rows, cards.ListEntry{Name: item.Name, Detail: item.Description, Badge: fmt.Sprintf("×%d · %.1f%%", e.Reward.Quantity, chance), ArtName: item.Name, SkillID: item.SkillID, Tier: item.Tier})
			}
			currency := "银两"
			if pool.CostItem != "silver" {
				item, _ := service.Catalog().Item(pool.CostItem)
				currency = item.Name
			}
			images, err := cards.Collection(c.Context(), pool.Name, fmt.Sprintf("每次%d%s · %s当前可得奖励 · 江湖寻宝", pool.Cost, currency, v.Profession.Name), rows)
			if err != nil {
				return replyError(c, err)
			}
			pages = append(pages, images...)
		}
		if len(pages) == 0 {
			return replyCard(c, "暂无可用奖池")
		}
		return sendPages(c, pages)
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
			return replyCard(c, "本群江湖尚未开张，发送「江湖」即可加入")
		}
		entries := []cards.ListEntry{}
		for i, v := range rows {
			entries = append(entries, cards.ListEntry{Name: fmt.Sprintf("%d · %s", i+1, v.Character.Name), Detail: v.Profession.Name, Badge: fmt.Sprintf("战力 %d", v.Power), Portrait: v.Profession.Portrait})
		}
		return replyCollection(c, "群侠榜", "本群战力排行", entries)
	})
	b.OnCommand("江湖帮助").Plugin(catalog.PluginWuxia).Priority(60).Block().Handle(func(c *bot.CommandContext) error {
		return replyCard(c, "月灵江湖 · 先发「江湖」即可开玩\n江湖：一键建角、连续历练三次、升级、换装\n我的侠客 / 江湖榜\n切磋 @群友：双方自动建角，兵刃对招，招架、卸力、破防，自动生成战报\n江湖练武：旧基础属性养成，每重增加1.5%，不是升级武技\n江湖转职 [职业名]：免费换职业，保留成长\n江湖武器 [剑|刀|拳套|武器名]\n江湖形象 [编号或名称，0跟随职业]\n江湖背包 / 江湖装备 物品名\n江湖学艺 秘籍名 / 江湖武学 武学名\n江湖奖池 / 江湖寻宝 [奖池名] [1或5]\n江湖赌场 [赌金]：默认100、最低20，直接开奖，豹子最高10倍；江湖赌场帮助查看规则\n江湖战报：重看最近一次结果，不重复领奖\n江湖历练：一次完成今日剩余三次历练；每日首次主动切磋奖励100银两、20阅历，不论胜负\n本期等级上限10级；寻宝按职业与当前等级筛选。每日零点重置日常；各群成长独立。图片中的指令请直接发送到群聊。")
	})
}

func sendResult(c *bot.CommandContext, ctx context.Context, r game.Result) error {
	data, err := cards.Report(ctx, r)
	if err != nil {
		logx.Warnf("[wuxia] render group=%d user=%d: %v", c.GroupID(), c.UserID(), err)
		return c.Reply(cards.Summary(r) + "\n结果已保存，可发送「江湖战报」重新查看。")
	}
	message := bot.Msg().Reply(c.MessageID())
	if (r.Joined || r.TargetJoined) && !r.Replayed {
		text := "欢迎入江湖，角色已创建。发「江湖转职」选职业，「我的侠客」看装备；每日历练由本人领取。"
		pages, e := cards.Notice(ctx, "初入江湖", text)
		if e == nil {
			for _, page := range pages {
				message.ImageBytes(page)
			}
		}
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
		return replyCard(c, err.Error())
	}
	logx.Errorf("[wuxia] group=%d user=%d: %v", c.GroupID(), c.UserID(), err)
	return replyCard(c, "江湖数据处理失败，请稍后重试；已结算的同一条消息不会重复领奖。")
}
func showOptions(c *bot.CommandContext, s *game.Service, action string) error {
	if action == "equip" || action == "learn" || action == "skill" || action == "mind" {
		return showInventory(c, s, action)
	}
	entries := []cards.ListEntry{}
	if action == "profession" {
		for _, p := range s.Catalog().Professions {
			entries = append(entries, cards.ListEntry{Name: p.Name, Portrait: p.Portrait, Detail: fmt.Sprintf("%s；每级气血+%d / 攻击+%d / 防御+%d / 身法+%d", p.Description, p.PerLevel.HP, p.PerLevel.Attack, p.PerLevel.Defense, p.PerLevel.Speed)})
		}
		return replyCollection(c, "江湖流派", "江湖转职 职业名 · 免费切换，保留成长", entries)
	}
	if action == "portrait" {
		for i, a := range s.Catalog().Appearances {
			entries = append(entries, cards.ListEntry{Name: a.Name, Badge: fmt.Sprintf("编号 %d", i+1), Portrait: a.ID})
		}
		return replyCollection(c, "侠客风貌", "江湖形象 编号或名称 · 0跟随职业 · 外观不改变属性", entries)
	}
	v, _, err := s.Profile(c.Context(), c.GroupID(), c.UserID())
	if err != nil {
		return replyError(c, err)
	}
	for _, w := range s.Catalog().Weapons {
		if v.Character.Inventory[w.ID] > 0 {
			badge := "已收藏"
			if v.Weapon.ID == w.ID {
				badge = "装备中"
			}
			entries = append(entries, cards.ListEntry{Name: w.Name, ArtName: w.Name, Tier: w.Tier, Detail: w.Kind, Badge: badge})
		}
	}
	return replyCollection(c, "兵器收藏", "江湖武器 名称 · 装备本职业兵器", entries)
}

func showInventory(c *bot.CommandContext, s *game.Service, action string) error {
	v, _, err := s.Profile(c.Context(), c.GroupID(), c.UserID())
	if err != nil {
		return replyError(c, err)
	}
	catalog := s.Catalog()
	entries := []cards.ListEntry{}
	title, hint := "江湖行囊", "江湖装备 物品名 · 江湖学艺 秘籍名"
	if action == "skill" || action == "mind" {
		title, hint = "所学武技", "江湖武技 名称 · 江湖武技 卸下"
		if action == "mind" {
			title, hint = "修习心法", "江湖心法 名称 · 江湖心法 卸下"
		}
		for _, sk := range catalog.Skills {
			if !v.Character.LearnedSkills[sk.ID] || (sk.Kind == "mind") != (action == "mind") {
				continue
			}
			badge := "已学"
			if (v.Skill != nil && v.Skill.ID == sk.ID) || (v.Mind != nil && v.Mind.ID == sk.ID) {
				badge = "使用中"
			}
			entries = append(entries, cards.ListEntry{Name: sk.Name, Detail: sk.RuleText(), Badge: badge, SkillID: sk.ID, Tier: sk.Tier})
		}
	} else {
		ids := []string{}
		for id, n := range v.Character.Inventory {
			if n > 0 {
				ids = append(ids, id)
			}
		}
		sort.Slice(ids, func(i, j int) bool {
			a, _ := catalog.Item(ids[i])
			b, _ := catalog.Item(ids[j])
			if a.Tier != b.Tier {
				return a.Tier > b.Tier
			}
			if a.Kind != b.Kind {
				return a.Kind < b.Kind
			}
			return a.Name < b.Name
		})
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
			badge := fmt.Sprintf("×%d", v.Character.Inventory[id])
			if v.Weapon.ID == id || v.Character.Equipment[item.Kind] == id {
				badge += " · 装备中"
			}
			entries = append(entries, cards.ListEntry{Name: item.Name, Detail: fmt.Sprintf("%s · 重复折银%d", item.Description, game.ItemPrice(item)*40/100), Badge: badge, ArtName: item.Name, SkillID: item.SkillID, Tier: item.Tier})
		}
	}
	pages, err := cards.Collection(c.Context(), title, hint, entries)
	if err != nil {
		return c.Reply("行囊图片暂时无法绘制，请稍后重试。")
	}
	return sendPages(c, pages)
}

func sendPages(c *bot.CommandContext, pages [][]byte) error {
	message := bot.Msg().Reply(c.MessageID())
	for _, page := range pages {
		message.ImageBytes(page)
	}
	return c.SendMsg(message.Build())
}

func replyCard(c *bot.CommandContext, text string) error {
	pages, err := cards.Notice(c.Context(), "江湖手帖", text)
	if err != nil {
		logx.Warnf("[wuxia] notice render: %v", err)
		return c.Reply(text)
	}
	return sendPages(c, pages)
}

func replyCollection(c *bot.CommandContext, title, hint string, entries []cards.ListEntry) error {
	pages, err := cards.Collection(c.Context(), title, hint, entries)
	if err != nil {
		return replyError(c, err)
	}
	return sendPages(c, pages)
}
