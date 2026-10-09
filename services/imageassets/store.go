// Package imageassets stores validated images for both commands and AI actions.
package imageassets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"github.com/Yuelioi/yueling-go/services/imaging"
	_ "golang.org/x/image/webp"
)

const MaxBytes = 16 << 20
const MaxImages = 8

type Options struct {
	Convert           bool
	MinBytes, Quality int
}
type Result struct {
	Saved, Duplicate, Failed int
	Lines                    []string
}

// Fetch only accepts public HTTP images; dimensions are checked before decoding.
func Fetch(ctx context.Context, url string) ([]byte, error) {
	data, err := httpclient.GetPublicBytesLimitContext(ctx, url, MaxBytes)
	if err != nil {
		return nil, err
	}
	if _, err = imaging.Run(ctx, func() (string, error) { return validate(data) }); err != nil {
		return nil, err
	}
	return data, nil
}
func validate(data []byte) (string, error) {
	if len(data) > MaxBytes {
		return "", errors.New("image too large")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", errors.New("invalid image")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
		return "", errors.New("image dimensions too large")
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return "", errors.New("invalid image data")
	}
	if format == "jpeg" {
		format = "jpg"
	}
	return format, nil
}

// Save publishes each complete file atomically and deduplicates its full target
// name, so identical images may belong to different names or groups.
func Save(ctx context.Context, dir string, urls []string, name func(string) string, opts Options) Result {
	return save(ctx, dir, urls, name, opts, func(ctx context.Context, url string) ([]byte, error) {
		return httpclient.GetPublicBytesLimitContext(ctx, url, MaxBytes)
	})
}
func save(ctx context.Context, dir string, urls []string, name func(string) string, opts Options, fetch func(context.Context, string) ([]byte, error)) Result {
	var result Result
	if len(urls) == 0 || len(urls) > MaxImages {
		result.Failed = 1
		result.Lines = []string{"请提供 1 至 8 张图片"}
		return result
	}
	for i, url := range urls {
		label := fmt.Sprintf("图片%d", i+1)
		err := ctx.Err()
		var data []byte
		if err == nil {
			data, err = fetch(ctx, url)
		}
		ext := ""
		if err == nil {
			ext, err = imaging.Run(ctx, func() (string, error) {
				format, e := validate(data)
				if e != nil {
					return "", e
				}
				if opts.Convert {
					data = services.ShrinkToJPEG(data, opts.MinBytes, opts.Quality)
					return validate(data)
				}
				return format, nil
			})
		}
		if err != nil {
			result.Failed++
			result.Lines = append(result.Lines, label+" 下载或校验失败")
			continue
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(data))[:16]
		base := name(hash)
		if err != nil || base == "" || base == "." || base == ".." || filepath.Base(base) != base || strings.ContainsAny(base, "/\\\x00\r\n") || len(base) > 220 {
			result.Failed++
			result.Lines = append(result.Lines, label+" 名称无效")
			continue
		}
		if err = ctx.Err(); err == nil {
			err = os.MkdirAll(dir, 0755)
		}
		if err == nil {
			err = publish(ctx, dir, base+"."+ext, data)
		}
		switch {
		case errors.Is(err, os.ErrExist):
			result.Duplicate++
			result.Lines = append(result.Lines, label+" 已收录（重复）")
		case err != nil:
			result.Failed++
			result.Lines = append(result.Lines, label+" 保存失败")
		default:
			result.Saved++
			result.Lines = append(result.Lines, label+" 上传成功")
		}
	}
	return result
}
func publish(ctx context.Context, dir, name string, data []byte) error {
	f, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Chmod(0644)
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return os.Link(f.Name(), filepath.Join(dir, name))
}
