package imageassets

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

var (
	ErrForbidden = errors.New("只有机器人超管可以管理本地图片；群主和群管理员不具备此权限")
	ErrExternal  = errors.New("这是外部网站图片类目，不支持本地素材查询或删除")
	ErrCategory  = errors.New("图片类目不存在或名称有歧义，请先查看图片列表中的类目")
	ErrAsset     = errors.New("素材编号不存在或已变化，请重新查询图片列表；不能按网址、路径或模糊名字删除")
	ErrUnsafe    = errors.New("素材目录或文件不安全，不能处理符号链接")
)

// Category describes the actual configured source, not the appearance of a URL.
// Even an external category with a Folder is never locally manageable.
type Category struct {
	Name     string
	Aliases  []string
	External bool
}
type Asset struct{ ID, Name string }
type Page struct {
	Category      string
	Assets        []Asset
	Total, Number int
}

const PageSize = 20

type Library struct {
	root       string
	categories []Category
	superUsers []int64
}

func NewLibrary(root string, categories []Category, superUsers []int64) *Library {
	return &Library{root: root, categories: categories, superUsers: append([]int64(nil), superUsers...)}
}
func (l *Library) authorize(userID int64) error {
	if !slices.Contains(l.superUsers, userID) {
		return ErrForbidden
	}
	return nil
}
func (l *Library) Categories(userID int64) ([]Category, error) {
	if err := l.authorize(userID); err != nil {
		return nil, err
	}
	return append([]Category(nil), l.categories...), nil
}
func (l *Library) resolve(name string) (Category, error) {
	var found *Category
	for _, c := range l.categories {
		if c.Name != name && !slices.Contains(c.Aliases, name) {
			continue
		}
		if found != nil && (found.Name != c.Name || found.External != c.External) {
			return Category{}, ErrCategory
		}
		copy := c
		found = &copy
	}
	if found == nil {
		return Category{}, ErrCategory
	}
	if found.External {
		return Category{}, ErrExternal
	}
	if !safeName(found.Name) {
		return Category{}, ErrUnsafe
	}
	return *found, nil
}

// SplitQuery supports category names/aliases containing spaces, preferring the
// longest complete category prefix. Authorization occurs before exposing names.
func (l *Library) SplitQuery(userID int64, text string) (category, query string, err error) {
	if err = l.authorize(userID); err != nil {
		return
	}
	for _, c := range l.categories {
		for _, name := range append([]string{c.Name}, c.Aliases...) {
			if name != "" && (text == name || strings.HasPrefix(text, name+" ")) && len(name) > len(category) {
				category = name
				query = strings.TrimSpace(strings.TrimPrefix(text, name))
			}
		}
	}
	if category == "" {
		err = ErrCategory
	}
	return
}
func safeName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name && !strings.ContainsAny(name, "/\\\x00\r\n")
}
func imageName(name string) bool {
	if !safeName(name) || strings.HasPrefix(name, ".") {
		return false
	}
	return slices.Contains([]string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".avif"}, strings.ToLower(filepath.Ext(name)))
}
func assetID(category string, info os.FileInfo) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d\x00%d", category, info.Name(), info.Size(), info.ModTime().UnixNano())))
	return fmt.Sprintf("%x", sum[:16])
}
func (l *Library) open(category string) (*os.Root, string, error) {
	root, err := os.OpenRoot(l.root)
	if err != nil {
		return nil, "", err
	}
	dir := filepath.Join("images", category)
	for _, path := range []string{"images", dir} {
		info, e := root.Lstat(path)
		if e != nil {
			root.Close()
			return nil, "", e
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			root.Close()
			return nil, "", ErrUnsafe
		}
	}
	return root, dir, nil
}
func assets(ctx context.Context, root *os.Root, dir, category, query string) ([]Asset, error) {
	f, err := root.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	var items []Asset
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !imageName(entry.Name()) || !strings.Contains(entry.Name(), query) {
			continue
		}
		info, err := root.Lstat(filepath.Join(dir, entry.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		items = append(items, Asset{ID: assetID(category, info), Name: entry.Name()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}
func (l *Library) List(ctx context.Context, userID int64, category, query string, page int) (Page, error) {
	if err := l.authorize(userID); err != nil {
		return Page{}, err
	}
	c, err := l.resolve(category)
	if err != nil {
		return Page{}, err
	}
	if page < 1 || page > 1000000 {
		return Page{}, errors.New("页码必须在1至1000000之间")
	}
	if err = ctx.Err(); err != nil {
		return Page{}, err
	}
	result := Page{Category: c.Name, Number: page}
	root, dir, err := l.open(c.Name)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return Page{}, err
	}
	defer root.Close()
	items, err := assets(ctx, root, dir, c.Name, query)
	if err != nil {
		return Page{}, err
	}
	result.Total = len(items)
	start := (page - 1) * PageSize
	if start < len(items) {
		result.Assets = items[start:min(start+PageSize, len(items))]
	}
	return result, nil
}

// Delete removes one exact local asset from circulation by moving it to a
// recoverable trash directory. All paths stay anchored under the data root.
func (l *Library) Delete(ctx context.Context, userID int64, category, id string) (string, error) {
	if err := l.authorize(userID); err != nil {
		return "", err
	}
	c, err := l.resolve(category)
	if err != nil {
		return "", err
	}
	if len(id) != 32 {
		return "", ErrAsset
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	root, dir, err := l.open(c.Name)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrAsset
	}
	if err != nil {
		return "", err
	}
	defer root.Close()
	items, err := assets(ctx, root, dir, c.Name, "")
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.ID != id {
			continue
		}
		source := filepath.Join(dir, item.Name)
		info, err := root.Lstat(source)
		if err != nil || !info.Mode().IsRegular() || assetID(c.Name, info) != id {
			return "", ErrAsset
		}
		if err = ctx.Err(); err != nil {
			return "", err
		}
		trash := filepath.Join("trash", "images", c.Name)
		if err = root.MkdirAll(trash, 0700); err != nil {
			return "", err
		}
		for _, path := range []string{"trash", filepath.Join("trash", "images"), trash} {
			info, e := root.Lstat(path)
			if e != nil {
				return "", e
			}
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return "", ErrUnsafe
			}
		}
		destination := filepath.Join(trash, rand.Text())
		if err = root.Mkdir(destination, 0700); err != nil {
			return "", err
		}
		defer root.Remove(destination) // succeeds only if empty (failed/canceled move)
		if err = ctx.Err(); err != nil {
			return "", err
		}
		if err = root.Rename(source, filepath.Join(destination, item.Name)); err != nil {
			return "", err
		}
		return item.Name, nil
	}
	return "", ErrAsset
}
