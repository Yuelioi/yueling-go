package cards

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/Yuelioi/yueling-go/services/wuxia"
	"golang.org/x/image/font"
)

func Report(ctx context.Context, r wuxia.Result) ([]byte, error) {
	if r.Casino != nil {
		return casinoCard(ctx, r)
	}
	if len(r.Rewards) > 0 {
		return drawCard(ctx, r)
	}
	if r.Battle != nil {
		return battleCard(ctx, r)
	}
	r.Lines = append([]string(nil), r.Lines...)
	for _, id := range []string{"menu-backdrop", "menu-frame", "ink-sprite-" + r.Actor.Fighter().Portrait, "ink-item-silver", "ink-item-scroll"} {
		if err := checkAsset(id); err != nil {
			return nil, err
		}
	}
	for _, e := range r.Encounters {
		if e.Scene != "" {
			if err := checkAsset(e.Scene); err != nil {
				return nil, err
			}
		}
	}
	columns := min(3, len(r.Encounters))
	rows := 0
	if columns > 0 {
		rows = (len(r.Encounters) + columns - 1) / columns
	}
	height := 860
	if rows > 0 {
		height = 450 + rows*394 + len(r.Lines)*27
	}
	return render(ctx, func(p *painter) {
		p.background(height)
		p.atmosphere(height)
		title := "江湖见闻"
		if rows == 0 {
			title = "修行小记"
		}
		if r.Replayed {
			title = "旧事重温"
		}
		p.heading(title, r.Actor.Character.Silver)

		p.text(64, 145, 27, r.Actor.Character.Name, uiText, 610)
		p.text(64, 177, 17, fmt.Sprintf("%d级 · %s · 今日历练 %d/%d", r.Actor.Level, r.Actor.Profession.Name, r.Daily.Runs, r.DailyLimit), uiMuted, 630)
		for i, item := range []struct{ icon, label, value string }{{"ink-item-silver", "本次银两", signedNumber(r.SilverDelta)}, {"ink-item-scroll", "本次阅历", signedNumber(r.ExperienceDelta)}} {
			x := 806 + i*180
			p.sprite(item.icon, image.Rect(x, 121, x+44, 165))
			p.text(x+54, 139, 14, item.label, uiMuted, 120)
			p.text(x+54, 176, 29, item.value, uiAccent, 120)
		}
		if rows == 0 {

			p.sprite("ink-sprite-"+r.Actor.Fighter().Portrait, image.Rect(81, 226, 384, 636))
			p.text(455, 267, 26, "武艺与行囊", uiText, 650)
			p.text(455, 311, 20, fmt.Sprintf("战力 %s    银两 %s", number(int64(r.Actor.Power)), number(r.Actor.Character.Silver)), uiAccent, 650)
			p.text(455, 352, 19, "当前兵刃 · "+r.Actor.Weapon.Name, uiText, 650)
			for i, line := range r.Lines {
				p.text(455, 410+i*32, 19, line, uiMuted, 650)
			}
		} else {
			gap := 22
			width := (1112 - (columns-1)*gap) / columns
			for i, e := range r.Encounters {
				x := 44 + (i%columns)*(width+gap)
				y := 211 + (i/columns)*394
				p.encounterCard(image.Rect(x, y, x+width, y+372), i+1, e)
			}
			y := 211 + rows*394
			p.line(60, y, 1140, y, uiEdge)
			for _, line := range r.Lines {
				p.text(64, y+36, 20, line, uiText, 1072)
				y += 27
			}
		}
		p.commandRibbon(height)
	}, height)
}
func signedNumber(n int64) string {
	if n >= 0 {
		return "+" + number(n)
	}
	return "−" + number(-n)
}

func (p *painter) wrapText(x, y, size int, text string, c color.Color, width, maxLines int) {
	var line strings.Builder
	used := 0
	for _, r := range []rune(text) {
		candidate := line.String() + string(r)
		if font.MeasureString(p.face(size), candidate).Ceil() > width {
			if used == maxLines-1 {
				p.text(x, y+used*(size+8), size, line.String()+"…", c, width)
				return
			}
			p.text(x, y+used*(size+8), size, line.String(), c, width)
			used++
			line.Reset()
		}
		line.WriteRune(r)
	}
	if line.Len() > 0 {
		p.text(x, y+used*(size+8), size, line.String(), c, width)
	}
}
func (p *painter) encounterCard(r image.Rectangle, index int, e wuxia.Encounter) {

	wide := r.Dx() > 600
	imageRect := image.Rect(r.Min.X+2, r.Min.Y+2, r.Max.X-2, r.Min.Y+160)
	x, y := r.Min.X+18, r.Min.Y+190
	width := r.Dx() - 36
	if wide {
		imageRect = image.Rect(r.Min.X+2, r.Min.Y+2, r.Min.X+500, r.Max.Y-2)
		x = r.Min.X + 525
		y = r.Min.Y + 46
		width = r.Max.X - x - 22
	}
	scene := e.Scene
	if scene == "" {
		scene = "bamboo"
	}
	scale(p.image, imageRect, asset(scene))
	fill(p.image, image.Rect(imageRect.Min.X+12, imageRect.Min.Y+12, imageRect.Min.X+51, imageRect.Min.Y+43), color.NRGBA{20, 36, 55, 230})
	p.text(imageRect.Min.X+20, imageRect.Min.Y+35, 17, fmt.Sprintf("%02d", index), uiText, 40)
	status := "路遇"
	c := uiSuccess
	if e.Kind == "setback" {
		status = "破财"
		c = uiDanger
	}
	if e.Kind == "mentor" {
		status = "受教"
	}
	if e.Battle != nil {
		status = "得胜"
		if e.Battle.Winner != 0 {
			status = "暂退"
			c = uiDanger
		}
	}
	p.title(x, y, 24, e.Title, uiText, width)
	p.text(x, y+29, 16, status, c, width)
	p.wrapText(x, y+59, 16, e.Text, uiMuted, width, 2)
	rewardY := r.Max.Y - 68
	p.line(x, rewardY-19, r.Max.X-18, rewardY-19, uiEdge)
	rewardX := x
	if e.Silver != 0 {
		p.sprite("ink-item-silver", image.Rect(rewardX, rewardY-8, rewardX+29, rewardY+20))
		tint := uiAccent
		if e.Silver < 0 {
			tint = uiDanger
		}
		p.text(rewardX+38, rewardY+14, 21, signedNumber(e.Silver), tint, 100)
		rewardX += 145
	}
	if e.Experience != 0 {
		p.sprite("ink-item-scroll", image.Rect(rewardX, rewardY-8, rewardX+29, rewardY+20))
		p.text(rewardX+38, rewardY+14, 21, signedNumber(e.Experience), uiText, 110)
	}
	if e.Kind == "setback" && e.Silver == 0 {
		p.text(x, rewardY+14, 18, "囊中羞涩，未再破财", uiMuted, width)
	}
	if e.Kind == "mentor" && e.Experience == 0 {
		p.text(x, rewardY+14, 18, "阅历已满，温故知新", uiMuted, width)
	}
	if e.Loot != "" {
		p.text(x, r.Max.Y-18, 16, "获得 · "+e.Loot, uiAccent, width)
	}
	p.line(r.Min.X+18, r.Max.Y, r.Max.X-18, r.Max.Y, uiEdge)
}
