package cards

import (
	"context"
	"fmt"
	"github.com/Yuelioi/yueling-go/services/wuxia"
	"image"
	"image/color"
)

func casinoCard(ctx context.Context, r wuxia.Result) ([]byte, error) {
	for _, id := range []string{"menu-backdrop", "menu-logo", "ink-item-silver"} {
		if err := checkAsset(id); err != nil {
			return nil, err
		}
	}
	return render(ctx, func(p *painter) {
		const height = 780
		p.background(height)
		p.atmosphere(height)
		p.heading("江湖赌坊", r.Actor.Character.Silver)
		c := r.Casino
		p.text(60, 160, 24, r.Actor.Character.Name, uiText, 600)
		p.text(60, 199, 20, fmt.Sprintf("赌金 · %d 银两", c.Stake), uiMuted, 650)
		title := "这把归庄家"
		quip := "骰盅一扣，银两换了主人。"
		tint := uiDanger
		if c.Won {
			title = "押中了"
			quip = "骰子一落，银两到手。"
			tint = uiAccent
		}
		if c.MultiplierHalf == 2 {
			title = "原银奉还"
			quip = "兜兜转转，银两还是你的。"
			tint = uiText
		}
		if c.MultiplierHalf == 1 {
			title = "留你半袋银两"
			quip = "三奇三偶，庄家退半。"
		}
		if c.MultiplierHalf > 2 {
			title = fmt.Sprintf("%d 倍！", c.MultiplierHalf/2)
			quip = "手气一到，银两自来。"
		}
		if c.MultiplierHalf == 20 {
			quip = "掌柜的笑容，突然就没了。"
		}
		if r.Replayed {
			p.text(940, 160, 18, "旧局回看", uiMuted, 200)
		}
		p.centerTitle(600, 263, 42, title, tint, 800)

		for i, v := range c.Dice {
			p.die(image.Rect(310+i*210, 305, 470+i*210, 465), v)

		}
		p.center(600, 514, 26, fmt.Sprintf("%s · 到账 %d 银两", c.Outcome, c.Payout), uiText, 700)
		p.centerTitle(600, 584, 48, signedNumber(c.Payout-c.Stake)+" 银两", tint, 900)
		p.center(600, 630, 20, quip, uiMuted, 900)
		p.commandRibbon(height)
	}, 780)
}
func (p *painter) die(r image.Rectangle, n int) {
	p.chamfer(r, paper)
	p.outline(r.Inset(5), uiAccent)
	spots := [][2]int{}
	if n%2 == 1 {
		spots = append(spots, [2]int{2, 2})
	}
	if n >= 2 {
		spots = append(spots, [2]int{1, 1}, [2]int{3, 3})
	}
	if n >= 4 {
		spots = append(spots, [2]int{3, 1}, [2]int{1, 3})
	}
	if n == 6 {
		spots = append(spots, [2]int{1, 2}, [2]int{3, 2})
	}
	var tint color.Color = ink
	if n == 1 || n == 4 {
		tint = cinnabar
	}
	radius := r.Dx() / 13
	for _, pt := range spots {
		cx, cy := r.Min.X+pt[0]*r.Dx()/4, r.Min.Y+pt[1]*r.Dy()/4
		for y := -radius; y <= radius; y++ {
			for x := -radius; x <= radius; x++ {
				if x*x+y*y <= radius*radius {
					p.image.Set(cx+x, cy+y, tint)
				}
			}
		}
	}
}
