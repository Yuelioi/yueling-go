package funny

import (
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/meme"
)

// RegisterMemes dynamically registers one command per meme key.
// Must be called after meme.Init() succeeds.
func RegisterMemes(b *bot.Bot) {
	keys := meme.AllKeys()
	if len(keys) == 0 {
		return
	}
	log.Printf("[meme] registering %d meme commands", len(keys))

	for _, key := range keys {
		key := key
		b.OnCommand(key).Handle(func(ctx *bot.CommandContext) error {
			return handleMeme(ctx, key)
		})
	}
}

func handleMeme(ctx *bot.CommandContext, key string) error {
	info := meme.GetInfo(key)
	if info == nil {
		return ctx.Reply("未知表情包：" + key)
	}

	// ── Collect image bytes ────────────────────────────────────────────────
	// Priority: images attached in message > @mentioned user avatars > sender avatar

	var imageBytes [][]byte

	// Attached images in message
	for _, imgURL := range ctx.Message().ImageURLs() {
		data, err := meme.FetchURL(imgURL)
		if err != nil {
			log.Printf("[meme] fetch attached image: %v", err)
			continue
		}
		imageBytes = append(imageBytes, data)
	}

	// @Mentioned users' avatars
	if len(imageBytes) < info.Params.MaxImages {
		for _, target := range ctx.Message().AtTargets() {
			if len(imageBytes) >= info.Params.MaxImages {
				break
			}
			// Skip @all (target == "all")
			if target == "all" {
				continue
			}
			var uid int64
			fmt.Sscan(target, &uid)
			if uid == 0 {
				continue
			}
			data, err := meme.FetchURL(meme.QQAvatarURL(uid))
			if err != nil {
				log.Printf("[meme] fetch avatar %d: %v", uid, err)
				continue
			}
			imageBytes = append(imageBytes, data)
		}
	}

	// Sender avatar as fallback when meme needs at least 1 image
	if info.Params.MinImages > 0 && len(imageBytes) == 0 {
		data, err := meme.FetchURL(meme.QQAvatarURL(ctx.UserID()))
		if err == nil {
			imageBytes = append(imageBytes, data)
		}
	}

	// For 2-image memes (self + user): prepend sender avatar if only 1 collected
	if info.Params.MinImages >= 2 && len(imageBytes) == 1 {
		data, err := meme.FetchURL(meme.QQAvatarURL(ctx.UserID()))
		if err == nil {
			imageBytes = append([][]byte{data}, imageBytes...)
		}
	}

	// Check minimum image count
	if len(imageBytes) < info.Params.MinImages {
		need := info.Params.MinImages
		hint := fmt.Sprintf("该表情包需要 %d 张图片，请 @用户 或附上图片", need)
		if need == 1 {
			hint = "请 @用户 或附上图片"
		}
		return ctx.Reply(hint)
	}
	// Clamp to max
	if info.Params.MaxImages > 0 && len(imageBytes) > info.Params.MaxImages {
		imageBytes = imageBytes[:info.Params.MaxImages]
	}

	// ── Collect text ──────────────────────────────────────────────────────
	var texts []string
	if len(ctx.Args) > 0 {
		// Each arg is one text item; join all as single text if meme takes 1 text
		joined := strings.Join(ctx.Args, " ")
		if info.Params.MaxTexts == 1 {
			texts = []string{joined}
		} else {
			texts = ctx.Args
		}
	}
	if info.Params.MinTexts > 0 && len(texts) < info.Params.MinTexts {
		return ctx.Reply(fmt.Sprintf("该表情包需要至少 %d 段文字", info.Params.MinTexts))
	}
	if info.Params.MaxTexts > 0 && len(texts) > info.Params.MaxTexts {
		texts = texts[:info.Params.MaxTexts]
	}

	// ── Generate ──────────────────────────────────────────────────────────
	data, _, err := meme.Generate(key, imageBytes, texts, nil)
	if err != nil {
		log.Printf("[meme] generate %s: %v", key, err)
		return ctx.Reply("生成失败：" + err.Error())
	}

	encoded := "base64://" + base64.StdEncoding.EncodeToString(data)
	_, err = ctx.SendGroupMsg(ctx.GroupID(), bot.Msg().Image(encoded).Build())
	return err
}
