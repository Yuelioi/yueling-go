package random

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services"
)

var dailyReplies = map[string][]string{
	"喝的": {"今天喝这个！", "月灵推荐喝「这个」~", "喝这个准没错！", "就决定是你了！"},
	"吃的": {"今天吃这个！", "月灵觉得这个不错哟~", "吃这个！冲！", "就决定是你了！"},
	"玩的": {"今天玩这个！", "月灵推荐「这个」~", "玩这个准没错！", "就决定是你了！"},
	"水果": {"今天吃这个水果！", "月灵推荐这个~", "吃这个补维生素！", "就决定是你了！"},
}

func RegisterDaily(b *bot.Bot) {
	b.OnFullMatch("随机喝的", "喝啥", "喝什么", "来点喝的").Handle(dailyHandler("喝的"))
	b.OnFullMatch("随机吃的", "吃啥", "吃什么", "来点吃的").Handle(dailyHandler("吃的"))
	b.OnFullMatch("随机玩的", "玩啥", "玩什么", "来点玩的").Handle(dailyHandler("玩的"))
	b.OnFullMatch("随机水果", "来点水果").Handle(dailyHandler("水果"))
}

func dailyHandler(category string) func(*bot.GroupContext) error {
	replies := dailyReplies[category]
	return func(ctx *bot.GroupContext) error {
		imgData, err := buildGrid(services.DataPath("images", category))
		if err != nil {
			return ctx.Reply("图片加载失败：" + err.Error())
		}
		hint := replies[rand.Intn(len(replies))]
		encoded := "base64://" + base64.StdEncoding.EncodeToString(imgData)
		_, err = ctx.SendGroupMsg(ctx.GroupID(), bot.Msg().Text(hint+"\n").Image(encoded).Build())
		return err
	}
}

func buildGrid(folder string) ([]byte, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}

	var valid []string
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") ||
			strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".gif") {
			valid = append(valid, filepath.Join(folder, e.Name()))
		}
	}
	if len(valid) == 0 {
		return nil, nil
	}

	// Pick up to 4 random images
	rand.Shuffle(len(valid), func(i, j int) { valid[i], valid[j] = valid[j], valid[i] })
	count := 4
	if len(valid) < count {
		count = len(valid)
	}
	picks := valid[:count]

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
			continue
		}
		scaled := resizeTo(img, cell, cell)
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

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".gif":
		g, err := gif.DecodeAll(f)
		if err != nil {
			return nil, err
		}
		return g.Image[0], nil
	case ".png":
		return png.Decode(f)
	default:
		return jpeg.Decode(f)
	}
}

func resizeTo(src image.Image, w, h int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}
