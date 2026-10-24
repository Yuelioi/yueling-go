package cards

import (
	"context"
	"fmt"
	"github.com/Yuelioi/yueling-go/services/wuxia"
	"image"
)

func Profile(ctx context.Context, v wuxia.View, day wuxia.Daily, limit int) ([]byte, error) {
	id := "ink-sprite-" + v.Fighter().Portrait
	for _, a := range []string{id, "menu-backdrop", "menu-frame", "ink-item-silver", weaponIcon(v.Weapon.Kind)} {
		if err := checkAsset(a); err != nil {
			return nil, err
		}
	}
	if err := loadEquipmentArt(); err != nil {
		return nil, err
	}
	return render(ctx, func(p *painter) {
		const height = 1000
		p.background(height)
		p.heading("侠客", v.Character.Silver)
		p.text(570, 59, 20, fmt.Sprintf("今日历练 %d/%d", day.Runs, limit), uiMuted, 370)
		p.atmosphere(height)
		p.text(60, 160, 18, "侠客录 / "+v.Profession.Name, uiAccent, 400)
		p.title(60, 213, 38, v.Character.Name, uiText, 610)
		p.text(62, 250, 20, fmt.Sprintf("%d级 · %s", v.Level, v.Title), uiMuted, 520)
		p.sprite(id, image.Rect(220, 260, 710, 868))
		p.text(62, 326, 18, "战力", uiMuted, 150)
		p.title(58, 387, 58, number(int64(v.Power)), uiText, 200)
		p.line(62, 416, 190, 416, uiEdge)
		for i, stat := range []struct {
			name  string
			value int
		}{{"气血", v.Stats.HP}, {"攻击", v.Stats.Attack}, {"防御", v.Stats.Defense}, {"身法", v.Stats.Speed}} {
			y := 462 + i*88
			p.text(62, y, 18, stat.name, uiMuted, 150)
			p.text(62, y+38, 30, fmt.Sprint(stat.value), uiText, 160)
		}
		grade, tint := qualityStyle(v.Weapon.Tier)
		p.line(734, 156, 734, 886, uiEdge)
		p.text(774, 160, 18, "随身兵刃", uiMuted, 340)
		p.title(774, 207, 36, v.Weapon.Name, uiText, 350)
		p.text(774, 244, 20, grade+" · "+v.Weapon.Kind+" · "+v.Weapon.Affinity, tint, 350)
		p.line(774, 263, 1126, 263, tint)
		artRect := image.Rect(774, 280, 1126, 566)
		if art := equipmentArt.images[v.Weapon.Name]; art != nil {
			p.containArt(art, artRect)
		} else {
			p.sprite(weaponIcon(v.Weapon.Kind), artRect)
		}

		for i, slot := range []struct {
			icon string
			item *wuxia.Item
		}{{"ink-item-armor", v.ArmorItem}, {"ink-item-jade", v.CharmItem}} {
			cx := 854 + i*188
			if slot.item != nil {
				if art := equipmentArt.images[slot.item.Name]; art != nil {
					p.containArt(art, image.Rect(cx-44, 570, cx+44, 658))
				} else {
					p.sprite(slot.icon, image.Rect(cx-44, 570, cx+44, 658))
				}
			} else {
				p.battleSkillIcon(image.Rect(cx-36, 580, cx+36, 652), nil)
			}
			name := "未佩戴"
			tint := uiMuted
			if slot.item != nil {
				name = slot.item.Name
				_, tint = qualityStyle(slot.item.Tier)
			}
			p.center(cx, 686, 20, name, tint, 174)
		}
		p.line(774, 709, 1126, 709, uiEdge)
		for i, sk := range []*wuxia.Skill{v.Skill, v.Mind} {
			cx := 854 + i*188
			p.skillIcon(image.Rect(cx-42, 730, cx+42, 814), sk)
			name := "未修习"
			tint := uiMuted
			if sk != nil {
				name = sk.Name
				_, tint = qualityStyle(sk.Tier)
			}
			p.center(cx, 845, 21, name, tint, 178)
		}
		if v.Synergy != "" {
			p.center(950, 882, 17, v.Synergy, uiAccent, 350)
		}

		base := wuxia.XPForLevel(v.Level)
		progress := float64(v.Character.Experience-base) / float64(max(int64(1), v.NextXP-base))
		label := fmt.Sprintf("阅历 %d / %d", v.Character.Experience-base, v.NextXP-base)
		if v.MaxLevel > 0 && v.Level >= v.MaxLevel {
			progress = 1
			label = "本期修行圆满"
		}
		p.text(62, 867, 18, label, uiMuted, 620)
		p.progress(image.Rect(62, 885, 688, 889), progress, uiSuccess)
		p.commandRibbon(height)
	}, 1000)
}
