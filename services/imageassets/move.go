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
	"strconv"
	"strings"
)

type MoveResult struct {
	Destination   string
	Copied        bool
	SourceRemoved bool
}

// Move publishes the destination before recycling the source. A failed recycle
// leaves both copies available and is reported separately instead of losing data.
func (l *Library) Move(ctx context.Context, category, id, destination, name string, groupID int64) (MoveResult, error) {
	return l.relocate(ctx, category, id, destination, name, groupID, false)
}

// Rename preserves category and quotation ownership, including across groups.
func (l *Library) Rename(ctx context.Context, category, id, name string) (MoveResult, error) {
	return l.relocate(ctx, category, id, category, name, 0, true)
}

func (l *Library) relocate(ctx context.Context, category, id, destination, name string, groupID int64, rename bool) (MoveResult, error) {
	result := MoveResult{}
	name = strings.TrimSpace(name)
	if rename && (name == "" || !safeName(name) || len(name) > 160) {
		return result, errors.New("请提供有效的新名称")
	}
	target, err := l.resolve(destination)
	if err != nil {
		return result, err
	}
	_, namer, err := l.UploadTarget(destination, name, groupID)
	if err != nil {
		return result, err
	}
	source, err := l.resolve(category)
	if err != nil {
		return result, err
	}
	f, err := l.Open(ctx, category, id)
	if err != nil {
		return result, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return result, err
	}
	// A same-category operation must not transfer a quotation to the caller's group.
	if source.Name == target.Name && target.Naming == "quotation" {
		prefix, _, ok := strings.Cut(info.Name(), "_")
		owner, e := strconv.ParseInt(prefix, 10, 64)
		if !ok || e != nil {
			return result, errors.New("无法确定语录所属群，未修改原图")
		}
		_, namer, err = l.UploadTarget(destination, name, owner)
		if err != nil {
			return result, err
		}
	}
	if rename && target.Naming == "hash" {
		namer = func(hash string) string { return name + "_" + hash }
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return result, err
	}
	if len(data) > MaxBytes {
		return result, errors.New("素材超过大小限制")
	}
	filename := namer(fmt.Sprintf("%x", sha256.Sum256(data))[:16]) + filepath.Ext(info.Name())
	if !imageName(filename) {
		return result, ErrUnsafe
	}
	if source.Name == target.Name && filename == info.Name() {
		return result, errors.New("源图片已是目标类别和名称，无需移动")
	}
	root, err := os.OpenRoot(l.root)
	if err != nil {
		return result, err
	}
	defer root.Close()
	dir := filepath.Join("images", target.Name)
	if err = root.MkdirAll(dir, 0755); err != nil {
		return result, err
	}
	for _, part := range []string{"images", dir} {
		stat, e := root.Lstat(part)
		if e != nil {
			return result, e
		}
		if !stat.IsDir() || stat.Mode()&os.ModeSymlink != 0 {
			return result, ErrUnsafe
		}
	}
	temp := filepath.Join(dir, ".move-"+rand.Text())
	out, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return result, err
	}
	defer root.Remove(temp)
	_, err = out.Write(data)
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return result, err
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	dest := filepath.Join(dir, filename)
	if err = root.Link(temp, dest); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return result, err
		}
		stat, e := root.Lstat(dest)
		if e != nil {
			return result, e
		}
		if !stat.Mode().IsRegular() {
			return result, ErrUnsafe
		}
		existing, e := root.Open(dest)
		if e != nil {
			return result, e
		}
		old, e := io.ReadAll(io.LimitReader(existing, MaxBytes+1))
		existing.Close()
		if e != nil {
			return result, e
		}
		if !bytes.Equal(old, data) {
			return result, errors.New("目标文件冲突，未修改源图片")
		}
	}
	result.Destination = filename
	result.Copied = true
	if _, err = l.Delete(ctx, category, id); err != nil {
		return result, err
	}
	result.SourceRemoved = true
	return result, nil
}
