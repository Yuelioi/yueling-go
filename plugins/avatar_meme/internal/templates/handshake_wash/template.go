// Package handshake_wash implements the "握手洗手" local template.
package handshake_wash

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

	"golang.org/x/image/font/opentype"

	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/textdraw"
	"github.com/Yuelioi/yueling-go/services/imaging"
)

const (
	canvasWidth  = 518
	canvasHeight = 1118
	outlineSize  = 2
)

var (
	textBoxes = [...]image.Rectangle{
		image.Rect(12, 55, 500, 145),
		image.Rect(18, 320, 500, 382),
		image.Rect(18, 425, 500, 525),
	}
	textAlignments = [...]textdraw.HorizontalAlign{
		textdraw.AlignLeft,
		textdraw.AlignRight,
		textdraw.AlignLeft,
	}
	textStyle = textdraw.BoxStyle{MaxFontSize: 32, MinFontSize: 16, LineGap: 3}
)

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
		Key:         "handshake_wash",
		Description: "在握手、伸手、洗手三联图上添加三段文字",
		Keywords:    []string{"握手洗手"},
		MinTexts:    3,
		MaxTexts:    3,
	}
}

func (t *Template) Render(ctx context.Context, request model.RenderRequest) (*imaging.Animation, error) {
	if len(request.Images) != 0 {
		return nil, errors.New("该模板不接受图片")
	}
	if len(request.Texts) != 3 {
		return nil, errors.New("该模板需要三段文字，请用 | 分隔")
	}
	for _, text := range request.Texts {
		if strings.TrimSpace(text) == "" {
			return nil, errors.New("三段文字均不能为空")
		}
	}
	t.once.Do(t.loadAssets)
	if t.err != nil {
		return nil, t.err
	}
	if !textdraw.Supports(t.font, request.Texts) {
		return nil, errors.New("当前字体不支持文字中的部分字符")
	}

	layouts := make([]*textdraw.BoxLayout, len(request.Texts))
	for i, text := range request.Texts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		layout, err := textdraw.FitBox(t.font, strings.TrimSpace(text), textBoxes[i], textStyle)
		if err != nil {
			for _, ready := range layouts[:i] {
				ready.Close()
			}
			return nil, fmt.Errorf("第 %d 段文字：%w", i+1, err)
		}
		layouts[i] = layout
	}
	defer func() {
		for _, layout := range layouts {
			layout.Close()
		}
	}()

	canvas := image.NewNRGBA(t.source.Bounds())
	draw.Draw(canvas, canvas.Bounds(), t.source, image.Point{}, draw.Src)
	for i, layout := range layouts {
		drawOutlined(layout, canvas, textBoxes[i], textAlignments[i])
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
		t.font, err = textdraw.LoadCJKFont([]string{"中文简体繁體握手洗手玩家恋与深空"})
		if err != nil {
			t.err = err
		}
	}
}

func drawOutlined(layout *textdraw.BoxLayout, dst draw.Image, bounds image.Rectangle, align textdraw.HorizontalAlign) {
	for dy := -outlineSize; dy <= outlineSize; dy++ {
		for dx := -outlineSize; dx <= outlineSize; dx++ {
			if dx == 0 && dy == 0 || dx*dx+dy*dy > outlineSize*outlineSize {
				continue
			}
			layout.DrawAligned(dst, bounds.Add(image.Pt(dx, dy)), color.NRGBA{A: 245}, align)
		}
	}
	layout.DrawAligned(dst, bounds, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, align)
}
