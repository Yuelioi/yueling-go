package image

import (
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"github.com/Yuelioi/yueling-go/services/logx"
)

// activeEntries 实际生效的配置表（默认表或配置覆盖），help.go 据此生成帮助。
var activeEntries []config.ImageEntry

func Register(b *bot.Bot) {
	entries := config.C.Image.Entry
	if len(entries) == 0 {
		entries = defaultEntries
	}
	if err := validateEntries(entries); err != nil {
		logx.Fatalf("[image] 配置校验失败: %v", err)
	}
	activeEntries = entries

	for _, e := range entries {
		switch kindOf(e) {
		case config.KindSingle:
			registerSingle(b, e)
		case config.KindGrid:
			registerGrid(b, e)
		case config.KindExternal:
			registerExternal(b, e)
		}
	}
}

func registerSingle(b *bot.Bot, e config.ImageEntry) {
	folder := e.Folder
	b.OnFullMatch(e.Call...).Handle(func(ctx *bot.GroupContext) error {
		path, err := services.GetRandomImage(folder, "")
		if err != nil {
			return ctx.Reply("图片不存在，请先放入素材")
		}
		return ctx.SendGroupLocalImage(ctx.GroupID(), path)
	})
	if e.Add != "" {
		b.OnCommand(e.Add).Handle(func(ctx *bot.CommandContext) error {
			return Upload(ctx, folder, nameByHash)
		})
	}
}

func registerGrid(b *bot.Bot, e config.ImageEntry) {
	folder, add := e.Folder, e.Add
	b.OnFullMatch(e.Call...).Handle(func(ctx *bot.GroupContext) error {
		return renderGrid(ctx, folder)
	})
	b.OnCommand(add).Handle(func(ctx *bot.CommandContext) error {
		if strings.TrimSpace(strings.Join(ctx.Args, "")) == "" {
			return ctx.Reply("请带上名字，如：" + add + " 麻辣烫")
		}
		return Upload(ctx, folder, nameByArg)
	})
}

func registerExternal(b *bot.Bot, e config.ImageEntry) {
	url, pick := e.URL, e.Pick
	b.OnFullMatch(e.Call...).Handle(func(ctx *bot.GroupContext) error {
		if pick == "" {
			_, err := ctx.SendGroupMsg(ctx.GroupID(), bot.Msg().Image(url).Build())
			return err
		}
		body, err := httpclient.Direct.GetBytes(url)
		if err != nil {
			logx.Warnf("[image] external GET %s: %v", url, err)
			return ctx.Reply("获取失败")
		}
		imgURL, err := ExtractImageURL(body, pick)
		if err != nil {
			logx.Warnf("[image] external pick %q: %v", pick, err)
			return ctx.Reply("解析失败")
		}
		_, err = ctx.SendGroupMsg(ctx.GroupID(), bot.Msg().Image(imgURL).Build())
		return err
	})
}
