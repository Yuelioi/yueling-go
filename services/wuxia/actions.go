package wuxia

import (
	"fmt"
	"github.com/Yuelioi/yueling-go/services/wuxia/internal/progression"
	"math/rand/v2"
	"strconv"
	"strings"
)

type gameAction struct {
	command      Command
	catalog      Catalog
	player       *Character
	request      Request
	result       *Result
	before       View
	settings     Settings
	daily        *Daily
	rng          *rand.Rand
	target       Character
	targetJoined bool
}

var actionHandlers = map[string]func(*gameAction) error{
	"daily":      handleAdventure,
	"adventure":  handleAdventure,
	"train":      handleTrain,
	"profession": handleProfession,
	"weapon":     handleWeapon,
	"equip":      handleEquip,
	"learn":      handleLearn,
	"skill":      handleSkill,
	"mind":       handleSkill,
	"draw":       handleDraw,
	"casino":     handleCasino,
	"portrait":   handlePortrait,
	"duel":       handleDuel,
}

func handleAdventure(g *gameAction) error {
	p, result := g.player, g.result
	d := g.daily
	rng := g.rng
	settings := g.settings
	n := max(0, 3-d.Runs)
	for range n {
		r, err := g.catalog.Explore(p, settings, rng.Uint64())
		if err != nil {
			return err
		}
		result.Encounters = append(result.Encounters, r)
		d.Runs++
	}
	if n == 0 {
		result.Lines = append(result.Lines, "今日有奖励历练已完成")
	}
	return nil
}

func handleTrain(g *gameAction) error {
	p, result := g.player, g.result
	before := g.before
	if p.Mastery >= before.Level {
		return rulef("武学已达当前等级上限，历练升级后可继续")
	}
	if p.Silver < before.PracticeCost {
		return rulef("练武需%d银两，当前%d", before.PracticeCost, p.Silver)
	}
	p.Silver -= before.PracticeCost
	p.Mastery++
	result.Lines = append(result.Lines, fmt.Sprintf("练武成功，武学%d重，花费%d银两", p.Mastery, before.PracticeCost))
	return nil
}

func handleProfession(g *gameAction) error {
	p, req, result := g.player, g.request, g.result
	prof, ok := g.catalog.Profession(req.Argument)
	if !ok {
		return rulef("发送「江湖转职」查看可选职业")
	}
	p.ProfessionID = prof.ID
	p.WeaponKind = prof.WeaponKind
	if sk, ok := g.catalog.Skill(p.ActiveSkill); ok && sk.WeaponKind != p.WeaponKind {
		p.ActiveSkill = ""
	}
	eligibleWeapon := false
	level := progression.Level(p.Experience, g.catalog.MaxLevel)
	for id, n := range p.Inventory {
		item, ok := g.catalog.Item(id)
		if ok && n > 0 && item.Kind == "weapon" && g.catalog.Allowed(item, prof.ID, level) {
			eligibleWeapon = true
			break
		}
	}
	if !eligibleWeapon {
		if err := g.catalog.Grant(p, []Reward{{ItemID: g.catalog.StarterFor(prof.ID).ID, Quantity: 1}}); err != nil {
			return err
		}
	}
	for slot, id := range p.Equipment {
		item, ok := g.catalog.Item(id)
		if !ok || !g.catalog.Allowed(item, prof.ID, level) {
			delete(p.Equipment, slot)
		}
	}
	g.catalog.AutoEquip(p)
	result.Lines = append(result.Lines, "已转为"+prof.Name+"，等级、银两和已收集武器保留")
	return nil
}

func handleWeapon(g *gameAction) error {
	p, req, result := g.player, g.request, g.result
	kind := strings.TrimSpace(req.Argument)
	w, byName := g.catalog.Weapon(kind)
	if byName {
		if p.Inventory[w.ID] < 1 {
			return rulef("尚未收集这件武器")
		}
		if err := g.catalog.Equip(p, w.ID); err != nil {
			return err
		}
		kind = w.Kind
	}
	if _, ok := p.Weapons[kind]; !ok {
		return rulef("尚未收集该类武器，请查看「江湖武器」")
	}
	if err := g.catalog.Equip(p, p.Weapons[kind]); err != nil {
		return err
	}
	selected, _ := g.catalog.Weapon(p.Weapons[kind])
	result.Lines = append(result.Lines, "已装备"+selected.Name)
	return nil
}

func handleEquip(g *gameAction) error {
	p, req, result := g.player, g.request, g.result
	if err := g.catalog.Equip(p, req.Argument); err != nil {
		return err
	}
	item, _ := g.catalog.Item(req.Argument)
	result.Lines = append(result.Lines, "已装备"+item.Name)
	return nil
}

func handleLearn(g *gameAction) error {
	p, req, result := g.player, g.request, g.result
	if err := g.catalog.Learn(p, req.Argument); err != nil {
		return err
	}
	item, _ := g.catalog.Item(req.Argument)
	skill, _ := g.catalog.Skill(item.SkillID)
	result.Lines = append(result.Lines, "学会「"+skill.Name+"」")
	return nil
}

func handleSkill(g *gameAction) error {
	p, req, result := g.player, g.request, g.result
	slot := &p.ActiveSkill
	kind := "武技"
	if req.Action == "mind" {
		slot = &p.ActiveMind
		kind = "心法"
	}
	if req.Argument == "卸下" {
		*slot = ""
		result.Lines = append(result.Lines, "已卸下"+kind)
		return nil
	}
	skill, ok := g.catalog.Skill(req.Argument)
	if !ok || !p.LearnedSkills[skill.ID] {
		return rulef("尚未学会这门武学")
	}
	if progression.Level(p.Experience, g.catalog.MaxLevel) < skill.MinLevel {
		return rulef("这门武学需%d级", skill.MinLevel)
	}
	if (skill.Kind == "mind") != (req.Action == "mind") {
		return rulef("请选择%s", kind)
	}
	if skill.Kind != "mind" && skill.WeaponKind != p.WeaponKind {
		return rulef("这门武学需装备%s", skill.WeaponKind)
	}
	*slot = skill.ID
	result.Lines = append(result.Lines, "已装备"+kind+"「"+skill.Name+"」")
	return nil
}

func handleDraw(g *gameAction) error {
	p, result := g.player, g.result
	rng := g.rng
	cmd, ok := g.command.(DrawCommand)
	if !ok {
		return rulef("寻宝命令类型无效")
	}
	poolID, count := cmd.PoolID, cmd.Count
	result.RewardSeed = rng.Uint64()
	rewards, err := g.catalog.Draw(p, poolID, count, result.RewardSeed)
	if err != nil {
		return err
	}
	g.catalog.AutoEquip(p)
	for _, reward := range rewards {
		item, _ := g.catalog.Item(reward.ItemID)
		if item.Kind == "manual" && !p.LearnedSkills[item.SkillID] {
			sk, _ := g.catalog.Skill(item.SkillID)
			found := false
			for _, n := range result.LearnedNames {
				if n == sk.Name {
					found = true
				}
			}
			if !found {
				result.LearnedNames = append(result.LearnedNames, sk.Name)
			}
		}
	}
	if err := g.catalog.LearnRewards(p, rewards); err != nil {
		return err
	}
	result.Rewards = rewards
	pool, _ := g.catalog.Pool(poolID)
	result.DrawPool = pool.ID
	result.DrawPoolName = pool.Name
	result.Lines = append(result.Lines, pool.Name+" · 寻宝所得")
	for _, reward := range rewards {
		item, _ := g.catalog.Item(reward.ItemID)
		result.RewardItems = append(result.RewardItems, item)
		result.Lines = append(result.Lines, fmt.Sprintf("%s ×%d", item.Name, reward.Quantity))
	}
	return nil
}

func handlePortrait(g *gameAction) error {
	p, req, result := g.player, g.request, g.result
	id := strings.TrimSpace(req.Argument)
	if id == "0" {
		p.Portrait = ""
	} else {
		selected := ""
		for i, a := range g.catalog.Appearances {
			if id == a.ID || id == a.Name || id == strconv.Itoa(i+1) {
				selected = a.ID
				break
			}
		}
		if selected == "" {
			return rulef("发送「江湖形象」查看可用形象")
		}
		p.Portrait = selected
	}
	result.Lines = append(result.Lines, "侠客形象已更新，仅改变外观")
	return nil
}

func handleDuel(g *gameAction) error {
	result := g.result
	before := g.before
	rng := g.rng
	settings := g.settings
	target := g.target
	targetJoined := g.targetJoined
	if targetJoined {
		result.Lines = append(result.Lines, "已为"+target.Name+"创建初始侠客，历练奖励由本人领取")
	}
	other, err := g.catalog.View(target, settings)
	if err != nil {
		return err
	}
	b := Duel(before.Fighter(), other.Fighter(), rng.Uint64())
	result.Battle = &b
	if !g.daily.DuelRewarded {
		const silver = 100
		gained := progression.Gain(g.player.Experience, 20, g.catalog.MaxLevel)
		g.player.Silver += silver
		g.player.Experience += gained
		g.daily.DuelRewarded = true
		result.Lines = append(result.Lines, fmt.Sprintf("今日首战 +%d银两 · +%d阅历", silver, gained))
	}
	result.Lines = append(result.Lines, "双方收势抱拳，以武会友")
	return nil
}
