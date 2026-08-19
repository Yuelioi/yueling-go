// Package screen_reaction implements the two-panel "老人微笑" local template.
package screen_reaction

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/textdraw"
	"github.com/Yuelioi/yueling-go/services/imaging"
)

const (
	canvasWidth          = 595
	canvasHeight         = 749
	upperPanelBottom     = 375
	subtitleMaxSize      = 32
	subtitleMinSize      = 18
	subtitleHorizontal   = 24
	subtitleBottomMargin = 13
	subtitleFadeHeight   = 64
	subtitleOutline      = 2
)

var defaultTexts = []string{"第一次看到梗圖產生器", "你在螢幕前的表情"}

//go:embed assets/source.png
var sourceBytes []byte

type Template struct {
	once   sync.Once
	source *image.NRGBA
	font   *opentype.Font
	err    error
}

func New() model.Template { return &Template{} }

func (*Template) Spec() model.TemplateSpec {
	return model.TemplateSpec{
		Key:          "screen_reaction",
		Description:  "给上下两张照片各添加一行底部字幕",
		Keywords:     []string{"老人微笑"},
		MinTexts:     2,
		MaxTexts:     2,
		DefaultTexts: append([]string(nil), defaultTexts...),
	}
}

func (t *Template) Render(ctx context.Context, request model.RenderRequest) (*imaging.Animation, error) {
	if len(request.Images) != 0 {
		return nil, errors.New("该模板不接受图片")
	}
	if len(request.Texts) != 2 || strings.TrimSpace(request.Texts[0]) == "" || strings.TrimSpace(request.Texts[1]) == "" {
		return nil, errors.New("该模板需要两段字幕，请用 | 分隔")
	}
	t.once.Do(t.loadAssets)
	if t.err != nil {
		return nil, t.err
	}
	if !textdraw.Supports(t.font, request.Texts) {
		return nil, errors.New("当前字体不支持字幕中的部分字符")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	canvas := image.NewNRGBA(t.source.Bounds())
	draw.Draw(canvas, canvas.Bounds(), t.source, image.Point{}, draw.Src)
	panelBottoms := [...]int{upperPanelBottom, canvasHeight}
	for i, text := range request.Texts {
		if err := drawSubtitle(canvas, t.font, strings.TrimSpace(text), panelBottoms[i]); err != nil {
			return nil, fmt.Errorf("第 %d 段字幕：%w", i+1, err)
		}
	}
	return &imaging.Animation{Frames: []*image.NRGBA{canvas}}, nil
}

func (t *Template) loadAssets() {
	source, err := imaging.Decode(sourceBytes, imaging.DefaultLimits)
	if err != nil || source.FrameCount() != 1 {
		t.err = errors.New("模板底图读取失败")
		return
	}
	if source.Frames[0].Bounds() != image.Rect(0, 0, canvasWidth, canvasHeight) {
		t.err = errors.New("模板底图尺寸无效")
		return
	}
	t.source = source.Frames[0]
	if t.font == nil {
		required := append([]string(nil), defaultTexts...)
		required = append(required, "梗图产生器 屏幕 萤幕")
		t.font, err = textdraw.LoadCJKFont(required)
		if err != nil {
			t.err = err
		}
	}
}

func drawSubtitle(dst *image.NRGBA, parsed *opentype.Font, text string, panelBottom int) error {
	face, width, err := fitSubtitleFace(parsed, text, dst.Bounds().Dx()-2*subtitleHorizontal)
	if err != nil {
		return err
	}
	if closer, ok := face.(interface{ Close() error }); ok {
		defer closer.Close()
	}
	metrics := face.Metrics()
	baseline := panelBottom - subtitleBottomMargin - metrics.Descent.Ceil()
	top := baseline - metrics.Ascent.Ceil()
	drawSubtitleFade(dst, max(panelBottom-subtitleFadeHeight, top-10), panelBottom)

	x := (dst.Bounds().Dx() - width) / 2
	for dy := -subtitleOutline; dy <= subtitleOutline; dy++ {
		for dx := -subtitleOutline; dx <= subtitleOutline; dx++ {
			if dx == 0 && dy == 0 || dx*dx+dy*dy > subtitleOutline*subtitleOutline {
				continue
			}
			drawSubtitleText(dst, face, color.NRGBA{A: 225}, x+dx, baseline+dy, text)
		}
	}
	drawSubtitleText(dst, face, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, x, baseline, text)
	return nil
}

func fitSubtitleFace(parsed *opentype.Font, text string, maxWidth int) (font.Face, int, error) {
	for size := subtitleMaxSize; size >= subtitleMinSize; size-- {
		face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
			Size: float64(size), DPI: 72, Hinting: font.HintingFull,
		})
		if err != nil {
			return nil, 0, errors.New("字幕字体加载失败")
		}
		width := font.MeasureString(face, text).Ceil()
		if width <= maxWidth {
			return face, width, nil
		}
		if closer, ok := face.(interface{ Close() error }); ok {
			closer.Close()
		}
	}
	return nil, 0, errors.New("文字过长，请缩短后重试")
}

func drawSubtitleFade(dst draw.Image, top, bottom int) {
	top = max(top, 0)
	bottom = min(bottom, dst.Bounds().Dy())
	height := max(1, bottom-top)
	for y := top; y < bottom; y++ {
		progress := float64(y-top) / float64(height)
		alpha := uint8(150 * progress * progress)
		draw.Draw(dst, image.Rect(0, y, dst.Bounds().Dx(), y+1), image.NewUniform(color.NRGBA{A: alpha}), image.Point{}, draw.Over)
	}
}

func drawSubtitleText(dst draw.Image, face font.Face, ink color.Color, x, baseline int, text string) {
	d := font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(ink),
		Face: face,
		Dot:  fixed.P(x, baseline),
	}
	d.DrawString(text)
}
