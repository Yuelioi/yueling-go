package image

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"github.com/Yuelioi/yueling-go/services/logx"
)

// Upload 下载附带图片入库到 <folder>，文件名由 nameFn 决定。相同图片(hash)不重复收录。
func Upload(ctx *bot.CommandContext, folder string, nameFn func(hash, arg string, gid int64) string) error {
	urls := ctx.CollectImageURLs()
	if len(urls) == 0 {
		return ctx.Reply("请附带图片")
	}

	arg := strings.TrimSpace(strings.Join(ctx.Args, " "))
	dir := services.DataPath("images", folder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ctx.Reply("目录创建失败")
	}

	var lines []string
	for i, imgURL := range urls {
		label := fmt.Sprintf("图片%d", i+1)
		data, err := httpclient.Direct.GetBytes(imgURL)
		if err != nil {
			logx.Warnf("[image] fetch %s: %v", label, err)
			lines = append(lines, label+" 下载失败")
			continue
		}

		if config.C.Image.Convert {
			data = services.ShrinkToJPEG(data, config.C.Image.ConvertMinKB*1024, config.C.Image.ConvertQuality)
		}

		h := sha256.Sum256(data)
		hash := fmt.Sprintf("%x", h)[:16]

		if hashExistsInDir(dir, hash) {
			lines = append(lines, label+" 已收录（重复）")
			continue
		}

		ext := detectImageExt(data)
		name := nameFn(hash, arg, ctx.GroupID())
		if err := os.WriteFile(filepath.Join(dir, name+"."+ext), data, 0o644); err != nil {
			lines = append(lines, label+" 保存失败")
			continue
		}
		logx.Infof("[image] saved %s/%s.%s", folder, name, ext)
		lines = append(lines, label+" 上传成功")
	}

	return ctx.Reply(strings.Join(lines, "\n"))
}

func hashExistsInDir(dir, hash string) bool {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), hash) {
			return true
		}
	}
	return false
}

func detectImageExt(data []byte) string {
	if len(data) < 12 {
		return "jpg"
	}
	switch {
	case data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G':
		return "png"
	case data[0] == 'G' && data[1] == 'I' && data[2] == 'F':
		return "gif"
	case string(data[8:12]) == "WEBP":
		return "webp"
	default:
		return "jpg"
	}
}
