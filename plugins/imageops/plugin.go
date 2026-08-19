// Package imageops registers stateless, in-process image processing commands.
package imageops

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/plugins/internal/imageinput"
	"github.com/Yuelioi/yueling-go/services/imaging"
)

type commandSpec struct {
	name      string
	aliases   []string
	operation imaging.Operation
}

var fixedCommands = []commandSpec{
	{"镜像", []string{"翻转", "水平翻转", "左右翻转"}, imaging.Operation{Kind: imaging.FlipHorizontal}},
	{"上下翻转", []string{"垂直翻转"}, imaging.Operation{Kind: imaging.FlipVertical}},
	{"左镜像", nil, imaging.Operation{Kind: imaging.MirrorLeft}},
	{"右镜像", nil, imaging.Operation{Kind: imaging.MirrorRight}},
	{"上镜像", nil, imaging.Operation{Kind: imaging.MirrorTop}},
	{"下镜像", nil, imaging.Operation{Kind: imaging.MirrorBottom}},
	{"灰度", []string{"黑白"}, imaging.Operation{Kind: imaging.Grayscale}},
	{"反色", []string{"负片"}, imaging.Operation{Kind: imaging.Invert}},
}

func Register(b *bot.Bot) {
	for _, spec := range fixedCommands {
		spec := spec
		b.OnCommand(spec.name, spec.aliases...).
			Plugin(catalog.PluginImageOps).
			Block().
			Handle(func(ctx *bot.CommandContext) error {
				if len(ctx.Args) != 0 {
					return ctx.Reply("该命令不需要文字参数，请附图或回复图片后直接发送「" + ctx.Cmd + "」")
				}
				return process(ctx, spec.operation)
			})
	}

	b.OnCommand("旋转").
		Plugin(catalog.PluginImageOps).
		Block().
		Handle(func(ctx *bot.CommandContext) error {
			operation, err := parseRotate(ctx.Args)
			if err != nil {
				return ctx.Reply(err.Error())
			}
			return process(ctx, operation)
		})

	b.OnCommand("缩放").
		Plugin(catalog.PluginImageOps).
		Block().
		Handle(func(ctx *bot.CommandContext) error {
			operation, err := parseResize(ctx.Args)
			if err != nil {
				return ctx.Reply(err.Error())
			}
			return process(ctx, operation)
		})
}

func process(ctx *bot.CommandContext, operation imaging.Operation) error {
	if len(ctx.CollectImageURLs()) == 0 {
		return ctx.Reply("请附带图片，或回复一张图片后使用命令")
	}
	ctx.React(bot.EmojiProcessing)
	data, err := imageinput.First(ctx)
	if err != nil {
		return ctx.Reply(err.Error())
	}
	result, err := imaging.Run(func() ([]byte, error) {
		return imaging.Process(data, operation, imaging.DefaultLimits)
	})
	if err != nil {
		return ctx.Reply("图片处理失败：" + err.Error())
	}
	return ctx.SendMsg(bot.Msg().ImageBytes(result).Build())
}

func parseRotate(args []string) (imaging.Operation, error) {
	degrees := 90
	if len(args) > 1 {
		return imaging.Operation{}, fmt.Errorf("用法：旋转 [90|180|270|-90] + 图片")
	}
	if len(args) == 1 {
		value, err := strconv.Atoi(args[0])
		if err != nil {
			return imaging.Operation{}, fmt.Errorf("旋转角度必须是 90、180、270 或 -90")
		}
		degrees = value
	}
	normalized := ((degrees % 360) + 360) % 360
	if normalized != 0 && normalized != 90 && normalized != 180 && normalized != 270 {
		return imaging.Operation{}, fmt.Errorf("首版旋转仅支持 90° 的倍数")
	}
	return imaging.Operation{Kind: imaging.Rotate, Degrees: degrees}, nil
}

func parseResize(args []string) (imaging.Operation, error) {
	if len(args) != 1 {
		return imaging.Operation{}, fmt.Errorf("用法：缩放 50%% / 缩放 512 / 缩放 512x512 + 图片")
	}
	value := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(args[0]), "×", "x"))
	if strings.HasSuffix(value, "%") {
		percent, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64)
		if err != nil || math.IsNaN(percent) || math.IsInf(percent, 0) || percent < 5 || percent > 400 {
			return imaging.Operation{}, fmt.Errorf("缩放百分比须在 5%% 到 400%% 之间")
		}
		return imaging.Operation{Kind: imaging.Resize, Scale: percent / 100}, nil
	}
	if strings.Contains(value, "x") {
		parts := strings.Split(value, "x")
		if len(parts) != 2 {
			return imaging.Operation{}, fmt.Errorf("尺寸格式应为 512x512")
		}
		width, errW := strconv.Atoi(parts[0])
		height, errH := strconv.Atoi(parts[1])
		if errW != nil || errH != nil || width <= 0 || height <= 0 {
			return imaging.Operation{}, fmt.Errorf("宽高必须是正整数")
		}
		return imaging.Operation{Kind: imaging.Resize, Width: width, Height: height}, nil
	}
	width, err := strconv.Atoi(value)
	if err != nil || width <= 0 {
		return imaging.Operation{}, fmt.Errorf("宽度必须是正整数")
	}
	return imaging.Operation{Kind: imaging.Resize, Width: width}, nil
}
