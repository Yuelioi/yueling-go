package tools

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"github.com/Yuelioi/yueling-go/services/logx"
)

const packMaxDepth = 5

// collectImages 递归收集一条消息里的图片 url（含展开的合并转发）。
// getForward 注入以便单测；maxImages 张数上限；visited 记已展开 forward id 防循环。
func collectImages(msg bot.Message, getForward func(string) ([]bot.Message, error), depth, maxImages int, visited map[string]bool, out *[]string) {
	if depth > packMaxDepth {
		return
	}
	for _, s := range msg {
		if len(*out) >= maxImages {
			return
		}
		switch s.Type {
		case "image":
			var d struct {
				File string `json:"file"`
				URL  string `json:"url"`
			}
			if json.Unmarshal(s.Data, &d) == nil {
				if d.URL != "" {
					*out = append(*out, d.URL)
				} else if d.File != "" {
					*out = append(*out, d.File)
				}
			}
		case "forward":
			var d struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(s.Data, &d) == nil && d.ID != "" && !visited[d.ID] {
				visited[d.ID] = true
				if inner, err := getForward(d.ID); err == nil {
					for _, im := range inner {
						collectImages(im, getForward, depth+1, maxImages, visited, out)
					}
				}
			}
		}
	}
}

type packItem struct {
	name string
	data []byte
}

func writeZipBytes(items []packItem) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, it := range items {
		w, err := zw.Create(it.name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(it.data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

// downloadItems 逐个下载 url，命名 NNN.ext，带张数/字节上限，单张失败跳过。
// get 注入便于单测。
func downloadItems(urls []string, get func(string) ([]byte, error), maxImages int, maxBytes int64) ([]packItem, int64) {
	var items []packItem
	var total int64
	for _, u := range urls {
		if len(items) >= maxImages || total >= maxBytes {
			break
		}
		data, err := get(u)
		if err != nil {
			logx.Warnf("[pack] 下载失败 %s: %v", u, err)
			continue
		}
		total += int64(len(data))
		name := fmt.Sprintf("%03d.%s", len(items)+1, detectImageExt(data))
		items = append(items, packItem{name: name, data: data})
	}
	return items, total
}

func RegisterPack(b *bot.Bot) {
	b.OnCommand("pack").Handle(func(ctx *bot.CommandContext) error {
		maxImages := config.C.Pack.MaxImages
		maxBytes := int64(config.C.Pack.MaxMB) * 1024 * 1024

		visited := map[string]bool{}
		var urls []string
		collectImages(ctx.Message(), ctx.GetForwardMsg, 0, maxImages, visited, &urls)
		if replyID, ok := ctx.Message().ReplyID(); ok {
			var mid int32
			fmt.Sscan(replyID, &mid)
			if mid != 0 {
				if replied, err := ctx.GetMsg(mid); err == nil {
					collectImages(replied, ctx.GetForwardMsg, 0, maxImages, visited, &urls)
				}
			}
		}

		seen := map[string]bool{}
		uniq := urls[:0]
		for _, u := range urls {
			if !seen[u] {
				seen[u] = true
				uniq = append(uniq, u)
			}
		}
		urls = uniq

		if len(urls) == 0 {
			return ctx.Reply("未找到可打包的图片")
		}

		items, _ := downloadItems(urls, func(u string) ([]byte, error) {
			return httpclient.Direct.GetBytes(u)
		}, maxImages, maxBytes)
		if len(items) == 0 {
			return ctx.Reply("图片下载失败")
		}

		zipBytes, err := writeZipBytes(items)
		if err != nil {
			logx.Errorf("[pack] 打包失败: %v", err)
			return ctx.Reply("打包失败")
		}

		dir := services.DataPath("tmp")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return ctx.Reply("打包失败")
		}
		ts := time.Now().Format("20060102_150405")
		zipPath := filepath.Join(dir, fmt.Sprintf("pack_%d_%s.zip", ctx.GroupID(), ts))
		if err := os.WriteFile(zipPath, zipBytes, 0o644); err != nil {
			logx.Errorf("[pack] 写临时文件失败: %v", err)
			return ctx.Reply("打包失败")
		}
		defer os.Remove(zipPath)

		if err := ctx.UploadGroupFile(ctx.GroupID(), zipPath, fmt.Sprintf("图片打包_%s.zip", ts), ""); err != nil {
			logx.Errorf("[pack] 上传群文件失败: %v", err)
			return ctx.Reply("上传失败")
		}

		msg := fmt.Sprintf("已打包 %d 张图片", len(items))
		if len(items) >= maxImages {
			msg += fmt.Sprintf("（已达上限 %d 张）", maxImages)
		}
		return ctx.Reply(msg)
	})
}
