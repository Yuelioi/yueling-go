package system

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/httpclient"
)

var imageUploadEntries = []struct {
	cmd    string
	folder string
}{
	{"添加老婆", "老婆"},
	{"添加老公", "老公"},
	{"添加福瑞", "福瑞"},
	{"添加龙图", "龙图"},
	{"添加杂鱼", "杂鱼"},
	{"添加沙雕图", "沙雕图"},
	{"添加美少女", "美少女"},
	{"添加吃的", "吃的"},
	{"添加喝的", "喝的"},
	{"添加玩的", "玩的"},
	{"添加水果", "水果"},
	{"添加表情", "表情"},
	{"添加语录", "语录"},
}

func RegisterImage(b *bot.Bot) {
	for _, entry := range imageUploadEntries {
		folder := entry.folder
		b.OnCommand(entry.cmd).Handle(func(ctx *bot.CommandContext) error {
			return uploadImages(ctx, folder)
		})
	}
}

func uploadImages(ctx *bot.CommandContext, folder string) error {
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
		data, err := fetchImageBytes(imgURL)
		if err != nil {
			log.Printf("[image] fetch %s: %v", label, err)
			lines = append(lines, label+" 下载失败")
			continue
		}

		h := sha256.Sum256(data)
		hash := fmt.Sprintf("%x", h)[:16]

		if hashExistsInDir(dir, hash) {
			lines = append(lines, label+" 已收录（重复）")
			continue
		}

		ext := detectImageExt(data)
		name := buildImageFilename(folder, hash, arg, ctx.GroupID())
		if err := os.WriteFile(filepath.Join(dir, name+"."+ext), data, 0o644); err != nil {
			lines = append(lines, label+" 保存失败")
			continue
		}
		log.Printf("[image] saved %s/%s.%s", folder, name, ext)
		lines = append(lines, label+" 上传成功")
	}

	return ctx.Reply(strings.Join(lines, "\n"))
}

// buildImageFilename generates the filename (without extension) based on category.
// 语录: {groupID}_{arg}_{hash} so quotation.go's keyword search works.
// 表情: {arg}_{hash} so emoticon.go's keyword search works.
// others: just hash.
func buildImageFilename(folder, hash, arg string, groupID int64) string {
	switch folder {
	case "语录":
		if arg != "" {
			return fmt.Sprintf("%d_%s_%s", groupID, arg, hash)
		}
		return fmt.Sprintf("%d_%s", groupID, hash)
	case "表情":
		if arg != "" {
			return arg + "_" + hash
		}
		return hash
	default:
		return hash
	}
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

func fetchImageBytes(imgURL string) ([]byte, error) {
	return httpclient.Direct.GetBytes(imgURL)
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
