// Package single_plan implements the "暂时没有恋爱的打算" local template.
package single_plan

import (
	"context"
	_ "embed"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"sync"

	"golang.org/x/image/font/opentype"

	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/textdraw"
	"github.com/Yuelioi/yueling-go/services/imaging"
)

const (
	splitY            = 769
	captionText       = "暂时没有恋爱的打算"
	captionMaxSize    = 68
	captionMinSize    = 42
	captionOutline    = 4
	captionSideMargin = 64
	captionBottomGap  = 24
	captionBoxHeight  = 122
)

//go:embed assets/source.jpg
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
		Key:         "single_plan",
		Description: "保留上半部分，把下半部分替换为图片或用户头像",
		Keywords:    []string{"单身"},
		MinImages:   1,
		MaxImages:   1,
	}
}

func (t *Template) Render(ctx context.Context, request model.RenderRequest) (*imaging.Animation, error) {
	if len(request.Images) != 1 || request.Images[0] == nil || request.Images[0].FrameCount() == 0 {
		return nil, errors.New("该模板需要一张图片")
	}
	t.once.Do(t.loadAssets)
	if t.err != nil {
		return nil, t.err
	}

	input := request.Images[0]
	bounds := t.source.Bounds()
	bottomHeight := bounds.Dy() - splitY
	captionBounds := image.Rect(
		captionSideMargin,
		bounds.Dy()-captionBottomGap-captionBoxHeight,
		bounds.Dx()-captionSideMargin,
		bounds.Dy()-captionBottomGap,
	)
	caption, err := textdraw.FitBox(t.font, captionText, captionBounds, textdraw.BoxStyle{
		MaxFontSize: captionMaxSize,
		MinFontSize: captionMinSize,
	})
	if err != nil {
		return nil, err
	}
	defer caption.Close()

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
		canvas := image.NewNRGBA(bounds)
		top := image.Rect(0, 0, bounds.Dx(), splitY)
		draw.Draw(canvas, top, t.source, image.Point{}, draw.Src)
		bottom := imaging.CoverImage(inputFrame, bounds.Dx(), bottomHeight)
		draw.Draw(canvas, image.Rect(0, splitY, bounds.Dx(), bounds.Dy()), bottom, image.Point{}, draw.Src)
		drawSinglePlanCaption(canvas, caption, captionBounds)
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
	t.source = source.Frames[0]
	t.font, err = textdraw.LoadCJKFont([]string{captionText})
	if err != nil {
		t.err = err
	}
}

func drawSinglePlanCaption(dst draw.Image, caption *textdraw.BoxLayout, bounds image.Rectangle) {
	for dy := -captionOutline; dy <= captionOutline; dy++ {
		for dx := -captionOutline; dx <= captionOutline; dx++ {
			if dx == 0 && dy == 0 || dx*dx+dy*dy > captionOutline*captionOutline {
				continue
			}
			caption.Draw(dst, bounds.Add(image.Pt(dx, dy)), color.NRGBA{A: 245})
		}
	}
	caption.Draw(dst, bounds, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
}
