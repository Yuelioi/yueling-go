package cards

import (
	"context"
	"fmt"
	"image"
	"strings"

	"github.com/Yuelioi/yueling-go/services/wuxia"
)

func drawCard(ctx context.Context, r wuxia.Result) ([]byte, error) {
	if err := loadEquipmentArt(); err != nil {
		return nil, err
	}
	for _, id := range []string{"menu-backdrop", "menu-frame", "ink-item-silver", "ink-item-scroll", "ink-item-armor", "ink-item-jade"} {
		if err := checkAsset(id); err != nil {
			return nil, err
		}
	}
	items := r.RewardItems
	if len(items) != len(r.Rewards) {
		return nil, fmt.Errorf("寻宝回执缺少物品快照")
	}
	return render(ctx, func(p *painter) {
		const height = 860
		p.background(height)
		heading := "江湖寻宝"
		if r.Replayed {
			heading = "寻宝回看"
		}
		p.heading(heading, r.Actor.Character.Silver)
		p.atmosphere(height)
		title := "寻宝所得"
		if r.DrawPoolName != "" {
			title = r.DrawPoolName
		}
		p.title(72, 170, 30, r.Actor.Character.Name+" · "+title, uiText, 820)
		if r.SilverDelta < 0 {
			p.text(916, 170, 21, fmt.Sprintf("净支出 %d 银两", -r.SilverDelta), uiAccent, 212)
		}
		p.text(72, 208, 20, fmt.Sprintf("此行寻得 %d 件宝物", len(items)), uiMuted, 900)
		p.line(72, 232, 1128, 232, uiEdge)
		width, gap := 204, 16
		start := (CanvasWidth - (len(items)*width + (len(items)-1)*gap)) / 2
		for i, item := range items {
			x := start + i*(width+gap)
			grade, tint := qualityStyle(item.Tier)
			if item.Kind == "manual" {
				grade = martialQuality(item.Tier)
			}
			p.line(x+16, 264, x+width-16, 264, tint)
			p.center(x+width/2, 588, 18, grade, tint, width-24)
			kind := map[string]string{"weapon": "兵刃", "armor": "护具", "charm": "饰品", "manual": "武学"}[item.Kind]
			if item.ManualKind == "mind" {
				kind = "心法"
			} else if item.ManualKind == "martial" {
				kind = "武技"
			}
			if kind == "" {
				kind = "宝物"
			}
			p.text(x+24, 302, 19, kind, uiMuted, width-48)
			icon := "ink-item-scroll"
			switch item.Kind {
			case "weapon":
				icon = weaponIcon(item.WeaponKind)
			case "armor":
				icon = "ink-item-armor"
			case "charm":
				icon = "ink-item-jade"
			}
			artRect := image.Rect(x+24, 318, x+width-24, 478)
			if art := equipmentArt.images[item.Name]; art != nil {
				p.containArt(art, artRect)
			} else if art := martialArt.images[item.SkillID]; art != nil {
				p.paintMartial(artRect, art)
			} else {
				p.sprite(icon, artRect)
			}
			p.center(x+width/2, 506, 22, item.Name, uiText, width-24)
			p.center(x+width/2, 544, 21, drawRewardLabel(r.Rewards[i]), uiAccent, width-24)
		}
		rebate := int64(0)
		for _, reward := range r.Rewards {
			rebate += reward.ConvertedSilver
		}
		message := "高品装备自动换上，同品随心搭配。"
		if rebate > 0 {
			message = fmt.Sprintf("重复宝物折银 · 共得 %d 银两", rebate)
		}
		if r.Replayed {
			message = "旧日所得，重展一观。"
		}
		p.text(72, 622, 23, message, uiText, 1056)
		note := "挑选心仪装备，搭配武技心法。"
		if len(r.LearnedNames) > 0 {
			note = "新学会 · " + strings.Join(r.LearnedNames, " / ")
		}
		p.text(72, 655, 18, note, uiMuted, 1056)

		p.commandRibbon(height)
	}, 860)
}

func martialQuality(tier int) string {
	names := []string{"入门", "精修", "上乘", "绝学", "传世"}
	if tier < 1 || tier > 5 {
		return "武学"
	}
	return names[tier-1]
}

func drawRewardLabel(r wuxia.Reward) string {
	if r.ConvertedSilver > 0 {
		return fmt.Sprintf("折银 +%d", r.ConvertedSilver)
	}
	return fmt.Sprintf("×%d", r.Quantity)
}
