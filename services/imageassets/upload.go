package imageassets

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/imaging"
)

var ErrUpload = errors.New("请提供有效图片，单张不超过 16 MiB、8192 像素边长及 1600 万像素")

type UploadResult struct {
	Name      string `json:"name"`
	Duplicate bool   `json:"duplicate"`
}

// Upload applies the category's command naming policy to local file bytes.
// It never overwrites a target or follows a supplied filesystem path.
func (l *Library) Upload(ctx context.Context, category, name string, groupID int64, data []byte, opts Options) (UploadResult, error) {
	result := UploadResult{}
	c, err := l.resolve(category)
	if err != nil {
		return result, err
	}
	if !c.Upload {
		return result, fmt.Errorf("%w：该类目未配置添加功能", ErrName)
	}
	if c.Naming == "quotation" && groupID <= 0 {
		return result, fmt.Errorf("%w：请选择语录所属群", ErrName)
	}
	_, namer, err := l.UploadTarget(category, name, groupID)
	if err != nil {
		return result, err
	}
	ext, err := imaging.Run(ctx, func() (string, error) {
		format, e := validate(data)
		if e != nil {
			return "", ErrUpload
		}
		if opts.Convert {
			data = services.ShrinkToJPEG(data, opts.MinBytes, opts.Quality)
			return validate(data)
		}
		return format, nil
	})
	if err != nil {
		return result, err
	}
	filename := namer(fmt.Sprintf("%x", sha256.Sum256(data))[:16]) + "." + ext
	if !imageName(filename) || len(filename) > 240 {
		return result, ErrName
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	root, err := os.OpenRoot(l.root)
	if err != nil {
		return result, err
	}
	defer root.Close()
	dir := filepath.Join("images", c.Name)
	for _, part := range []string{"images", dir} {
		if err := root.Mkdir(part, 0755); err != nil && !errors.Is(err, os.ErrExist) {
			return result, err
		}
		stat, err := root.Lstat(part)
		if err != nil {
			return result, err
		}
		if !stat.IsDir() || stat.Mode()&os.ModeSymlink != 0 {
			return result, ErrUnsafe
		}
	}
	temp := filepath.Join(dir, ".upload-"+rand.Text())
	out, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return result, err
	}
	defer root.Remove(temp)
	_, err = out.Write(data)
	closeErr := out.Close()
	if err != nil {
		return result, err
	}
	if closeErr != nil {
		return result, closeErr
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	dest := filepath.Join(dir, filename)
	if err := root.Link(temp, dest); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return result, err
		}
		stat, err := root.Lstat(dest)
		if err != nil {
			return result, err
		}
		if !stat.Mode().IsRegular() {
			return result, ErrUnsafe
		}
		f, err := root.Open(dest)
		if err != nil {
			return result, err
		}
		old, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
		f.Close()
		if err != nil {
			return result, err
		}
		if !bytes.Equal(old, data) {
			return result, ErrConflict
		}
		result.Duplicate = true
	}
	result.Name = filename
	return result, nil
}
