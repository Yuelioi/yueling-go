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
	"unicode"
)

var (
	ErrExternal = errors.New("这是外部网站图片类目，不支持本地素材查询或删除")
	ErrCategory = errors.New("图片类目不存在或名称有歧义，请刷新类目列表")
	ErrAsset    = errors.New("素材编号不存在或已变化，请刷新素材列表；不能按网址、路径或模糊名字删除")
	ErrUnsafe   = errors.New("素材目录或文件不安全，不能处理符号链接")
)

// Category describes the actual configured source, not the appearance of a URL.
// Even an external category with a Folder is never locally manageable.
type Category struct {
	Name         string   `json:"name"`
	Aliases      []string `json:"aliases"`
	External     bool     `json:"external"`
	Upload       bool     `json:"upload"`
	NameRequired bool     `json:"name_required"`
	Naming       string   `json:"naming,omitempty"`
}
type Asset struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Page struct {
	Category string  `json:"category"`
	Assets   []Asset `json:"assets"`
	Total    int     `json:"total"`
	Number   int     `json:"page"`
}

const PageSize = 20

type Library struct {
	root       string
	categories []Category
}

func NewLibrary(root string, categories []Category) *Library {
	return &Library{root: root, categories: categories}
}

// Categories returns the configured sources available to authorized callers.
func (l *Library) Categories() []Category { return append([]Category(nil), l.categories...) }
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
func (l *Library) List(ctx context.Context, category, query string, page int) (Page, error) {
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
	result := Page{Category: c.Name, Number: page, Assets: []Asset{}}
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
func (l *Library) Delete(ctx context.Context, category, id string) (string, error) {
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

// Open returns an already-open local file. Authorization belongs to the WebUI
// session middleware, and no path supplied by the caller is opened directly.
func (l *Library) Open(ctx context.Context, category, id string) (*os.File, error) {
	c, err := l.resolve(category)
	if err != nil {
		return nil, err
	}
	if len(id) != 32 {
		return nil, ErrAsset
	}
	root, dir, err := l.open(c.Name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrAsset
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	items, err := assets(ctx, root, dir, c.Name, "")
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID != id {
			continue
		}
		path := filepath.Join(dir, item.Name)
		info, err := root.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, ErrAsset
		}
		f, err := root.Open(path)
		if err != nil {
			return nil, err
		}
		current, err := f.Stat()
		if err != nil || !current.Mode().IsRegular() || assetID(c.Name, current) != id || current.Size() > MaxBytes {
			f.Close()
			return nil, ErrAsset
		}
		if err = ctx.Err(); err != nil {
			f.Close()
			return nil, err
		}
		return f, nil
	}
	return nil, ErrAsset
}

func (l *Library) UploadTarget(category, name string, groupID int64) (string, func(string) string, error) {
	c, err := l.resolve(category)
	if err != nil {
		return "", nil, err
	}
	if !c.Upload {
		return "", nil, errors.New("该类目未配置添加功能")
	}
	name = strings.TrimSpace(name)
	if (c.NameRequired && name == "") || (name != "" && (!safeName(name) || len(name) > 160)) {
		return "", nil, errors.New("请提供该类目要求的有效名称")
	}
	if c.Naming == "label" {
		name = AssetLabel(name)
		if name == "" {
			return "", nil, errors.New("名称没有可用字符")
		}
	}
	filename := func(hash string) string {
		switch c.Naming {
		case "quotation":
			return fmt.Sprintf("%d_%s_%s", groupID, name, hash)
		case "label", "optional_label":
			if name != "" {
				return name + "_" + hash
			}
		}
		return hash
	}
	return filepath.Join(l.root, "images", c.Name), filename, nil
}

func AssetLabel(value string) string {
	var builder strings.Builder
	count := 0
	for _, r := range strings.TrimSpace(value) {
		if count >= 64 {
			break
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '_' || r == ' ' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('_')
		}
		count++
	}
	return strings.Trim(builder.String(), " ._-")
}
