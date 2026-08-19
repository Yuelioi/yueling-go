// Package imaging provides bounded, in-process image and GIF processing.
// It deliberately knows nothing about bot messages, commands, or meme templates.
package imaging

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	"image/png"

	_ "golang.org/x/image/webp"
)

const (
	defaultFrameDelay = 6 // GIF delays are hundredths of a second.
	minimumFrameDelay = 2
)

// Limits bounds compressed input, decoded geometry, animation size, and output.
type Limits struct {
	MaxInputBytes  int
	MaxOutputBytes int
	MaxWidth       int
	MaxHeight      int
	MaxFrames      int
	MaxTotalPixels int64
}

// DefaultLimits are shared by user-triggered image commands.
var DefaultLimits = Limits{
	MaxInputBytes:  16 * 1024 * 1024,
	MaxOutputBytes: 16 * 1024 * 1024,
	MaxWidth:       4096,
	MaxHeight:      4096,
	MaxFrames:      200,
	MaxTotalPixels: 24_000_000,
}

// Animation is a normalized image sequence. Every frame is a complete display
// frame with zero-based, identical bounds; GIF disposal has already been applied.
type Animation struct {
	Frames    []*image.NRGBA
	Delays    []int
	LoopCount int
}

func (a *Animation) Animated() bool { return a != nil && len(a.Frames) > 1 }
func (a *Animation) FrameCount() int {
	if a == nil {
		return 0
	}
	return len(a.Frames)
}

// Decode converts PNG/JPEG/WebP or GIF bytes into normalized complete frames.
func Decode(data []byte, limits Limits) (*Animation, error) {
	if len(data) == 0 {
		return nil, errors.New("图片内容为空")
	}
	if limits.MaxInputBytes > 0 && len(data) > limits.MaxInputBytes {
		return nil, fmt.Errorf("图片超过 %d MiB 限制", limits.MaxInputBytes/(1024*1024))
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("无法识别图片格式")
	}
	if err := validateGeometry(cfg.Width, cfg.Height, 1, limits); err != nil {
		return nil, err
	}

	if format == "gif" {
		return decodeGIF(data, limits)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("图片解码失败")
	}
	return &Animation{Frames: []*image.NRGBA{ToNRGBA(img)}}, nil
}

func decodeGIF(data []byte, limits Limits) (*Animation, error) {
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("GIF 解码失败")
	}
	if len(g.Image) == 0 {
		return nil, errors.New("GIF 没有可用帧")
	}
	if err := validateGeometry(g.Config.Width, g.Config.Height, len(g.Image), limits); err != nil {
		return nil, err
	}

	bounds := image.Rect(0, 0, g.Config.Width, g.Config.Height)
	background := gifBackground(g)
	canvas := image.NewNRGBA(bounds)
	fill(canvas, bounds, background)
	frames := make([]*image.NRGBA, 0, len(g.Image))

	for i, frame := range g.Image {
		disposal := byte(0)
		if i < len(g.Disposal) {
			disposal = g.Disposal[i]
		}
		var before *image.NRGBA
		if disposal == gif.DisposalPrevious {
			before = cloneNRGBA(canvas)
		}
		r := frame.Bounds().Intersect(bounds)
		if !r.Empty() {
			draw.Draw(canvas, r, frame, r.Min, draw.Over)
		}
		frames = append(frames, cloneNRGBA(canvas))

		switch disposal {
		case gif.DisposalBackground:
			fill(canvas, r, background)
		case gif.DisposalPrevious:
			canvas = before
		}
	}

	delays := make([]int, len(frames))
	for i := range delays {
		delays[i] = minimumFrameDelay
		if i < len(g.Delay) && g.Delay[i] > 0 {
			delays[i] = max(g.Delay[i], minimumFrameDelay)
		}
	}
	return &Animation{Frames: frames, Delays: delays, LoopCount: g.LoopCount}, nil
}

func validateGeometry(width, height, frames int, limits Limits) error {
	if width <= 0 || height <= 0 {
		return errors.New("图片尺寸无效")
	}
	if (limits.MaxWidth > 0 && width > limits.MaxWidth) || (limits.MaxHeight > 0 && height > limits.MaxHeight) {
		return fmt.Errorf("图片尺寸超过 %dx%d 限制", limits.MaxWidth, limits.MaxHeight)
	}
	if limits.MaxFrames > 0 && frames > limits.MaxFrames {
		return fmt.Errorf("GIF 超过 %d 帧限制", limits.MaxFrames)
	}
	total := int64(width) * int64(height) * int64(frames)
	if limits.MaxTotalPixels > 0 && total > limits.MaxTotalPixels {
		return errors.New("图片解码规模过大")
	}
	return nil
}

// Encode writes a static animation as PNG and a multi-frame animation as GIF.
func Encode(animation *Animation, limits Limits) ([]byte, error) {
	if animation == nil || len(animation.Frames) == 0 {
		return nil, errors.New("没有可编码的图片帧")
	}
	if animation.Frames[0] == nil {
		return nil, errors.New("图片帧为空")
	}
	bounds := animation.Frames[0].Bounds()
	if err := validateGeometry(bounds.Dx(), bounds.Dy(), len(animation.Frames), limits); err != nil {
		return nil, err
	}
	if len(animation.Frames) == 1 {
		var buf bytes.Buffer
		if err := png.Encode(&buf, animation.Frames[0]); err != nil {
			return nil, fmt.Errorf("PNG 编码失败: %w", err)
		}
		return checkedOutput(buf.Bytes(), limits)
	}

	frames := make([]*image.Paletted, len(animation.Frames))
	delays := make([]int, len(animation.Frames))
	disposals := make([]byte, len(animation.Frames))
	pal := gifPalette()
	for i, frame := range animation.Frames {
		if frame == nil {
			return nil, errors.New("图片帧为空")
		}
		if frame.Bounds() != bounds {
			return nil, errors.New("GIF 帧尺寸不一致")
		}
		p := image.NewPaletted(bounds, pal)
		draw.FloydSteinberg.Draw(p, bounds, frame, bounds.Min)
		frames[i] = p
		delays[i] = defaultFrameDelay
		if i < len(animation.Delays) && animation.Delays[i] > 0 {
			delays[i] = max(animation.Delays[i], minimumFrameDelay)
		}
		// Every encoded frame covers the full canvas. Clearing it after display
		// prevents transparent pixels in the next frame retaining stale content.
		disposals[i] = gif.DisposalBackground
	}

	g := &gif.GIF{
		Image:     frames,
		Delay:     delays,
		LoopCount: animation.LoopCount,
		Disposal:  disposals,
		Config:    image.Config{ColorModel: pal, Width: bounds.Dx(), Height: bounds.Dy()},
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		return nil, fmt.Errorf("GIF 编码失败: %w", err)
	}
	return checkedOutput(buf.Bytes(), limits)
}

// EncodePNG writes one complete display frame as PNG.
func EncodePNG(frame image.Image, limits Limits) ([]byte, error) {
	if frame == nil {
		return nil, errors.New("图片帧为空")
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, frame); err != nil {
		return nil, fmt.Errorf("PNG 编码失败: %w", err)
	}
	return checkedOutput(buf.Bytes(), limits)
}

func checkedOutput(data []byte, limits Limits) ([]byte, error) {
	if limits.MaxOutputBytes > 0 && len(data) > limits.MaxOutputBytes {
		return nil, fmt.Errorf("处理结果超过 %d MiB 限制", limits.MaxOutputBytes/(1024*1024))
	}
	return data, nil
}

// ToNRGBA normalizes an image to zero-based, unpremultiplied RGBA pixels.
func ToNRGBA(src image.Image) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

func gifBackground(g *gif.GIF) color.NRGBA {
	if pal, ok := g.Config.ColorModel.(color.Palette); ok && int(g.BackgroundIndex) < len(pal) {
		return color.NRGBAModel.Convert(pal[g.BackgroundIndex]).(color.NRGBA)
	}
	return color.NRGBA{}
}

func gifPalette() color.Palette {
	pal := make(color.Palette, 0, 256)
	pal = append(pal, color.NRGBA{})
	pal = append(pal, palette.Plan9[:255]...)
	return pal
}

func cloneNRGBA(src *image.NRGBA) *image.NRGBA {
	dst := image.NewNRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}

func fill(dst draw.Image, r image.Rectangle, c color.Color) {
	if r.Empty() {
		return
	}
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Src)
}
