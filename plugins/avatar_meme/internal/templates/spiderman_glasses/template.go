// Package spiderman_glasses implements the "蜘蛛人戴眼镜" local template.
package spiderman_glasses

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
	canvasWidth  = 600
	canvasHeight = 555
	maxFontSize  = 34
	minFontSize  = 16
	lineGap      = 5
)

var textBoxes = [...]image.Rectangle{
	image.Rect(303, 18, 582, 260),
	image.Rect(303, 299, 582, 537),
}

var textBoxStyle = textdraw.BoxStyle{
	MaxFontSize: maxFontSize,
	MinFontSize: minFontSize,
	LineGap:     lineGap,
	Color:       color.NRGBA{R: 24, G: 24, B: 27, A: 255},
}

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
		Key:         "spiderman_glasses",
		Description: "把两段文字居中放进右侧上下区域",
		Keywords:    []string{"蜘蛛人戴眼镜"},
		MinTexts:    2,
		MaxTexts:    2,
	}
}

func (t *Template) Render(ctx context.Context, request model.RenderRequest) (*imaging.Animation, error) {
	if len(request.Images) != 0 {
		return nil, errors.New("该模板不接受图片")
	}
	if len(request.Texts) != 2 || strings.TrimSpace(request.Texts[0]) == "" || strings.TrimSpace(request.Texts[1]) == "" {
		return nil, errors.New("该模板需要两段文字，请用 | 分隔")
	}
	t.once.Do(t.loadAssets)
	if t.err != nil {
		return nil, t.err
	}
	if !textdraw.Supports(t.font, request.Texts) {
		return nil, errors.New("当前字体不支持文字中的部分字符")
	}

	canvas := image.NewNRGBA(t.source.Bounds())
	draw.Draw(canvas, canvas.Bounds(), t.source, image.Point{}, draw.Src)
	for i, text := range request.Texts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if err := drawCenteredTextBox(canvas, t.font, strings.TrimSpace(text), textBoxes[i]); err != nil {
			return nil, fmt.Errorf("第 %d 段文字：%w", i+1, err)
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
		t.font, err = textdraw.LoadCJKFont([]string{"中文简体繁體蜘蛛人戴眼镜"})
		if err != nil {
			t.err = err
		}
	}
}

func drawCenteredTextBox(dst draw.Image, parsed *opentype.Font, text string, bounds image.Rectangle) error {
	return textdraw.DrawCenteredBox(dst, parsed, text, bounds, textBoxStyle)
}
