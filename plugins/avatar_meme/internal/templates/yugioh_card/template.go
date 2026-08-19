// Package yugioh_card implements the "游戏王" local template.
package yugioh_card

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
	canvasWidth  = 604
	canvasHeight = 876
)

var (
	titleBox         = image.Rect(58, 42, 480, 98)
	pictureBox       = image.Rect(79, 165, 526, 612)
	descriptionBox   = image.Rect(68, 670, 537, 806)
	darkInk          = color.NRGBA{R: 18, G: 29, B: 31, A: 255}
	titleStyle       = textdraw.BoxStyle{MaxFontSize: 30, MinFontSize: 16, LineGap: 2, Color: darkInk}
	descriptionStyle = textdraw.BoxStyle{
		MaxFontSize: 26, MinFontSize: 14, LineGap: 4, Color: darkInk,
	}
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
		Key:         "yugioh_card",
		Description: "生成带标题、图片和说明文字的游戏王卡牌",
		Keywords:    []string{"游戏王"},
		MinImages:   1,
		MaxImages:   1,
		MinTexts:    2,
		MaxTexts:    2,
	}
}

func (t *Template) Render(ctx context.Context, request model.RenderRequest) (*imaging.Animation, error) {
	if len(request.Images) != 1 || request.Images[0] == nil || request.Images[0].FrameCount() == 0 {
		return nil, errors.New("该模板需要一张图片")
	}
	if len(request.Texts) != 2 || strings.TrimSpace(request.Texts[0]) == "" || strings.TrimSpace(request.Texts[1]) == "" {
		return nil, errors.New("该模板需要标题和说明两段文字，请用 | 分隔")
	}
	t.once.Do(t.loadAssets)
	if t.err != nil {
		return nil, t.err
	}
	if !textdraw.Supports(t.font, request.Texts) {
		return nil, errors.New("当前字体不支持文字中的部分字符")
	}

	title, err := textdraw.FitBox(t.font, strings.TrimSpace(request.Texts[0]), titleBox, titleStyle)
	if err != nil {
		return nil, fmt.Errorf("标题：%w", err)
	}
	defer title.Close()
	description, err := textdraw.FitBox(t.font, strings.TrimSpace(request.Texts[1]), descriptionBox, descriptionStyle)
	if err != nil {
		return nil, fmt.Errorf("说明：%w", err)
	}
	defer description.Close()

	input := request.Images[0]
	frames := make([]*image.NRGBA, input.FrameCount())
	for i, inputFrame := range input.Frames {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if inputFrame == nil {
			return nil, errors.New("输入图片包含空帧")
		}
		canvas := image.NewNRGBA(t.source.Bounds())
		draw.Draw(canvas, canvas.Bounds(), t.source, image.Point{}, draw.Src)
		picture := imaging.CoverImage(inputFrame, pictureBox.Dx(), pictureBox.Dy())
		draw.Draw(canvas, pictureBox, picture, image.Point{}, draw.Src)
		title.Draw(canvas, titleBox, titleStyle.Color)
		description.Draw(canvas, descriptionBox, descriptionStyle.Color)
		frames[i] = canvas
	}
	return &imaging.Animation{
		Frames:    frames,
		Delays:    append([]int(nil), input.Delays...),
		LoopCount: input.LoopCount,
	}, nil
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
		t.font, err = textdraw.LoadCJKFont([]string{"中文简体繁體游戏王卡牌标题说明"})
		if err != nil {
			t.err = err
		}
	}
}
