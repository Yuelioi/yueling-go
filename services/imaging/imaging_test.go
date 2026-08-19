package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"testing"
)

func TestApplyPixelTransforms(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	colors := []color.NRGBA{
		{R: 10, A: 255}, {R: 20, A: 255}, {R: 30, A: 255},
		{R: 40, A: 255}, {R: 50, A: 255}, {R: 60, A: 255},
	}
	for i, c := range colors {
		src.SetNRGBA(i%3, i/3, c)
	}

	tests := []struct {
		name string
		op   Operation
		want [][]uint8
	}{
		{"flip horizontal", Operation{Kind: FlipHorizontal}, [][]uint8{{30, 20, 10}, {60, 50, 40}}},
		{"flip vertical", Operation{Kind: FlipVertical}, [][]uint8{{40, 50, 60}, {10, 20, 30}}},
		{"mirror left", Operation{Kind: MirrorLeft}, [][]uint8{{10, 20, 10}, {40, 50, 40}}},
		{"mirror right", Operation{Kind: MirrorRight}, [][]uint8{{30, 20, 30}, {60, 50, 60}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			animation := &Animation{Frames: []*image.NRGBA{cloneNRGBA(src)}}
			if err := Apply(animation, tt.op, DefaultLimits); err != nil {
				t.Fatal(err)
			}
			for y, row := range tt.want {
				for x, want := range row {
					if got := animation.Frames[0].NRGBAAt(x, y).R; got != want {
						t.Fatalf("pixel (%d,%d) red=%d want %d", x, y, got, want)
					}
				}
			}
		})
	}
}

func TestRotateAndResize(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	animation := &Animation{Frames: []*image.NRGBA{src}}
	if err := Apply(animation, Operation{Kind: Rotate, Degrees: 90}, DefaultLimits); err != nil {
		t.Fatal(err)
	}
	if got := animation.Frames[0].Bounds().Size(); got != (image.Point{X: 2, Y: 3}) {
		t.Fatalf("rotated size=%v", got)
	}
	if got := animation.Frames[0].NRGBAAt(1, 0).R; got != 255 {
		t.Fatalf("rotated marker=%d want 255", got)
	}
	if err := Apply(animation, Operation{Kind: Resize, Width: 4}, DefaultLimits); err != nil {
		t.Fatal(err)
	}
	if got := animation.Frames[0].Bounds().Size(); got != (image.Point{X: 4, Y: 6}) {
		t.Fatalf("resized size=%v", got)
	}
}

func TestCoverImageUsesCenteredCrop(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		src.SetNRGBA(0, y, color.NRGBA{R: 255, A: 255})
		src.SetNRGBA(3, y, color.NRGBA{B: 255, A: 255})
	}
	got := CoverImage(src, 2, 2)
	// A centered square crop removes the colored outer columns.
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			p := got.NRGBAAt(x, y)
			if p.R != 0 || p.B != 0 {
				t.Fatalf("outer column leaked into cover crop at (%d,%d): %#v", x, y, p)
			}
		}
	}
}

func TestDecodeGIFAppliesDisposalPrevious(t *testing.T) {
	data := encodeDisposalFixture(t)
	animation, err := Decode(data, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if animation.FrameCount() != 3 {
		t.Fatalf("frames=%d want 3", animation.FrameCount())
	}
	assertPixel(t, animation.Frames[0], 0, 0, color.NRGBA{R: 255, A: 255})
	assertPixel(t, animation.Frames[0], 1, 0, color.NRGBA{R: 255, A: 255})
	assertPixel(t, animation.Frames[1], 0, 0, color.NRGBA{R: 255, A: 255})
	assertPixel(t, animation.Frames[1], 1, 0, color.NRGBA{B: 255, A: 255})
	// Frame 1 restores the canvas, so frame 2 starts from the original red frame.
	assertPixel(t, animation.Frames[2], 0, 0, color.NRGBA{G: 255, A: 255})
	assertPixel(t, animation.Frames[2], 1, 0, color.NRGBA{R: 255, A: 255})
}

func TestAnimationEncodeRoundTrip(t *testing.T) {
	animation, err := Decode(encodeDisposalFixture(t), DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Encode(animation, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(out, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.FrameCount() != 3 {
		t.Fatalf("roundtrip frames=%d want 3", decoded.FrameCount())
	}
	assertPixel(t, decoded.Frames[2], 0, 0, color.NRGBA{G: 255, A: 255})
	assertPixel(t, decoded.Frames[2], 1, 0, color.NRGBA{R: 255, A: 255})
}

func TestProcessStaticPNG(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	src.SetNRGBA(1, 0, color.NRGBA{R: 100, G: 110, B: 120, A: 255})
	var input bytes.Buffer
	if err := png.Encode(&input, src); err != nil {
		t.Fatal(err)
	}
	out, err := Process(input.Bytes(), Operation{Kind: FlipHorizontal}, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("result is not png: %v", err)
	}
	got := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
	if got.R != 100 {
		t.Fatalf("flipped red=%d want 100", got.R)
	}
}

func TestDecodeLimits(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	limits := DefaultLimits
	limits.MaxWidth = 3
	if _, err := Decode(buf.Bytes(), limits); err == nil {
		t.Fatal("expected dimension limit error")
	}
}

func TestRenderTimelineSamplesAnimatedInput(t *testing.T) {
	red := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	red.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	blue := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	blue.SetNRGBA(0, 0, color.NRGBA{B: 255, A: 255})
	input := &Animation{Frames: []*image.NRGBA{red, blue}, Delays: []int{5, 5}}
	result, err := RenderTimeline(3, 5, []*Animation{input}, func(_ int, sampled []*image.NRGBA) (image.Image, error) {
		return sampled[0], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assertPixel(t, result.Frames[0], 0, 0, color.NRGBA{R: 255, A: 255})
	assertPixel(t, result.Frames[1], 0, 0, color.NRGBA{B: 255, A: 255})
	assertPixel(t, result.Frames[2], 0, 0, color.NRGBA{R: 255, A: 255})
}

func encodeDisposalFixture(t *testing.T) []byte {
	t.Helper()
	pal := color.Palette{
		color.NRGBA{},
		color.NRGBA{R: 255, A: 255},
		color.NRGBA{B: 255, A: 255},
		color.NRGBA{G: 255, A: 255},
	}
	frame0 := image.NewPaletted(image.Rect(0, 0, 2, 1), pal)
	frame0.SetColorIndex(0, 0, 1)
	frame0.SetColorIndex(1, 0, 1)
	frame1 := image.NewPaletted(image.Rect(1, 0, 2, 1), pal)
	frame1.SetColorIndex(1, 0, 2)
	frame2 := image.NewPaletted(image.Rect(0, 0, 1, 1), pal)
	frame2.SetColorIndex(0, 0, 3)
	g := &gif.GIF{
		Image:           []*image.Paletted{frame0, frame1, frame2},
		Delay:           []int{5, 7, 9},
		Disposal:        []byte{gif.DisposalNone, gif.DisposalPrevious, gif.DisposalNone},
		LoopCount:       0,
		Config:          image.Config{ColorModel: pal, Width: 2, Height: 1},
		BackgroundIndex: 0,
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func assertPixel(t *testing.T, img image.Image, x, y int, want color.NRGBA) {
	t.Helper()
	got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	if got != want {
		t.Fatalf("pixel (%d,%d)=%#v want %#v", x, y, got, want)
	}
}
