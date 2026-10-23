package cards

import (
	"image"
	"image/color"
	"image/draw"
	"sync"

	xdraw "golang.org/x/image/draw"
)

var skinFrames sync.Map

func (p *painter) frame(r image.Rectangle) {
	if r.Empty() {
		return
	}
	key := r.Size()
	load := sync.OnceValue(func() image.Image {
		dst := image.NewRGBA(image.Rectangle{Max: key})
		paintFrame(dst, dst.Bounds())
		return dst
	})
	stored, _ := skinFrames.LoadOrStore(key, load)
	draw.Draw(p.image, r, stored.(func() image.Image)(), image.Point{}, draw.Over)
}
func paintFrame(dst draw.Image, r image.Rectangle) {
	src := asset("menu-frame")
	b := src.Bounds()
	b = b.Inset(b.Dx() / 32)
	cut := b.Dx() / 7
	corner := min(30, min(r.Dx()/4, r.Dy()/4))
	xs := []int{b.Min.X, b.Min.X + cut, b.Max.X - cut, b.Max.X}
	ys := []int{b.Min.Y, b.Min.Y + cut, b.Max.Y - cut, b.Max.Y}
	dx := []int{r.Min.X, r.Min.X + corner, r.Max.X - corner, r.Max.X}
	dy := []int{r.Min.Y, r.Min.Y + corner, r.Max.Y - corner, r.Max.Y}
	for y := range 3 {
		for x := range 3 {
			if x == 1 && y == 1 {
				continue
			}
			xdraw.CatmullRom.Scale(dst, image.Rect(dx[x], dy[y], dx[x+1], dy[y+1]), src, image.Rect(xs[x], ys[y], xs[x+1], ys[y+1]), draw.Over, nil)
		}
	}
}
func (p *painter) brocade(r image.Rectangle, light bool) {
	c := color.RGBA{33, 49, 67, 255}
	if light {
		c = color.RGBA{227, 223, 210, 255}
	}
	for y := r.Min.Y + 5; y+15 < r.Max.Y; y += 24 {
		for x := r.Min.X + 5; x+16 < r.Max.X; x += 24 {
			p.line(x, y, x+15, y, c)
			p.line(x+15, y, x+15, y+15, c)
			p.line(x+15, y+15, x+5, y+15, c)
			p.line(x+5, y+15, x+5, y+6, c)
			p.line(x+5, y+6, x+10, y+6, c)
		}
	}
}
func (p *painter) chamfer(r image.Rectangle, c color.Color) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		pad := max(0, 7-min(y-r.Min.Y, r.Max.Y-y-1))
		fill(p.image, image.Rect(r.Min.X+pad, y, r.Max.X-pad, y+1), c)
	}
}
func (p *painter) plaque(r image.Rectangle, title string) {
	p.chamfer(r, uiEdge)
	p.chamfer(r.Inset(2), color.RGBA{19, 33, 50, 255})
	p.brocade(r.Inset(5), false)
	p.line(r.Min.X+10, r.Min.Y+5, r.Max.X-10, r.Min.Y+5, uiEdge)
	p.line(r.Min.X+10, r.Max.Y-6, r.Max.X-10, r.Max.Y-6, uiEdge)
	p.centerTitle((r.Min.X+r.Max.X)/2, r.Min.Y+r.Dy()/2+9, 24, title, uiText, r.Dx()-28)
}
func (p *painter) gamePanel(r image.Rectangle, title string, light bool) {
	body := uiPanel
	if light {
		body = color.RGBA{237, 231, 215, 255}
	}
	fill(p.image, r.Inset(7), body)
	if light {
		p.line(r.Min.X+24, r.Min.Y+53, r.Max.X-24, r.Min.Y+53, color.RGBA{205, 202, 188, 255})
	}
	p.frame(r)
	if title != "" {
		w := r.Dx() - 58
		p.plaque(image.Rect(r.Min.X+29, r.Min.Y-14, r.Min.X+29+w, r.Min.Y+26), title)
	}
}
func (p *painter) itemSlot(r image.Rectangle, id string, selected bool) {
	fill(p.image, r, color.RGBA{20, 34, 51, 255})
	p.outline(r, uiEdge)
	p.outline(r.Inset(3), color.RGBA{55, 77, 101, 255})
	fill(p.image, image.Rect(r.Min.X+4, r.Max.Y-8, r.Max.X-4, r.Max.Y-4), color.RGBA{70, 114, 137, 255})
	p.sprite(id, r.Inset(7))
	if selected {
		p.outline(r, uiAccent)
		p.diamond(r.Max.X-2, r.Max.Y-2, 3, uiAccent)
	}
}
func (p *painter) emblem(x, y int, id, label, value string) {
	p.diamond(x, y, 35, color.RGBA{19, 34, 53, 255})
	for _, n := range []int{35, 30} {
		p.line(x-n, y, x, y-n, uiEdge)
		p.line(x, y-n, x+n, y, uiEdge)
		p.line(x+n, y, x, y+n, uiEdge)
		p.line(x, y+n, x-n, y, uiEdge)
	}
	p.sprite(id, image.Rect(x-25, y-25, x+25, y+25))
	p.centerTitle(x, y+58, 18, label, uiText, 76)
	p.center(x, y+84, 21, value, uiText, 77)
}
func (p *painter) portrait(id string, r image.Rectangle) {
	src := asset(id)
	b := src.Bounds()
	b.Max.Y = b.Min.Y + b.Dy()*76/100
	scaleAligned(p.image, r, crop(src, b), 0)
}
