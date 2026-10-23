package cards

import (
	"image"
	"image/color"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var display struct {
	sync.Once
	parsed *opentype.Font
	err    error
}

func loadDisplayFont() (*opentype.Font, error) {
	display.Do(func() {
		data, err := artwork.ReadFile("assets/ui-display.otf")
		if err != nil {
			display.err = err
			return
		}
		display.parsed, display.err = opentype.Parse(data)
	})
	return display.parsed, display.err
}
func (p *painter) titleFace(size int, r rune) font.Face {
	index, err := p.display.GlyphIndex(nil, r)
	if err != nil || index == 0 {
		return p.face(size)
	}
	if f := p.faces[-size]; f != nil {
		return f
	}
	f, err := opentype.NewFace(p.display, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	p.faces[-size] = f
	return f
}
func (p *painter) titleWidth(size int, s string) int {
	width := 0
	for _, r := range s {
		width += font.MeasureString(p.titleFace(size, r), string(r)).Ceil()
	}
	return width
}
func (p *painter) title(x, y, size int, s string, c color.Color, width int) {
	runes := []rune(s)
	short := false
	for len(runes) > 0 && p.titleWidth(size, string(runes)) > width {
		runes = runes[:len(runes)-1]
		short = true
	}
	if short && len(runes) > 1 {
		runes = append(runes[:len(runes)-1], '…')
	}
	d := font.Drawer{Dst: p.image, Src: image.NewUniform(c), Dot: fixed.P(x, y)}
	for _, r := range runes {
		d.Face = p.titleFace(size, r)
		d.DrawString(string(r))
	}
}
func (p *painter) centerTitle(x, y, size int, s string, c color.Color, width int) {
	w := min(width, p.titleWidth(size, s))
	p.title(x-w/2, y, size, s, c, width)
}
