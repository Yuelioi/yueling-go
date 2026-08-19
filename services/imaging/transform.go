package imaging

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"

	xdraw "golang.org/x/image/draw"
)

// Kind identifies a deterministic image operation.
type Kind string

const (
	FlipHorizontal Kind = "flip_horizontal"
	FlipVertical   Kind = "flip_vertical"
	MirrorLeft     Kind = "mirror_left"
	MirrorRight    Kind = "mirror_right"
	MirrorTop      Kind = "mirror_top"
	MirrorBottom   Kind = "mirror_bottom"
	Rotate         Kind = "rotate"
	Resize         Kind = "resize"
	Grayscale      Kind = "grayscale"
	Invert         Kind = "invert"
)

// Operation describes one transform. Rotate uses Degrees. Resize accepts one
// or both dimensions; Scale takes precedence when greater than zero.
type Operation struct {
	Kind    Kind
	Degrees int
	Width   int
	Height  int
	Scale   float64
}

// Process decodes, transforms, and re-encodes an image through one small seam.
func Process(data []byte, operation Operation, limits Limits) ([]byte, error) {
	animation, err := Decode(data, limits)
	if err != nil {
		return nil, err
	}
	if err := Apply(animation, operation, limits); err != nil {
		return nil, err
	}
	return Encode(animation, limits)
}

// Apply transforms every complete display frame in place while retaining GIF timing.
func Apply(animation *Animation, operation Operation, limits Limits) error {
	if animation == nil || len(animation.Frames) == 0 {
		return errors.New("没有可处理的图片帧")
	}
	var targetW, targetH int
	if operation.Kind == Resize {
		targetW, targetH = resizeDimensions(animation.Frames[0].Bounds(), operation)
		if targetW <= 0 || targetH <= 0 {
			return errors.New("缩放尺寸无效")
		}
		if err := validateGeometry(targetW, targetH, len(animation.Frames), limits); err != nil {
			return err
		}
	}

	frames := make([]*image.NRGBA, len(animation.Frames))
	for i, frame := range animation.Frames {
		var transformed *image.NRGBA
		switch operation.Kind {
		case FlipHorizontal:
			transformed = flipHorizontal(frame)
		case FlipVertical:
			transformed = flipVertical(frame)
		case MirrorLeft, MirrorRight, MirrorTop, MirrorBottom:
			transformed = mirror(frame, operation.Kind)
		case Rotate:
			var err error
			transformed, err = rotate(frame, operation.Degrees)
			if err != nil {
				return err
			}
		case Resize:
			transformed = ResizeImage(frame, targetW, targetH)
		case Grayscale:
			transformed = grayscale(frame)
		case Invert:
			transformed = invert(frame)
		default:
			return fmt.Errorf("不支持的图片操作 %q", operation.Kind)
		}
		frames[i] = transformed
	}
	animation.Frames = frames
	return nil
}

func resizeDimensions(bounds image.Rectangle, operation Operation) (int, int) {
	w, h := bounds.Dx(), bounds.Dy()
	if operation.Scale > 0 {
		return max(1, int(float64(w)*operation.Scale+0.5)), max(1, int(float64(h)*operation.Scale+0.5))
	}
	tw, th := operation.Width, operation.Height
	if tw > 0 && th == 0 {
		th = max(1, int(float64(h)*float64(tw)/float64(w)+0.5))
	}
	if th > 0 && tw == 0 {
		tw = max(1, int(float64(w)*float64(th)/float64(h)+0.5))
	}
	return tw, th
}

// ResizeImage resamples src to exactly width x height.
func ResizeImage(src image.Image, width, height int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
	return dst
}

// CoverImage center-crops src to the requested aspect ratio, then resizes it.
func CoverImage(src image.Image, width, height int) *image.NRGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	cropW, cropH := sw, sh
	if int64(sw)*int64(height) > int64(sh)*int64(width) {
		cropW = max(1, int(float64(sh)*float64(width)/float64(height)+0.5))
	} else {
		cropH = max(1, int(float64(sw)*float64(height)/float64(width)+0.5))
	}
	x := b.Min.X + (sw-cropW)/2
	y := b.Min.Y + (sh-cropH)/2
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, image.Rect(x, y, x+cropW, y+cropH), xdraw.Src, nil)
	return dst
}

// CropSquare returns a centered square crop, normalized to zero-based bounds.
func CropSquare(src image.Image) *image.NRGBA {
	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	x := b.Min.X + (b.Dx()-side)/2
	y := b.Min.Y + (b.Dy()-side)/2
	dst := image.NewNRGBA(image.Rect(0, 0, side, side))
	draw.Draw(dst, dst.Bounds(), src, image.Pt(x, y), draw.Src)
	return dst
}

// Circle masks pixels outside the centered circle to transparent.
func Circle(src image.Image) *image.NRGBA {
	dst := ToNRGBA(src)
	cx := float64(dst.Bounds().Dx()-1) / 2
	cy := float64(dst.Bounds().Dy()-1) / 2
	r := min(float64(dst.Bounds().Dx()), float64(dst.Bounds().Dy())) / 2
	r2 := r * r
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < dst.Bounds().Dx(); x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			if dx*dx+dy*dy > r2 {
				dst.SetNRGBA(x, y, color.NRGBA{})
			}
		}
	}
	return dst
}

func flipHorizontal(src *image.NRGBA) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.SetNRGBA(w-1-x, y, src.NRGBAAt(x, y))
		}
	}
	return dst
}

func flipVertical(src *image.NRGBA) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.SetNRGBA(x, h-1-y, src.NRGBAAt(x, y))
		}
	}
	return dst
}

func mirror(src *image.NRGBA, kind Kind) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx, sy := x, y
			switch kind {
			case MirrorLeft:
				sx = min(x, w-1-x)
			case MirrorRight:
				sx = max(x, w-1-x)
			case MirrorTop:
				sy = min(y, h-1-y)
			case MirrorBottom:
				sy = max(y, h-1-y)
			}
			dst.SetNRGBA(x, y, src.NRGBAAt(sx, sy))
		}
	}
	return dst
}

func rotate(src *image.NRGBA, degrees int) (*image.NRGBA, error) {
	degrees = ((degrees % 360) + 360) % 360
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	switch degrees {
	case 0:
		return cloneNRGBA(src), nil
	case 90:
		dst := image.NewNRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.SetNRGBA(h-1-y, x, src.NRGBAAt(x, y))
			}
		}
		return dst, nil
	case 180:
		return flipVertical(flipHorizontal(src)), nil
	case 270:
		dst := image.NewNRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.SetNRGBA(y, w-1-x, src.NRGBAAt(x, y))
			}
		}
		return dst, nil
	default:
		return nil, errors.New("首版旋转仅支持 0、90、180、270 度")
	}
}

func grayscale(src *image.NRGBA) *image.NRGBA {
	dst := cloneNRGBA(src)
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < dst.Bounds().Dx(); x++ {
			p := dst.NRGBAAt(x, y)
			v := uint8((299*uint32(p.R) + 587*uint32(p.G) + 114*uint32(p.B)) / 1000)
			dst.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: p.A})
		}
	}
	return dst
}

func invert(src *image.NRGBA) *image.NRGBA {
	dst := cloneNRGBA(src)
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < dst.Bounds().Dx(); x++ {
			p := dst.NRGBAAt(x, y)
			dst.SetNRGBA(x, y, color.NRGBA{R: 255 - p.R, G: 255 - p.G, B: 255 - p.B, A: p.A})
		}
	}
	return dst
}
