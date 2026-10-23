package cards

import (
	"context"
	"fmt"
	"image"

	"github.com/Yuelioi/yueling-go/services/wuxia"
)

func drawCard(ctx context.Context, r wuxia.Result) ([]byte, error) {
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
			p.text(916, 170, 21, fmt.Sprintf("花费 %d 银两", -r.SilverDelta), uiAccent, 212)
		}
		p.text(72, 208, 20, fmt.Sprintf("此行寻得 %d 件宝物", len(items)), uiMuted, 900)
		p.line(72, 232, 1128, 232, uiEdge)
		width, gap := 204, 16
		start := (CanvasWidth - (len(items)*width + (len(items)-1)*gap)) / 2
		for i, item := range items {
			x := start + i*(width+gap)
			grade, tint := qualityStyle(item.Tier)
			p.line(x+16, 264, x+width-16, 264, tint)
			p.center(x+width/2, 588, 18, grade, tint, width-24)
			kind := map[string]string{"weapon": "兵刃", "armor": "护具", "charm": "饰品", "manual": "武学"}[item.Kind]
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
			p.sprite(icon, image.Rect(x+30, 326, x+174, 470))
			p.center(x+width/2, 506, 22, item.Name, uiText, width-24)
			p.center(x+width/2, 544, 21, fmt.Sprintf("×%d", r.Rewards[i].Quantity), uiAccent, width-24)
		}
		message := "宝物各有归处，且去行囊细看。"
		if r.Replayed {
			message = "旧日所得，重展一观。"
		}
		p.text(72, 622, 23, message, uiText, 1056)
		p.text(72, 655, 18, "换上心仪装备，或研习新得的武学。", uiMuted, 1056)

		p.commandRibbon(height, []string{"江湖背包", "江湖寻宝", "江湖奖池", "我的侠客"})
	}, 860)
}
