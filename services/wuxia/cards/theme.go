package cards

import (
	"fmt"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"image"
	"image/color"
	"image/draw"
)

const CanvasWidth = 1200

var ink = color.RGBA{33, 48, 65, 255}
var cinnabar = color.RGBA{176, 65, 54, 255}
var paper = color.RGBA{244, 238, 224, 255}
var whiteInk = color.RGBA{245, 239, 222, 255}

func number(n int64) string {
	if n >= 100000000 {
		return fmt.Sprintf("%.1f亿", float64(n)/100000000)
	}
	if n >= 10000 {
		return fmt.Sprintf("%.1f万", float64(n)/10000)
	}
	return fmt.Sprint(n)
}

func (p *painter) center(x, y, size int, s string, c color.Color, width int) {
	w := font.MeasureString(p.face(size), s).Ceil()
	if w > width {
		p.text(x-width/2, y, size, s, c, width)
		return
	}
	p.text(x-w/2, y, size, s, c, width)
}
func (p *painter) line(x1, y1, x2, y2 int, c color.Color) {
	dx, dy := abs(x2-x1), -abs(y2-y1)
	sx, sy := -1, -1
	if x1 < x2 {
		sx = 1
	}
	if y1 < y2 {
		sy = 1
	}
	err := dx + dy
	for {
		p.image.Set(x1, y1, c)
		if x1 == x2 && y1 == y2 {
			break
		}
		e := 2 * err
		if e >= dy {
			err += dy
			x1 += sx
		}
		if e <= dx {
			err += dx
			y1 += sy
		}
	}
}
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
func (p *painter) diamond(x, y, r int, c color.Color) {
	for yy := -r; yy <= r; yy++ {
		w := r - abs(yy)
		fill(p.image, image.Rect(x-w, y+yy, x+w+1, y+yy+1), c)
	}
}
func (p *painter) outline(r image.Rectangle, c color.Color) {
	fill(p.image, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), c)
	fill(p.image, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), c)
	fill(p.image, image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), c)
	fill(p.image, image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), c)
}
func (p *painter) sprite(id string, r image.Rectangle) {
	src := asset(id)
	if cached := loaded.icons[id]; cached != nil && r.Dx() <= cached.Bounds().Dx() && r.Dy() <= cached.Bounds().Dy() {
		src = cached
	}
	b := src.Bounds()
	w, h := r.Dx(), r.Dy()
	if b.Dx()*h > b.Dy()*w {
		h = b.Dy() * w / b.Dx()
	} else {
		w = b.Dx() * h / b.Dy()
	}
	target := image.Rect(r.Min.X+(r.Dx()-w)/2, r.Min.Y+(r.Dy()-h)/2, r.Min.X+(r.Dx()+w)/2, r.Min.Y+(r.Dy()+h)/2)
	xdraw.ApproxBiLinear.Scale(p.image, target, src, b, draw.Over, nil)
}
func (p *painter) progress(r image.Rectangle, fraction float64, c color.Color) {
	fill(p.image, r, uiEdge)
	w := int(float64(r.Dx()) * max(0, min(1, fraction)))
	fill(p.image, image.Rect(r.Min.X, r.Min.Y, r.Min.X+w, r.Max.Y), c)
	if w > 0 {
		p.diamond(r.Min.X+w, r.Min.Y+r.Dy()/2, 4, c)
	}
}
func (p *painter) seal(x, y int, word string) {
	fill(p.image, image.Rect(x, y, x+44, y+44), cinnabar)
	p.outline(image.Rect(x+3, y+3, x+41, y+41), color.RGBA{232, 172, 143, 255})
	p.center(x+22, y+31, 27, word, whiteInk, 36)
}

var uiPanel = color.RGBA{34, 50, 70, 255}
var uiText = color.RGBA{242, 235, 219, 255}
var uiMuted = color.RGBA{164, 184, 202, 255}
var uiAccent = color.RGBA{236, 175, 151, 255}
var uiSuccess = color.RGBA{124, 180, 199, 255}
var uiDanger = color.RGBA{205, 98, 83, 255}
var uiEdge = color.RGBA{83, 108, 132, 255}

func (p *painter) panel(r image.Rectangle) { fill(p.image, r.Inset(6), uiPanel); p.frame(r) }
func (p *painter) heading(section string, silver int64) {
	p.sprite("menu-logo", image.Rect(22, 2, 350, 94))
	p.title(377, 61, 25, section, uiText, 440)
	p.chamfer(image.Rect(967, 27, 1165, 72), color.RGBA{22, 36, 53, 255})
	p.sprite("ink-item-silver", image.Rect(976, 30, 1018, 68))
	p.text(1032, 59, 23, number(silver), uiText, 121)
}
func (p *painter) background(height int) {
	scale(p.image, image.Rect(0, 0, CanvasWidth, height), asset("menu-backdrop"))
}

// Command tiles are reading aids for QQ images, not interactive buttons.
func (p *painter) button(r image.Rectangle, title, command string, active bool) {
	background, foreground := uiPanel, uiText
	if active {
		background = color.RGBA{65, 48, 48, 255}
	}
	p.chamfer(r, background)
	p.outline(r.Inset(1), uiEdge)
	if command == "" {
		p.center((r.Min.X+r.Max.X)/2, r.Min.Y+r.Dy()/2+7, 18, title, foreground, r.Dx()-48)
		return
	}
	p.text(r.Min.X+24, r.Min.Y+34, 19, title, uiMuted, r.Dx()-48)
	p.text(r.Min.X+24, r.Min.Y+68, 23, command, foreground, r.Dx()-48)
}
func (p *painter) dock(height int, commands []struct{ title, command string }) {
	fill(p.image, image.Rect(0, height-158, CanvasWidth, height), color.RGBA{19, 32, 48, 255})
	p.line(40, height-148, 1160, height-148, uiEdge)
	p.text(48, height-121, 17, "在群里发送下方指令", uiMuted, 1104)
	gap, width := 16, (1120-(len(commands)-1)*16)/max(1, len(commands))
	for i, e := range commands {
		x := 40 + i*(width+gap)
		p.button(image.Rect(x, height-100, x+width, height-16), e.title, e.command, i == 0)
	}
}
func weaponIcon(kind string) string {
	icons := map[string]string{"剑": "ink-item-sword", "刀": "ink-item-blade", "拳套": "ink-item-wraps"}
	if id := icons[kind]; id != "" {
		return id
	}
	return "ink-item-scroll"
}

// A continuous veil keeps the scenery visible without placing every datum in a box.
func (p *painter) atmosphere(height int) {
	for y := 104; y < height; y++ {
		a := uint8(210)
		if y > height-220 {
			a = uint8(210 + (y-(height-220))*35/220)
		}
		draw.Draw(p.image, image.Rect(24, y, 1176, y+1), &image.Uniform{color.NRGBA{13, 27, 39, a}}, image.Point{}, draw.Over)
	}
	p.line(60, 110, 1140, 110, uiEdge)
}
func (p *painter) commandRibbon(height int, commands []string) {
	y := height - 76
	fill(p.image, image.Rect(24, y-16, 1176, height), color.RGBA{16, 29, 42, 255})
	p.line(60, y-16, 1140, y-16, uiEdge)
	p.text(60, y+29, 17, "群内指令", uiMuted, 130)
	for i, c := range commands {
		x := 224 + i*230
		p.text(x, y+30, 21, c, uiText, 215)
	}
}

// Equipment must remain whole, including the tip and tassel.
func (p *painter) containArt(src image.Image, r image.Rectangle) {
	b := src.Bounds()
	w, h := r.Dx(), r.Dy()
	if b.Dx()*h > b.Dy()*w {
		h = b.Dy() * w / b.Dx()
	} else {
		w = b.Dx() * h / b.Dy()
	}
	x, y := r.Min.X+(r.Dx()-w)/2, r.Min.Y+(r.Dy()-h)/2
	xdraw.ApproxBiLinear.Scale(p.image, image.Rect(x, y, x+w, y+h), src, b, draw.Over, nil)
}
