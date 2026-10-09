package image

import (
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/imageassets"
	"strings"
)

// Upload shares validation and atomic storage with AI uploads.
func Upload(ctx *bot.CommandContext, folder string, nameFn func(hash, arg string, gid int64) string) error {
	urls := ctx.CollectImageURLs()
	if len(urls) == 0 {
		return ctx.Reply("请附带图片")
	}
	arg := strings.TrimSpace(strings.Join(ctx.Args, " "))
	cfg := config.Current().Image
	result := imageassets.Save(ctx.Context(), services.DataPath("images", folder), urls, func(hash string) string { return nameFn(hash, arg, ctx.GroupID()) }, imageassets.Options{Convert: cfg.Convert, MinBytes: cfg.ConvertMinKB * 1024, Quality: cfg.ConvertQuality})
	return ctx.Reply(strings.Join(result.Lines, "\n"))
}
