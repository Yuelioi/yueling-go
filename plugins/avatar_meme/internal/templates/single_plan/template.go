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

	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
	"github.com/Yuelioi/yueling-go/services/imaging"
)

const (
	splitY              = 769
	captionTargetWidth  = 560
	captionBottomMargin = 38
)

//go:embed assets/source.jpg
var sourceBytes []byte

// The image generator returned a checkerboard-backed caption despite requesting
// alpha. loadAssets deterministically reconstructs the black-outline/white-fill
// glyph mask and removes that checkerboard before the asset is ever rendered.
//
//go:embed assets/caption.png
var captionBytes []byte

type Template struct {
	once    sync.Once
	source  *image.NRGBA
	caption *image.NRGBA
	err     error
}

func New() model.Template { return &Template{} }

func (*Template) Spec() model.TemplateSpec {
	return model.TemplateSpec{
		Key:         "single_plan",
		Description: "保留上半部分，把下半部分替换为图片或用户头像",
		Keywords:    []string{"单身", "单身打算", "暂时不恋爱", "没有恋爱的打算"},
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
	captionWidth := min(captionTargetWidth, bounds.Dx()-40)
	captionHeight := max(1, t.caption.Bounds().Dy()*captionWidth/t.caption.Bounds().Dx())
	caption := imaging.ResizeImage(t.caption, captionWidth, captionHeight)
	captionAt := image.Pt((bounds.Dx()-captionWidth)/2, bounds.Dy()-captionBottomMargin-captionHeight)

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
		draw.Draw(canvas, caption.Bounds().Add(captionAt), caption, image.Point{}, draw.Over)
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
	caption, err := imaging.Decode(captionBytes, imaging.DefaultLimits)
	if err != nil || caption.FrameCount() != 1 {
		t.err = errors.New("模板字幕读取失败")
		return
	}
	t.source = source.Frames[0]
	t.caption, err = extractOutlinedCaption(caption.Frames[0])
	if err != nil {
		t.err = err
	}
}

// extractOutlinedCaption interprets each scanline with an even-odd fill rule.
// Dark runs are the generated black outline; alternating gaps are the white
// glyph fill, while checkerboard and glyph counters remain transparent.
func extractOutlinedCaption(src *image.NRGBA) (*image.NRGBA, error) {
	b := src.Bounds()
	mask := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	minX, minY, maxX, maxY := b.Dx(), b.Dy(), -1, -1
	for y := 0; y < b.Dy(); y++ {
		type run struct{ start, end int }
		var runs []run
		for x := 0; x < b.Dx(); {
			if !darkPixel(src.NRGBAAt(x, y)) {
				x++
				continue
			}
			start := x
			for x < b.Dx() && darkPixel(src.NRGBAAt(x, y)) {
				x++
			}
			runs = append(runs, run{start: start, end: x - 1})
		}
		for _, r := range runs {
			for x := r.start; x <= r.end; x++ {
				mask.SetNRGBA(x, y, color.NRGBA{A: 255})
			}
			minX, maxX = min(minX, r.start), max(maxX, r.end)
			minY, maxY = min(minY, y), max(maxY, y)
		}
		for i := 0; i+1 < len(runs); i += 2 {
			for x := runs[i].end + 1; x < runs[i+1].start; x++ {
				mask.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}
	if maxX < minX || maxY < minY {
		return nil, errors.New("模板字幕提取失败")
	}
	padding := 8
	minX, minY = max(0, minX-padding), max(0, minY-padding)
	maxX, maxY = min(b.Dx(), maxX+padding+1), min(b.Dy(), maxY+padding+1)
	dst := image.NewNRGBA(image.Rect(0, 0, maxX-minX, maxY-minY))
	draw.Draw(dst, dst.Bounds(), mask, image.Pt(minX, minY), draw.Src)
	return dst, nil
}

func darkPixel(pixel color.NRGBA) bool {
	luma := (299*uint32(pixel.R) + 587*uint32(pixel.G) + 114*uint32(pixel.B)) / 1000
	return luma < 205
}
