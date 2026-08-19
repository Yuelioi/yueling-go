// Package mocking_grid implements the nine-panel "嘲笑" local template.
package mocking_grid

import (
	"context"
	_ "embed"
	"errors"
	"image"
	"image/draw"
	"sync"

	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
	"github.com/Yuelioi/yueling-go/services/imaging"
)

var centerRect = image.Rect(245, 245, 475, 475)

//go:embed assets/source.jpg
var sourceBytes []byte

type Template struct {
	once   sync.Once
	source *image.NRGBA
	err    error
}

func New() model.Template { return &Template{} }

func (*Template) Spec() model.TemplateSpec {
	return model.TemplateSpec{
		Key:                 "mocking_grid",
		Description:         "把九宫格中心替换为图片或 @用户头像",
		Keywords:            []string{"嘲笑"},
		MinImages:           1,
		MaxImages:           1,
		AllowAvatarFallback: true,
	}
}

func (t *Template) Render(ctx context.Context, request model.RenderRequest) (*imaging.Animation, error) {
	if len(request.Images) != 1 || request.Images[0] == nil || request.Images[0].FrameCount() == 0 {
		return nil, errors.New("该模板需要一张图片")
	}
	t.once.Do(t.loadAsset)
	if t.err != nil {
		return nil, t.err
	}

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
		center := imaging.CoverImage(inputFrame, centerRect.Dx(), centerRect.Dy())
		draw.Draw(canvas, centerRect, center, image.Point{}, draw.Src)
		frames[i] = canvas
	}

	return &imaging.Animation{
		Frames:    frames,
		Delays:    append([]int(nil), input.Delays...),
		LoopCount: input.LoopCount,
	}, nil
}

func (t *Template) loadAsset() {
	source, err := imaging.Decode(sourceBytes, imaging.DefaultLimits)
	if err != nil || source.FrameCount() != 1 {
		t.err = errors.New("模板底图读取失败")
		return
	}
	if source.Frames[0].Bounds() != image.Rect(0, 0, 720, 720) {
		t.err = errors.New("模板底图尺寸无效")
		return
	}
	t.source = source.Frames[0]
}
