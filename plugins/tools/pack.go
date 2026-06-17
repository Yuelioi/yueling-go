package tools

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
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
				ID      string          `json:"id"`
				Content json.RawMessage `json:"content"`
			}
			if json.Unmarshal(s.Data, &d) != nil {
				continue
			}
			// 嵌套合并转发：NapCat 把子消息内联在 data.content 里，且内层 forward 往往没有
			// 可二次查询的 id，故优先吃内联 content；没有内联才回退到 getForward(id)。
			inner := parseForwardContent(d.Content)
			if len(inner) == 0 && d.ID != "" && !visited[d.ID] {
				visited[d.ID] = true
				if fwd, err := getForward(d.ID); err == nil {
					inner = fwd
				}
			}
			for _, im := range inner {
				collectImages(im, getForward, depth+1, maxImages, visited, out)
			}
		}
	}
}

// parseForwardContent 解析 forward 段内联的 content。NapCat 的嵌套合并转发形状在不同版本
// 间有出入（节点段可能在 message / content，也可能裹在 data 下），这里对几种常见形状都容错，
// 解不出就返回 nil（调用方据此回退到 getForward(id)）。
func parseForwardContent(raw json.RawMessage) []bot.Message {
	if len(raw) == 0 {
		return nil
	}
	var nodes []struct {
		Message bot.Message `json:"message"`
		Content bot.Message `json:"content"`
		Data    struct {
			Message bot.Message `json:"message"`
			Content bot.Message `json:"content"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &nodes) != nil {
		return nil
	}
	out := make([]bot.Message, 0, len(nodes))
	for _, n := range nodes {
		switch {
		case len(n.Message) > 0:
			out = append(out, n.Message)
		case len(n.Content) > 0:
			out = append(out, n.Content)
		case len(n.Data.Message) > 0:
			out = append(out, n.Data.Message)
		case len(n.Data.Content) > 0:
			out = append(out, n.Data.Content)
		}
	}
	return out
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

// downloadItems 逐个下载 url，命名 NNN.ext，单张失败跳过。带张数/字节上限：
// 到达上限即停并标记 truncated（调用方据此提示用户还有图没打包进来）。
// get 注入便于单测。
func downloadItems(urls []string, get func(string) ([]byte, error), maxImages int, maxBytes int64) (items []packItem, total int64, truncated bool) {
	for _, u := range urls {
		if len(items) >= maxImages {
			truncated = true
			break
		}
		data, err := get(u)
		if err != nil {
			logx.Warnf("[pack] 下载失败 %s: %v", u, err)
			continue
		}
		if total+int64(len(data)) > maxBytes {
			truncated = true
			break
		}
		total += int64(len(data))
		name := fmt.Sprintf("%03d.%s", len(items)+1, detectImageExt(data))
		items = append(items, packItem{name: name, data: data})
	}
	return items, total, truncated
}

func RegisterPack(b *bot.Bot) {
	b.OnCommand("pack").Handle(func(ctx *bot.CommandContext) error {
		ctx.React(bot.EmojiProcessing)
		maxImages := config.C.Pack.MaxImages
		maxBytes := int64(config.C.Pack.MaxMB) * 1024 * 1024

		visited := map[string]bool{}
		var urls []string
		collectImages(ctx.Message(), ctx.GetForwardMsg, 0, maxImages, visited, &urls)
		if replied, ok := ctx.RepliedMessage(); ok {
			collectImages(replied, ctx.GetForwardMsg, 0, maxImages, visited, &urls)
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

		// collectImages 在 maxImages 处停止收集，故收满即说明还有图被截断
		cappedAtCollect := len(urls) >= maxImages
		items, _, cappedAtDownload := downloadItems(urls, func(u string) ([]byte, error) {
			return httpclient.Direct.GetBytesLimit(u, maxBytes)
		}, maxImages, maxBytes)
		if len(items) == 0 {
			return ctx.Reply("图片下载失败")
		}

		zipBytes, err := writeZipBytes(items)
		if err != nil {
			logx.Errorf("[pack] 打包失败: %v", err)
			return ctx.Reply("打包失败")
		}

		// NapCat 与 bot 多为独立容器/进程，不共享文件系统，且 base64:// 内联会撑爆单条 WS
		// 消息（大包触发 close 1009 断连）。故走 upload_file_stream 分片上传：协议端把分片重组
		// 落到自己本机，返回的路径再交给 upload_group_file 引用。
		ts := time.Now().Format("20060102_150405")
		name := fmt.Sprintf("图片打包_%s.zip", ts)
		filePath, err := ctx.UploadFileStream(zipBytes, name)
		if err != nil {
			logx.Errorf("[pack] 流式上传失败: %v", err)
			return ctx.Reply("上传失败")
		}
		if err := ctx.UploadGroupFile(ctx.GroupID(), filePath, name, ""); err != nil {
			logx.Errorf("[pack] 上传群文件失败: %v", err)
			return ctx.Reply("上传失败")
		}

		msg := fmt.Sprintf("已打包 %d 张图片", len(items))
		if cappedAtCollect || cappedAtDownload {
			msg += fmt.Sprintf("（已达上限：最多 %d 张 / %d MB，其余未打包）", maxImages, config.C.Pack.MaxMB)
		}
		return ctx.Reply(msg)
	})
}
