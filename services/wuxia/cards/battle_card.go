package cards

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/Yuelioi/yueling-go/services/wuxia"
)

// Effect symbols are shared by the finishing move and the compact exchange strip.
func (p *painter) combatIcon(x, y, size int, effect string, c color.Color) {
	stroke := max(2, size/18)
	line := func(x1, y1, x2, y2 int) {
		for n := -stroke / 2; n <= stroke/2; n++ {
			p.line(x+x1+n, y+y1, x+x2+n, y+y2, c)
		}
	}
	switch {
	case strings.Contains(effect, "暴击"):
		for n := 0; n < 8; n++ {
			a := float64(n) * math.Pi / 4
			b := a + math.Pi/8
			d := a + math.Pi/4
			r := float64(size) / 2
			q := r * .35
			line(int(math.Cos(a)*r), int(math.Sin(a)*r), int(math.Cos(b)*q), int(math.Sin(b)*q))
			line(int(math.Cos(b)*q), int(math.Sin(b)*q), int(math.Cos(d)*r), int(math.Sin(d)*r))
		}
	case strings.Contains(effect, "破防"):
		line(-size/3, -size/3, 0, -size/2)
		line(0, -size/2, size/3, -size/3)
		line(-size/3, -size/3, -size/3, size/6)
		line(size/3, -size/3, size/3, size/6)
		line(-size/3, size/6, 0, size/2)
		line(size/3, size/6, 0, size/2)
		line(size/5, -size/2, -size/8, 0)
		line(-size/8, 0, size/8, 0)
		line(size/8, 0, -size/5, size/2)
	case strings.Contains(effect, "闪"):
		line(-size/3, size/3, size/3, -size/3)
		line(-size/3, 0, 0, -size/3)
		line(0, size/3, size/3, 0)
	default:
		line(-size/3, size/3, size/3, -size/3)
		line(-size/4, size/8, -size/8, size/4)
		line(-size/3, size/3, -size/2, size/2)
		line(size/3, -size/3, size/6, -size/3)
	}
}

func (p *painter) fighterAvatar(id string, r image.Rectangle, tint color.Color) {
	src := asset("ink-sprite-" + id)
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	b = image.Rect(b.Min.X+w*30/100, b.Min.Y+h*6/100, b.Min.X+w*80/100, b.Min.Y+h*38/100)
	p.chamfer(r, color.RGBA{28, 44, 57, 255})
	scaleAligned(p.image, r.Inset(4), crop(src, b), 0)
	p.line(r.Min.X, r.Max.Y, r.Max.X, r.Max.Y, tint)
}

// Recorded seeds keep replayed cards consistent while different bouts vary their banter.
var battleBanter = map[string][]string{
	"完胜":   {"就这狗屁功夫，也敢出来装逼？", "回家吃奶去，别在这儿丢人现眼。", "你他妈倒是还手啊，来碰瓷的？", "剑没开刃，你嘴倒是开光了。", "菜成这逼样，还敢报师门？", "我热身都没完，你就躺平了？", "打你都嫌涨不了经验，晦气。", "你这身装备，穿狗身上都比你凶。"},
	"稳稳拿下": {"装最狠的逼，挨最稳的揍。", "嘴硬得像神兵，身板脆得像饼。", "你他妈除了叫，还会点别的吗？", "菜就认，别把借口当内功练。", "就你这两下，配剑不如配钥匙。", "打不过就闭嘴，别在地上输出。", "人菜瘾还大，挨揍还插队。", "别吹牛逼了，先把血条找回来。"},
	"险胜":   {"残血都收不掉，你他妈玩什么？", "就剩这点血，你也配不走。", "差点赢？败字看不懂是吧？", "你打了半天，就配看我丝血装逼。", "血条给你打，胜场你别想。", "一滴血够赢你，再多都是浪费。", "不是喜欢叫？来，对着败字叫。", "操作猛如虎，结算你是个二百五。", "老子血少，不代表你配赢。", "你把吃奶的劲都使了，就这？", "赢不了残血，还搁这儿吹牛逼？", "你负责打满伤害，我负责收下胜场。"},
	"胜一筹":  {"刚才不是挺能装？接着叫啊。", "不服？憋着，战绩不听你放屁。", "你的嘴负责装逼，我的剑负责拆。", "别瞪了，再瞪也是个败字。", "出来混，欠的揍迟早要挨。", "输了就认，少他妈嘴硬。", "打架不行，赛后放屁你第一。", "菜得挺稳定，嘴硬得挺离谱。"},
	"不分伯仲": {"两个菜逼，打出了巅峰对决。", "打得狗屁不通，嘴都硬得要命。", "谁也别装逼，半斤对八两。", "互相刮痧半天，还都觉得自己牛逼。"},
}

func battleFlavor(b *wuxia.Battle) (string, string) {
	label := "不分伯仲"
	if b.Winner >= 0 && b.Winner < 2 {
		label = "胜一筹"
		if len(b.Actions) > 0 {
			f := [2]wuxia.Fighter{b.Left, b.Right}
			hp := max(1, f[b.Winner].Stats.HP)
			left := b.Remaining[b.Winner]
			switch {
			case left >= hp:
				label = "完胜"
			case left*100 >= hp*75:
				label = "稳稳拿下"
			case left*100 <= hp*25:
				label = "险胜"
			}
		} else {
			return label, "承让承让，下回还来。"
		}
	}
	lines := battleBanter[label]
	return label, lines[b.Seed%uint64(len(lines))]
}

// A winding sword trail connects actions in reading order, including row turns.
func battleTrailPoint(n int) (int, int) {
	col := n % 4
	if n/4%2 == 1 {
		col = 3 - col
	}
	return 180 + col*275, 823 + n/4*120
}
func (p *painter) swordTrail(n, total int) {
	x, y := battleTrailPoint(n)
	p.diamond(x, y, 4, uiAccent)
	if n == 0 {
		p.text(x-47, y+6, 16, "起", uiAccent, 28)
	}
	if n == total-1 {
		p.text(x-8, y+25, 16, "收", uiAccent, 28)
		return
	}
	xx, yy := battleTrailPoint(n + 1)
	if y == yy {
		p.line(x, y, xx, yy, uiEdge)
		dir := 1
		if xx < x {
			dir = -1
		}
		mid := (x + xx) / 2
		p.line(mid-dir*9, y-5, mid, y, uiAccent)
		p.line(mid-dir*9, y+5, mid, y, uiAccent)
	} else {
		side := 1
		if n/4%2 == 1 {
			side = -1
		}
		px, py := x, y
		for i := 1; i <= 40; i++ {
			t := float64(i) / 40
			nx := x + int(float64(side*110)*math.Sin(math.Pi*t))
			ny := y + int(float64(yy-y)*t)
			p.line(px, py, nx, ny, uiEdge)
			px, py = nx, ny
		}
		ax, ay := x+side*110, (y+yy)/2
		p.line(ax-5, ay-8, ax, ay, uiAccent)
		p.line(ax+5, ay-8, ax, ay, uiAccent)
	}
}

func battleCard(ctx context.Context, r wuxia.Result) ([]byte, error) {
	b := r.Battle
	for _, id := range []string{"menu-backdrop", "menu-frame", "ink-sprite-" + b.Left.Portrait, "ink-sprite-" + b.Right.Portrait, "ink-item-silver"} {
		if err := checkAsset(id); err != nil {
			return nil, err
		}
	}
	rows := (len(b.Actions) + 3) / 4
	height := 870 + rows*120
	return render(ctx, func(p *painter) {
		p.background(height)
		p.atmosphere(height)
		p.heading("群侠切磋", r.Actor.Character.Silver)
		if r.SilverDelta > 0 || r.ExperienceDelta > 0 {
			p.text(510, 63, 17, fmt.Sprintf("今日首战 +%d银两 · +%d阅历", r.SilverDelta, r.ExperienceDelta), uiAccent, 440)
		}
		fighters := [2]wuxia.Fighter{b.Left, b.Right}
		outcome, quip := battleFlavor(b)
		verdict := outcome
		if b.Winner >= 0 && b.Winner < 2 {
			verdict = fighters[b.Winner].Name + " · " + outcome
		}
		p.centerTitle(600, 176, 40, verdict, uiAccent, 1000)
		p.center(600, 210, 18, fmt.Sprintf("交手 %d 招", len(b.Actions)), uiMuted, 500)
		for i, f := range fighters {
			x := 80 + i*780
			c := uiSuccess
			if i == 1 {
				c = uiDanger
			}
			p.resultAvatar(f.Portrait, image.Rect(x, 254, x+260, 494), c, b.Winner >= 0 && b.Winner < 2 && b.Winner != i)
			p.center(x+130, 536, 27, f.Name, uiText, 340)
			p.center(x+130, 568, 18, f.Profession+" · "+f.Weapon, uiMuted, 340)
			if len(b.Actions) > 0 {
				p.progress(image.Rect(x, 590, x+260, 596), float64(b.Remaining[i])/float64(max(1, f.Stats.HP)), c)
				p.center(x+130, 625, 18, fmt.Sprintf("%d / %d", b.Remaining[i], f.Stats.HP), uiMuted, 300)
			}
			// Equipped martial art and mind remain visible even if neither triggered.
			for n, sk := range []*wuxia.Skill{f.Martial, f.Mind} {
				sx := x + n*134
				p.battleSkillIcon(image.Rect(sx, 634, sx+28, 662), sk)
				name := "未修习"
				tint := uiMuted
				if sk != nil {
					name = sk.Name
					_, tint = qualityStyle(sk.Tier)
				}
				p.text(sx+34, 654, 14, name, tint, 96)
			}
		}
		if len(b.Actions) > 0 {
			last := b.Actions[len(b.Actions)-1]
			// Delayed poison can kill the acting fighter after their attack.
			if last.PoisonDamage > 0 && b.Winner == 1-last.Actor && b.Remaining[last.Actor] == 0 {
				last.Actor = b.Winner
				last.Damage = last.PoisonDamage
				last.Technique = "蛊毒噬心"
				last.Effect = "毒发"
				last.SkillID = ""
				if fighters[last.Actor].Martial != nil {
					last.SkillID = fighters[last.Actor].Martial.ID
				}
			}
			p.center(600, 268, 20, "施展", uiAccent, 400)
			p.actionArt(image.Rect(530, 274, 670, 414), fighters[last.Actor], last.SkillID, false)
			if last.Effect != "命中" {
				p.combatIcon(668, 400, 26, last.Effect, uiAccent)
			}
			p.centerTitle(600, 450, 38, last.Technique, uiText, 470)
			p.center(600, 493, 19, quip, uiMuted, 440)
			p.centerTitle(600, 582, 78, fmt.Sprintf("−%d", last.Damage), uiAccent, 440)
			p.center(600, 622, 23, last.Effect, uiAccent, 400)
		}
		p.line(60, 670, 1140, 670, uiEdge)
		p.text(60, 707, 18, "剑路", uiMuted, 360)
		for i, e := range []string{"命中", "暴击", "破防"} {
			x := 790 + i*120
			p.combatIcon(x, 700, 22, e, uiMuted)
			p.text(x+20, 707, 16, e, uiMuted, 90)
		}
		for n, a := range b.Actions {
			tx, ty := battleTrailPoint(n)
			x, y := tx-120, ty-82
			p.swordTrail(n, len(b.Actions))
			c := uiSuccess
			if a.Actor == 1 {
				c = uiDanger
			}

			p.fighterAvatar(fighters[a.Actor].Portrait, image.Rect(x+32, y, x+80, y+48), c)
			p.actionArt(image.Rect(x+84, y, x+132, y+48), fighters[a.Actor], a.SkillID, true)
			if a.Effect != "命中" {
				p.combatIcon(x+128, y+46, 16, a.Effect, c)
			}
			p.text(x+133, y+35, 28, fmt.Sprintf("−%d", a.Damage), c, 106)

			// One node per action: a combo never expands the trail.
			if len(a.Hits) > 1 {
				p.text(x+134, y+61, 15, fmt.Sprintf("×%d", len(a.Hits)), uiAccent, 45)
			}
			if a.Combo {
				p.text(x+180, y+61, 14, "追击", uiAccent, 55)
			}
			if a.PoisonApplied > 0 {
				p.text(x+32, y+94, 14, fmt.Sprintf("附毒 %d", a.PoisonApplied), uiAccent, 95)
			}
			if a.PoisonDamage > 0 {
				p.text(x+134, y+94, 14, fmt.Sprintf("毒 −%d", a.PoisonDamage), uiDanger, 100)
			}
			if a.LifeSteal > 0 && a.MindID == "" {
				p.text(x+134, y+78, 14, fmt.Sprintf("汲取 +%d", a.LifeSteal), uiSuccess, 100)
			}
			actorMind := fighters[a.Actor].Mind
			if a.MindID != "" && actorMind != nil && actorMind.ID == a.MindID {
				p.battleSkillIcon(image.Rect(x+32, y+53, x+54, y+75), actorMind)
				if a.Healing > 0 {
					p.text(x+60, y+71, 17, fmt.Sprintf("+%d", a.Healing), uiSuccess, 65)
				}
			} else {
				streak := 1
				for j := n - 1; j >= 0 && b.Actions[j].Actor == a.Actor; j-- {
					streak++
				}
				if streak > 1 {
					p.text(x+32, y+70, 15, fmt.Sprintf("连攻 ×%d", streak), c, 100)
				}
			}
			defender := fighters[1-a.Actor]
			if a.DefenderMindID != "" && defender.Mind != nil && defender.Mind.ID == a.DefenderMindID {
				dc := uiSuccess
				if a.Actor == 0 {
					dc = uiDanger
				}
				p.fighterAvatar(defender.Portrait, image.Rect(x+181, y+53, x+201, y+73), dc)
				p.battleSkillIcon(image.Rect(x+204, y+53, x+226, y+75), defender.Mind)
			}

		}
		if len(b.Actions) == 0 {
			p.center(600, 759, 22, "此战已了，来日再会。", uiMuted, 900)
		}
		p.commandRibbon(height)
	}, height)
}

// Defeat art belongs to the result portrait only, never to earlier combat actions.
func (p *painter) resultAvatar(id string, r image.Rectangle, tint color.Color, defeated bool) {
	if defeated {
		if _, ok := loaded.regions["ink-defeat-"+id]; ok {
			p.chamfer(r, color.RGBA{28, 44, 57, 255})
			scaleAligned(p.image, r.Inset(4), asset("ink-defeat-"+id), .5)
			p.line(r.Min.X, r.Max.Y, r.Max.X, r.Max.Y, tint)
			return
		}
	}
	p.fighterAvatar(id, r, tint)
}
