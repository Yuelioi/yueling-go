package cards

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/imaging"
	"github.com/Yuelioi/yueling-go/services/wuxia"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed assets/menu-frame-v6.png assets/menu-backdrop-v6.png assets/menu-logo-v5.png assets/stat-glyphs-v5.png assets/ui-display.otf assets/OFL.txt assets/scene-*-v5.png assets/ink-items-v3.png assets/ink-character-*.png assets/ink-defeat-*.png assets/regions.json
var artwork embed.FS

var loaded struct {
	sync.Once
	images  map[string]image.Image
	icons   map[string]image.Image
	regions map[string]region
	err     error
}

var fonts struct {
	sync.Mutex
	parsed *opentype.Font
}

type region struct {
	File    string `json:"file"`
	Columns int    `json:"columns"`
	Rows    int    `json:"rows"`
	Index   int    `json:"index"`
}

func loadAssets() error {
	if err := loadMartialArt(); err != nil {
		return err
	}
	loaded.Do(func() {
		data, err := artwork.ReadFile("assets/regions.json")
		if err != nil {
			loaded.err = err
			return
		}
		if err = json.Unmarshal(data, &loaded.regions); err != nil {
			loaded.err = err
			return
		}
		loaded.images = map[string]image.Image{}
		for id, r := range loaded.regions {
			if r.Columns < 1 || r.Rows < 1 || r.Index < 0 || r.Index >= r.Columns*r.Rows {
				loaded.err = fmt.Errorf("无效图片区域%s", id)
				return
			}
			if loaded.images[r.File] != nil {
				continue
			}
			data, err := artwork.ReadFile("assets/" + r.File)
			if err != nil {
				loaded.err = err
				return
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				loaded.err = err
				return
			}
			loaded.images[r.File] = img
		}
		loaded.icons = map[string]image.Image{}
		for id := range loaded.regions {
			if strings.HasPrefix(id, "ink-item-") || strings.HasPrefix(id, "glyph-") {
				src := asset(id)
				icon := image.NewRGBA(image.Rect(0, 0, 128, 128))
				xdraw.CatmullRom.Scale(icon, icon.Bounds(), src, src.Bounds(), draw.Over, nil)
				loaded.icons[id] = icon
			}
		}
		logo := asset("menu-logo")
		preview := image.NewRGBA(image.Rect(0, 0, 768, 256))
		xdraw.CatmullRom.Scale(preview, preview.Bounds(), logo, logo.Bounds(), draw.Over, nil)
		loaded.icons["menu-logo"] = preview

	})
	return loaded.err
}

func loadFont() (*opentype.Font, error) {
	fonts.Lock()
	defer fonts.Unlock()
	if fonts.parsed != nil {
		return fonts.parsed, nil
	}
	var paths []string
	entries, _ := os.ReadDir(services.DataPath("fonts"))
	for _, e := range entries {
		if !e.IsDir() {
			paths = append(paths, filepath.Join(services.DataPath("fonts"), e.Name()))
		}
	}
	paths = append(paths, "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", "/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc", "/System/Library/Fonts/PingFang.ttc", "/System/Library/Fonts/Supplemental/Songti.ttc", `C:\Windows\Fonts\msyh.ttc`)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		collection, err := opentype.ParseCollection(data)
		if err != nil {
			continue
		}
		for i := 0; i < collection.NumFonts(); i++ {
			f, err := collection.Font(i)
			if err != nil {
				continue
			}
			index, err := f.GlyphIndex(nil, '侠')
			if err == nil && index != 0 {
				fonts.parsed = f
				return f, nil
			}
		}
	}
	return nil, fmt.Errorf("角色卡需要中文字体，请在data/fonts放入Noto或思源字体")
}

type painter struct {
	image   *image.RGBA
	parsed  *opentype.Font
	display *opentype.Font
	faces   map[int]font.Face
}

func (p *painter) close() {
	for _, f := range p.faces {
		_ = f.Close()
	}
}
func (p *painter) face(size int) font.Face {
	if f := p.faces[size]; f != nil {
		return f
	}
	f, err := opentype.NewFace(p.parsed, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	p.faces[size] = f
	return f
}
func (p *painter) text(x, y, size int, s string, c color.Color, maxWidth int) {
	f := p.face(size)
	runes := []rune(s)
	for len(runes) > 0 && font.MeasureString(f, string(runes)).Ceil() > maxWidth {
		runes = runes[:len(runes)-1]
	}
	if len(runes) < len([]rune(s)) && len(runes) > 1 {
		runes = append(runes[:len(runes)-1], '…')
	}
	d := font.Drawer{Dst: p.image, Src: image.NewUniform(c), Face: f, Dot: fixed.P(x, y)}
	d.DrawString(string(runes))
}
func fill(dst draw.Image, r image.Rectangle, c color.Color) {
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Over)
}
func crop(src image.Image, r image.Rectangle) image.Image {
	return src.(interface {
		SubImage(image.Rectangle) image.Image
	}).SubImage(r)
}
func asset(id string) image.Image {
	r := loaded.regions[id]
	img := loaded.images[r.File]
	b := img.Bounds()
	w, h := b.Dx()/r.Columns, b.Dy()/r.Rows
	return crop(img, image.Rect(r.Index%r.Columns*w, r.Index/r.Columns*h, (r.Index%r.Columns+1)*w, (r.Index/r.Columns+1)*h))
}
func scale(dst draw.Image, r image.Rectangle, src image.Image) {
	scaleAligned(dst, r, src, .5)
}
func scaleAligned(dst draw.Image, r image.Rectangle, src image.Image, vertical float64) {
	b := src.Bounds()
	if b.Dx()*r.Dy() > b.Dy()*r.Dx() {
		w := b.Dy() * r.Dx() / r.Dy()
		b.Min.X += (b.Dx() - w) / 2
		b.Max.X = b.Min.X + w
	} else {
		h := b.Dx() * r.Dy() / r.Dx()
		b.Min.Y += int(float64(b.Dy()-h) * vertical)
		b.Max.Y = b.Min.Y + h
	}
	xdraw.ApproxBiLinear.Scale(dst, r, src, b, draw.Over, nil)
}

func render(ctx context.Context, paint func(*painter), height int) ([]byte, error) {
	return imaging.Run(ctx, func() ([]byte, error) {
		if err := loadAssets(); err != nil {
			return nil, err
		}
		f, err := loadFont()
		if err != nil {
			return nil, err
		}
		display, err := loadDisplayFont()
		if err != nil {
			return nil, err
		}
		img := image.NewRGBA(image.Rect(0, 0, CanvasWidth, height))
		fill(img, img.Bounds(), paper)
		p := &painter{image: img, parsed: f, display: display, faces: map[int]font.Face{}}
		defer p.close()
		paint(p)
		var out bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.BestSpeed}
		if err := encoder.Encode(&out, img); err != nil {
			return nil, err
		}
		return out.Bytes(), nil
	})
}

func Summary(r wuxia.Result) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%s · %s · 战力%d · 银两%d\n", r.Actor.Character.Name, r.Actor.Profession.Name, r.Actor.Power, r.Actor.Character.Silver)
	for _, line := range r.Lines {
		fmt.Fprintln(&out, line)
	}
	for _, e := range r.Encounters {
		fmt.Fprintf(&out, "%s %s银两 %s阅历", e.Title, signedNumber(e.Silver), signedNumber(e.Experience))
		if e.Loot != "" {
			fmt.Fprintf(&out, " 获得%s", e.Loot)
		}
		fmt.Fprintln(&out)
	}
	if r.Battle != nil {
		b := r.Battle
		who := "平手"
		if b.Winner == 0 {
			who = b.Left.Name + "胜出"
		}
		if b.Winner == 1 {
			who = b.Right.Name + "胜出"
		}
		fmt.Fprintf(&out, "%s 对 %s：%s · 交手%d招\n", b.Left.Name, b.Right.Name, who, b.Rounds)
		for _, line := range b.Highlights {
			fmt.Fprintln(&out, line)
		}
		return out.String()
	}
	fmt.Fprintf(&out, "今日历练 %d/%d", r.Daily.Runs, r.DailyLimit)
	if r.Replayed {
		fmt.Fprint(&out, " · 旧事重温")
	}
	return out.String()
}

func checkAsset(id string) error {
	if err := loadAssets(); err != nil {
		return err
	}
	if _, ok := loaded.regions[id]; !ok {
		return fmt.Errorf("形象或场景资源不存在: %s", id)
	}
	return nil
}
