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
		signed := "未签到"
		if day.Signed {
			signed = "已签到"
		}
		p.text(570, 59, 20, fmt.Sprintf("历练 %d/%d  ·  %s", day.Runs, limit, signed), uiMuted, 370)
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
		p.line(734, 156, 734, 826, uiEdge)
		p.text(774, 160, 18, "随身兵刃", uiMuted, 340)
		p.title(774, 207, 36, v.Weapon.Name, uiText, 350)
		p.text(774, 244, 20, grade+" · "+v.Weapon.Kind, tint, 350)
		p.line(774, 263, 1126, 263, tint)
		artRect := image.Rect(774, 280, 1126, 566)
		if art := equipmentArt.images[v.Weapon.Name]; art != nil {
			p.containArt(art, artRect)
		} else {
			p.sprite(weaponIcon(v.Weapon.Kind), artRect)
		}

		tech := v.Weapon.Techniques[0]
		if v.Skill != nil {
			tech = v.Skill.Name
		}
		p.text(774, 637, 21, "武学 · "+tech, uiText, 350)
		for i, slot := range []struct {
			label, icon string
			item        *wuxia.Item
		}{{"护具", "ink-item-armor", v.ArmorItem}, {"饰品", "ink-item-jade", v.CharmItem}} {
			y := 674 + i*84
			p.line(774, y, 1126, y, uiEdge)
			if slot.item == nil {
				p.text(774, y+38, 18, slot.label, uiMuted, 80)
				p.text(870, y+38, 21, "尚未佩戴", uiMuted, 250)
			} else {
				p.sprite(slot.icon, image.Rect(774, y+8, 840, y+74))
				g, c := qualityStyle(slot.item.Tier)
				p.text(860, y+32, 22, slot.item.Name, uiText, 260)
				p.text(860, y+62, 17, slot.label+" · "+g, c, 260)
			}
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
		p.commandRibbon(height, []string{"江湖", "江湖练武", "江湖背包", "切磋 @群友"})
	}, 1000)
}
