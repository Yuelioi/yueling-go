package tools

import (
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/imageassets"
	"github.com/Yuelioi/yueling-go/services/llm"
)

func (t *toolset) registerImageGeneration() {
	for _, edit := range []bool{false, true} {
		name, description, feature := "generate_image", "根据用户明确提出的纯文字绘图要求生成一张图片并发到当前群。用户要求参考附图或引用图片重绘、换风格时使用 edit_image；不要用于查找现有图片。", "image_generation"
		params := []ai.Param{{Name: "prompt", Type: "string", Required: true, Description: "完整画面要求，保留用户指定的风格、主体和修改限制"}}
		if edit {
			name = "edit_image"
			description = "根据附件、引用图片或QQ头像生成新图，包括参考原图重绘、换风格、修改内容。参考头像时先用get_user_avatar取得图片编号。用户说‘照这张生图，风格是xxx’也使用此工具，将原图和风格要求一起提交；不会覆盖原素材。"
			feature = "image_edit"
			params = append(params, ai.Param{Name: "image_id", Type: "string", Required: true, Description: "附件、引用图片或get_user_avatar返回的图片编号"})
		}
		var requires []string
		if edit {
			requires = []string{"get_user_avatar"}
		}
		t.registry.Register(ai.ToolMeta{Name: name, Description: description, Permission: ai.PermMember, Requires: requires, Triggers: []string{"画一张", "生成图片", "生图", "画个", "改图", "编辑图片", "重绘", "换风格"}, Params: params, ResultHandler: func(c *ai.ToolContext) ai.ToolResult {
			cfg := config.Current().AI
			if !cfg.Available(feature) {
				return outcome(ai.ToolRejected, "尚未配置对应的图片模型", false)
			}
			prompt := strings.TrimSpace(c.String("prompt"))
			if prompt == "" || len(prompt) > 16000 {
				return outcome(ai.ToolRejected, "图片描述为空或过长", false)
			}
			var sourceURL string
			if edit {
				urls, err := c.ImageURLs([]string{c.String("image_id")})
				if err != nil {
					return outcome(ai.ToolRejected, "参考图不可用或已过期，请重新获取头像或附图。", false)
				}
				sourceURL = urls[0]
			}
			c.React(bot.EmojiProcessing)
			var source []byte
			if edit {
				var err error
				source, err = imageassets.Fetch(c.Context(), sourceURL)
				if err != nil {
					return outcome(ai.ToolFailed, "原图下载失败", false)
				}
			}
			data, err := llm.New(llm.ForFeature(cfg, feature)).Image(c.Context(), prompt, source)
			if err != nil {
				return outcome(ai.ToolFailed, llm.UserMessage(err)+" 图片请求可能已计费，请勿自动重试。", true)
			}
			_, err = c.BotAPI().SendGroupMsg(c.GroupID(), bot.Msg().Reply(c.MessageID()).ImageBytes(data).Build())
			if err != nil {
				return outcome(ai.ToolFailed, "图片已生成，但发送未确认成功，请勿自动重复生成。", true)
			}
			return outcome(ai.ToolSucceeded, "生成的图片已发送到当前群。", true)
		}})
	}
}
