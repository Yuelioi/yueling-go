package random

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/logx"
)

var dailyReplies = map[string][]string{
	"喝的": {"随手摇了几个出来", "看上哪个喝哪个", "难选就闭眼点一个", "这几个都不错"},
	"吃的": {"随手摇了几个出来", "看上哪个吃哪个", "难选就闭眼点一个", "这几个都不错"},
	"玩的": {"随手摇了几个出来", "看上哪个玩哪个", "难选就闭眼点一个", "这几个都不错"},
	"水果": {"随手摇了几个出来", "看上哪个吃哪个", "难选就闭眼点一个", "这几个都不错"},
}

var dailyNums = []string{"①", "②", "③", "④"}

func RegisterDaily(b *bot.Bot) {
	b.OnFullMatch("随机喝的", "喝啥", "喝什么", "来点喝的").Handle(dailyHandler("喝的"))
	b.OnFullMatch("随机吃的", "吃啥", "吃什么", "来点吃的").Handle(dailyHandler("吃的"))
	b.OnFullMatch("随机玩的", "玩啥", "玩什么", "来点玩的").Handle(dailyHandler("玩的"))
	b.OnFullMatch("随机水果", "来点水果").Handle(dailyHandler("水果"))
}

func dailyHandler(category string) func(*bot.GroupContext) error {
	replies := dailyReplies[category]
	return func(ctx *bot.GroupContext) error {
		picks, err := pickFiles(services.DataPath("images", category), 4)
		if err != nil || len(picks) == 0 {
			return ctx.Reply("暂无素材")
		}

		hint := replies[rand.Intn(len(replies))]
		var parts []string
		for i, p := range picks {
			stem := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
			parts = append(parts, fmt.Sprintf("%s %s", dailyNums[i], stem))
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "%s\n%s", hint, strings.Join(parts, "  "))

		imgData, err := buildGrid(picks)
		if err != nil {
			return ctx.Reply(sb.String())
		}
		_, err = ctx.SendGroupMsg(ctx.GroupID(), bot.Msg().Text(sb.String()+"\n").ImageBytes(imgData).Build())
		return err
	}
}

func pickFiles(folder string, n int) ([]string, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		lower := strings.ToLower(e.Name())
		if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") ||
			strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".gif") {
			paths = append(paths, filepath.Join(folder, e.Name()))
		}
	}
	rand.Shuffle(len(paths), func(i, j int) { paths[i], paths[j] = paths[j], paths[i] })
	if len(paths) > n {
		paths = paths[:n]
	}
	return paths, nil
}

func buildGrid(picks []string) ([]byte, error) {
	count := len(picks)
	const cell = 250
	cols := 2
	rows := (count + 1) / 2
	if count == 1 {
		cols, rows = 1, 1
	}

	grid := image.NewRGBA(image.Rect(0, 0, cols*cell, rows*cell))
	draw.Draw(grid, grid.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	for i, path := range picks {
		img, err := decodeImage(path)
		if err != nil {
			logx.Warnf("[daily] decode failed %s: %v", filepath.Base(path), err)
			continue
		}
		scaled := coverResize(img, cell, cell)
		col := i % cols
		row := i / cols
		dst := image.Rect(col*cell, row*cell, (col+1)*cell, (row+1)*cell)
		xdraw.BiLinear.Scale(grid, dst, scaled, scaled.Bounds(), xdraw.Over, nil)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, grid, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// coverResize crops src to the correct aspect ratio (centered) then scales to w×h RGBA.
// The intermediate RGBA buffer ensures any source color model (CMYK, Paletted…) is
// converted before the second scale step that draws to the grid.
func coverResize(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	scaleW := float64(sw) / float64(w)
	scaleH := float64(sh) / float64(h)
	scale := min(scaleW, scaleH)
	cropW := int(float64(w) * scale)
	cropH := int(float64(h) * scale)
	ox := b.Min.X + (sw-cropW)/2
	oy := b.Min.Y + (sh-cropH)/2
	srcRect := image.Rect(ox, oy, ox+cropW, oy+cropH)

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), src, srcRect, xdraw.Over, nil)
	return dst
}
