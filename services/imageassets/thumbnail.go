package imageassets

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"

	"github.com/Yuelioi/yueling-go/services/imaging"
)

// Thumbnail renders a bounded first-frame preview, including for animated GIFs.
func Thumbnail(ctx context.Context, r io.Reader) ([]byte, error) {
	return imaging.Run(ctx, func() ([]byte, error) {
		data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
		if err != nil {
			return nil, err
		}
		if len(data) > MaxBytes {
			return nil, errors.New("image too large")
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
			return nil, errors.New("image dimensions too large")
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		width, height := cfg.Width, cfg.Height
		if width > 384 || height > 384 {
			if width >= height {
				height = max(1, height*384/width)
				width = 384
			} else {
				width = max(1, width*384/height)
				height = 384
			}
			img = imaging.ResizeImage(img, width, height)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var out bytes.Buffer
		err = png.Encode(&out, img)
		return out.Bytes(), err
	})
}
