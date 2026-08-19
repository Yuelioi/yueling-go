// Package textdraw contains shared font loading helpers for local meme templates.
package textdraw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"

	"github.com/Yuelioi/yueling-go/services"
)

var fontCache = struct {
	sync.Mutex
	paths map[string]struct{}
	fonts []*opentype.Font
}{paths: map[string]struct{}{}}

// LoadCJKFont finds a locally available font that covers every required string.
func LoadCJKFont(required []string) (*opentype.Font, error) {
	fontCache.Lock()
	defer fontCache.Unlock()
	for _, parsed := range fontCache.fonts {
		if Supports(parsed, required) {
			return parsed, nil
		}
	}
	for _, path := range fontCandidates() {
		if _, loaded := fontCache.paths[path]; loaded {
			continue
		}
		fontCache.paths[path] = struct{}{}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		collection, err := opentype.ParseCollection(data)
		if err != nil {
			continue
		}
		for i := 0; i < collection.NumFonts(); i++ {
			parsed, err := collection.Font(i)
			if err != nil {
				continue
			}
			fontCache.fonts = append(fontCache.fonts, parsed)
			if Supports(parsed, required) {
				return parsed, nil
			}
		}
	}
	return nil, fmt.Errorf("未找到可用中文字体，请在 %s 放入 TTF/OTF/TTC 字体", services.DataPath("fonts"))
}

// Supports reports whether parsed contains every non-space rune in texts.
func Supports(parsed *opentype.Font, texts []string) bool {
	if parsed == nil {
		return false
	}
	var buffer sfnt.Buffer
	for _, text := range texts {
		for _, r := range text {
			if unicode.IsSpace(r) {
				continue
			}
			index, err := parsed.GlyphIndex(&buffer, r)
			if err != nil || index == 0 {
				return false
			}
		}
	}
	return true
}

func fontCandidates() []string {
	var candidates []string
	entries, _ := os.ReadDir(services.DataPath("fonts"))
	sort.SliceStable(entries, func(i, j int) bool {
		a := strings.ToLower(entries[i].Name())
		b := strings.ToLower(entries[j].Name())
		preferred := func(name string) bool {
			return strings.Contains(name, "noto") || strings.Contains(name, "sourcehan") || strings.Contains(name, "思源")
		}
		return preferred(a) && !preferred(b)
	})
	for _, entry := range entries {
		if entry.IsDir() || !isFontFile(entry.Name()) {
			continue
		}
		candidates = append(candidates, filepath.Join(services.DataPath("fonts"), entry.Name()))
	}
	candidates = append(candidates,
		services.DataPath("fortune", "fonts", "sakura.ttf"),
		`C:\Windows\Fonts\msyh.ttc`,
		`C:\Windows\Fonts\simhei.ttf`,
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
		"/System/Library/Fonts/PingFang.ttc",
	)
	return candidates
}

func isFontFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return true
	default:
		return false
	}
}
