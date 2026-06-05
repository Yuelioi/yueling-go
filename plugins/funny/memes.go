package funny

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/logx"
	"github.com/Yuelioi/yueling-go/services/meme"
)

// RegisterMemes registers one OnCommand handler per meme keyword.
// Must be called after meme.Init() succeeds.
func RegisterMemes(b *bot.Bot) {
	keywords := meme.AllKeywords()
	if len(keywords) == 0 {
		return
	}
	logx.Infof("[meme] registering %d keyword triggers", len(keywords))

	for _, kw := range keywords {
		kw := kw
		b.OnCommand(kw).Handle(func(ctx *bot.CommandContext) error {
			return handleMeme(ctx, kw)
		})
	}

	b.OnCommand("随机表情").Handle(func(ctx *bot.CommandContext) error {
		return handleRandomMeme(ctx)
	})

	b.OnCommand("表情详情", "表情帮助", "表情示例").Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("用法：表情详情 <关键词>，如：表情详情 摸摸")
		}
		kw := strings.Join(ctx.Args, " ")
		info := meme.GetInfoByKeyword(kw)
		if info == nil {
			return ctx.Reply("未找到表情「" + kw + "」，发送「头像表情包」查看列表")
		}

		// Build info text
		imageNum := fmt.Sprintf("%d", info.Params.MinImages)
		if info.Params.MaxImages > info.Params.MinImages {
			imageNum += fmt.Sprintf(" ~ %d", info.Params.MaxImages)
		}
		textNum := fmt.Sprintf("%d", info.Params.MinTexts)
		if info.Params.MaxTexts > info.Params.MinTexts {
			textNum += fmt.Sprintf(" ~ %d", info.Params.MaxTexts)
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("【%s】\n", kw))
		sb.WriteString("关键词：" + strings.Join(info.Keywords, " / ") + "\n")
		sb.WriteString("图片数：" + imageNum + "\n")
		sb.WriteString("文字数：" + textNum)
		if len(info.Params.DefaultTexts) > 0 {
			sb.WriteString("\n默认文字：" + strings.Join(info.Params.DefaultTexts, " / "))
		}

		// Fetch preview image
		imgData, err := meme.GetPreview(info.Key)
		if err != nil {
			logx.Warnf("[meme] preview %s: %v", info.Key, err)
			return ctx.Reply(sb.String())
		}
		_, err = ctx.SendGroupMsg(ctx.GroupID(), bot.Msg().Text(sb.String()+"\n").ImageBytes(imgData).Build())
		return err
	})

	b.OnFullMatch("头像表情包").Handle(func(ctx *bot.GroupContext) error {
		data, err := meme.RenderList(meme.TextOnlyKeys())
		if err != nil {
			return ctx.Reply("获取失败：" + err.Error())
		}
		_, err = ctx.SendGroupMsg(ctx.GroupID(), bot.Msg().ImageBytes(data).Build())
		return err
	})
}

func handleRandomMeme(ctx *bot.CommandContext) error {
	info := meme.RandomEligible()
	if info == nil {
		return ctx.Reply("没有合适的表情模板")
	}

	// Fetch sender avatar (slot 0 = self)
	selfData, err := meme.FetchURL(meme.QQAvatarURL(ctx.UserID()))
	if err != nil {
		return ctx.Reply("获取头像失败：" + err.Error())
	}

	// Fetch first @mentioned avatar (slot 1+ = target)
	var targetData []byte
	for _, target := range ctx.Message().AtTargets() {
		var uid int64
		fmt.Sscan(target, &uid)
		if uid == 0 {
			continue
		}
		data, err := meme.FetchURL(meme.QQAvatarURL(uid))
		if err != nil {
			continue
		}
		targetData = data
		break
	}

	// Fill exactly MinImages slots.
	// 1-image meme: use target if @, otherwise self.
	// multi-image meme: slot 0 = self, slot 1+ = target (or self if no @).
	imageBytes := make([][]byte, info.Params.MinImages)
	for i := range imageBytes {
		useTarget := targetData != nil && (info.Params.MinImages == 1 || i > 0)
		if useTarget {
			imageBytes[i] = targetData
		} else {
			imageBytes[i] = selfData
		}
	}

	texts := info.Params.DefaultTexts
	if texts == nil {
		texts = []string{}
	}

	data, _, err := meme.Generate(info.Key, imageBytes, texts, nil)
	if err != nil {
		logx.Warnf("[meme] random %s: %v", info.Key, err)
		return ctx.Reply("生成失败：" + err.Error())
	}

	keyword := info.Keywords[0]
	_, err = ctx.SendGroupMsg(ctx.GroupID(), bot.Msg().Text("【"+keyword+"】\n").ImageBytes(data).Build())
	return err
}

func handleMeme(ctx *bot.CommandContext, keyword string) error {
	info := meme.GetInfoByKeyword(keyword)
	if info == nil {
		return nil
	}

	// ── Collect images ────────────────────────────────────────────────────────
	// Priority: attached/reply images > @mentioned avatars > sender avatar
	var imageBytes [][]byte

	for _, imgURL := range ctx.CollectImageURLs() {
		data, err := meme.FetchURL(imgURL)
		if err != nil {
			logx.Warnf("[meme] fetch image: %v", err)
			continue
		}
		imageBytes = append(imageBytes, data)
		if info.Params.MaxImages > 0 && len(imageBytes) >= info.Params.MaxImages {
			break
		}
	}

	if info.Params.MaxImages == 0 || len(imageBytes) < info.Params.MaxImages {
		for _, target := range ctx.Message().AtTargets() {
			if info.Params.MaxImages > 0 && len(imageBytes) >= info.Params.MaxImages {
				break
			}
			var uid int64
			fmt.Sscan(target, &uid)
			if uid == 0 {
				continue
			}
			data, err := meme.FetchURL(meme.QQAvatarURL(uid))
			if err != nil {
				logx.Warnf("[meme] fetch avatar %d: %v", uid, err)
				continue
			}
			imageBytes = append(imageBytes, data)
		}
	}

	// For memes needing 1+ images with none yet, use sender avatar
	if info.Params.MinImages > 0 && len(imageBytes) == 0 {
		if data, err := meme.FetchURL(meme.QQAvatarURL(ctx.UserID())); err == nil {
			imageBytes = append(imageBytes, data)
		}
	}

	// For 2-image memes where only 1 collected (@user given but not self):
	// prepend sender avatar as "self"
	if info.Params.MinImages >= 2 && len(imageBytes) == 1 {
		if data, err := meme.FetchURL(meme.QQAvatarURL(ctx.UserID())); err == nil {
			imageBytes = append([][]byte{data}, imageBytes...)
		}
	}

	if len(imageBytes) < info.Params.MinImages {
		if info.Params.MinImages == 1 {
			return ctx.Reply("请附上图片或 @某人")
		}
		return ctx.Reply(fmt.Sprintf("该表情需要 %d 张图片，请附上图片或 @用户", info.Params.MinImages))
	}
	if info.Params.MaxImages > 0 && len(imageBytes) > info.Params.MaxImages {
		imageBytes = imageBytes[:info.Params.MaxImages]
	}

	// ── Collect texts ─────────────────────────────────────────────────────────
	var texts []string
	if len(ctx.Args) > 0 {
		raw := strings.Join(ctx.Args, " ")
		if info.Params.MaxTexts == 1 {
			texts = []string{raw}
		} else {
			texts = ctx.Args
		}
	}
	// Fall back to default_texts when args absent but text is required
	if len(texts) < info.Params.MinTexts && len(info.Params.DefaultTexts) >= info.Params.MinTexts {
		texts = info.Params.DefaultTexts
	}
	if len(texts) < info.Params.MinTexts {
		return ctx.Reply(fmt.Sprintf("该表情需要至少 %d 段文字", info.Params.MinTexts))
	}
	if info.Params.MaxTexts > 0 && len(texts) > info.Params.MaxTexts {
		texts = texts[:info.Params.MaxTexts]
	}

	// ── Generate ──────────────────────────────────────────────────────────────
	data, _, err := meme.Generate(info.Key, imageBytes, texts, nil)
	if err != nil {
		logx.Warnf("[meme] generate %s (%s): %v", keyword, info.Key, err)
		return ctx.Reply("生成失败：" + err.Error())
	}

	_, err = ctx.SendGroupMsg(ctx.GroupID(), bot.Msg().ImageBytes(data).Build())
	return err
}
