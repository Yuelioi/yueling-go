// Package avatar_meme is yueling-go's locally owned Go meme plugin.
package avatar_meme

import (
	"context"
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/templates"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/plugins/internal/imageinput"
	"github.com/Yuelioi/yueling-go/services/imaging"
)

func Register(b *bot.Bot) {
	registry, err := newRegistry(templates.All())
	if err != nil {
		panic(err)
	}
	b.OnFullMatch("月灵表情包", "自制表情包").
		Plugin(catalog.PluginAvatarMeme).
		Block().
		Handle(func(ctx *bot.GroupContext) error {
			if len(registry.templates) == 0 {
				return ctx.Reply("自制头像表情模板框架已就绪，暂未添加模板")
			}
			var lines []string
			for _, template := range registry.templates {
				spec := template.Spec()
				lines = append(lines, strings.Join(spec.Keywords, " / ")+" — "+spec.Description)
			}
			return ctx.Reply("月灵自制表情包：\n" + strings.Join(lines, "\n"))
		})

	for _, template := range registry.templates {
		template := template
		spec := template.Spec()
		b.OnCommand(spec.Keywords[0], spec.Keywords[1:]...).
			Plugin(catalog.PluginAvatarMeme).
			Block().
			Handle(func(ctx *bot.CommandContext) error {
				return renderTemplate(ctx, template)
			})
	}
}

func renderTemplate(ctx *bot.CommandContext, template model.Template) error {
	spec := template.Spec()
	ctx.React(bot.EmojiProcessing)
	var inputs []imageinput.Item
	var err error
	if spec.AllowAvatarFallback {
		inputs, err = imageinput.ResolveAvatarInputs(ctx, spec.MinImages, spec.MaxImages)
	} else {
		inputs, err = imageinput.ResolveAttachedInputs(ctx, spec.MinImages, spec.MaxImages)
	}
	if err != nil {
		return ctx.Reply(err.Error())
	}
	texts, err := resolveTexts(ctx.Args, spec)
	if err != nil {
		return ctx.Reply(err.Error())
	}

	data, err := imaging.Run(func() ([]byte, error) {
		animations := make([]*imaging.Animation, len(inputs))
		names := make([]string, len(inputs))
		for i, input := range inputs {
			animation, err := imaging.Decode(input.Data, imaging.DefaultLimits)
			if err != nil {
				return nil, fmt.Errorf("第 %d 张图片读取失败：%s", i+1, err)
			}
			animations[i] = animation
			names[i] = input.Name
		}
		result, err := template.Render(context.Background(), model.RenderRequest{
			Images:  animations,
			Names:   names,
			Texts:   texts,
			Options: map[string]string{},
		})
		if err != nil {
			return nil, err
		}
		return imaging.Encode(result, imaging.DefaultLimits)
	})
	if err != nil {
		return ctx.Reply("表情生成失败：" + err.Error())
	}
	return ctx.SendMsg(bot.Msg().ImageBytes(data).Build())
}

func resolveTexts(args []string, spec model.TemplateSpec) ([]string, error) {
	if spec.MaxTexts == 0 {
		return nil, nil
	}
	raw := strings.TrimSpace(strings.Join(args, " "))
	var texts []string
	if raw != "" {
		if spec.MaxTexts == 1 {
			texts = []string{raw}
		} else {
			for _, part := range strings.Split(raw, "|") {
				if text := strings.TrimSpace(part); text != "" {
					texts = append(texts, text)
				}
			}
		}
	}
	if len(texts) < spec.MinTexts && len(spec.DefaultTexts) >= spec.MinTexts {
		texts = append([]string(nil), spec.DefaultTexts...)
	}
	if len(texts) < spec.MinTexts {
		return nil, fmt.Errorf("该模板需要至少 %d 段文字", spec.MinTexts)
	}
	if len(texts) > spec.MaxTexts {
		return nil, fmt.Errorf("该模板最多接受 %d 段文字，多段文字请用 | 分隔", spec.MaxTexts)
	}
	return texts, nil
}
