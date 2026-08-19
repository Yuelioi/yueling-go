// Package imageinput resolves image bytes from bot message attachments and replies.
package imageinput

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/httpclient"
)

const MaxBytes int64 = 16 * 1024 * 1024

// URLCollector is implemented by bot.GroupContext and bot.CommandContext.
type URLCollector interface {
	CollectImageURLs() []string
}

// Item is one resolved image and the user-facing name associated with it.
type Item struct {
	Data []byte
	Name string
}

// First downloads the first current-message image, or the first replied image.
func First(ctx URLCollector) ([]byte, error) {
	urls := ctx.CollectImageURLs()
	if len(urls) == 0 {
		return nil, errors.New("请附带图片，或回复一张图片后使用命令")
	}
	data, err := httpclient.GetPublicBytesLimit(urls[0], MaxBytes)
	if err != nil {
		return nil, errors.New("图片下载失败")
	}
	return data, nil
}

// ResolveAttachedInputs downloads current/replied images without avatar fallback.
func ResolveAttachedInputs(ctx URLCollector, minImages, maxImages int) ([]Item, error) {
	if maxImages <= 0 {
		return nil, nil
	}
	urls := ctx.CollectImageURLs()
	items := make([]Item, 0, min(len(urls), maxImages))
	for i, rawURL := range urls {
		if len(items) >= maxImages {
			break
		}
		data, err := httpclient.GetPublicBytesLimit(rawURL, MaxBytes)
		if err != nil {
			return nil, fmt.Errorf("图片%d下载失败", i+1)
		}
		items = append(items, Item{Data: data, Name: fmt.Sprintf("图片%d", i+1)})
	}
	if len(items) < minImages {
		return nil, fmt.Errorf("该模板需要至少 %d 张图片，请附图或回复图片", minImages)
	}
	return items, nil
}

// ResolveAvatarInputs applies the meme input policy: attachments/replied images,
// then mentioned avatars, then the sender avatar until minImages is satisfied.
func ResolveAvatarInputs(ctx *bot.CommandContext, minImages, maxImages int) ([]Item, error) {
	if maxImages <= 0 {
		return nil, nil
	}
	items, err := ResolveAttachedInputs(ctx, 0, maxImages)
	if err != nil {
		return nil, err
	}
	var mentioned []Item
	for _, target := range ctx.Message().AtTargets() {
		if len(items)+len(mentioned) >= maxImages {
			break
		}
		uid, err := strconv.ParseInt(target, 10, 64)
		if err != nil || uid == 0 {
			continue
		}
		data, err := httpclient.GetPublicBytesLimit(QQAvatarURL(uid), MaxBytes)
		if err != nil {
			continue
		}
		mentioned = append(mentioned, Item{Data: data, Name: target})
	}

	var self *Item
	loadSelf := func() *Item {
		if self != nil {
			return self
		}
		data, err := httpclient.GetPublicBytesLimit(QQAvatarURL(ctx.UserID()), MaxBytes)
		if err != nil {
			return nil
		}
		self = &Item{Data: data, Name: ctx.Nickname()}
		return self
	}

	// For a two-person template with one @ target, slots are sender then target.
	if len(items) == 0 && minImages >= 2 && len(mentioned) > 0 {
		if own := loadSelf(); own != nil {
			items = append(items, *own)
		}
	}
	items = append(items, mentioned...)
	if len(items) > maxImages {
		items = items[:maxImages]
	}
	for len(items) < minImages && len(items) < maxImages {
		own := loadSelf()
		if own == nil {
			break
		}
		items = append(items, *own)
	}
	if len(items) < minImages {
		return nil, fmt.Errorf("该模板需要至少 %d 张图片，请附图或 @用户", minImages)
	}
	return items, nil
}

func QQAvatarURL(userID int64) string {
	return fmt.Sprintf("https://q.qlogo.cn/headimg_dl?dst_uin=%d&spec=640&img_type=jpg", userID)
}
