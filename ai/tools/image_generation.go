package tools

import (
	"encoding/json"
	"errors"
	"fmt"
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
		params := []ai.Param{
			{Name: "prompt", Type: "string", Required: true, Description: "完整画面要求，保留用户指定的风格、主体和修改限制"},
			{Name: "transparent_background", Type: "boolean", Description: "用户要求抠图、去背景、透明底时必须为true；否则省略。结果检查真实透明通道，不接受画出来的棋盘格。"},
		}
		if edit {
			name = "edit_image"
			description = "结合1至3张附件、引用图片或QQ头像生成新图、重绘或修改内容。头像先用get_user_avatar取编号，也计入3张上限；可与三视图一起参考。用image_ids按顺序提交全部选定图片，prompt说明各图参考用途；超过3张先让用户选择，不得自行丢图。不会覆盖原素材。支持单个GIF逐帧修改，可配静态参考图；最多12帧/1200万像素，每帧可能计费且重绘可能闪烁。基础变换用transform_image。"
			feature = "image_edit"
			params = append(params,
				ai.Param{Name: "image_ids", Type: "array", ItemsType: "string", Description: "1至3个不同图片编号，按参考用途顺序排列；包括头像，优先使用此参数"},
				ai.Param{Name: "image_id", Type: "string", Description: "单张参考图编号，与image_ids只能选一种"},
			)
		}
		var requires []string
		var actionKey ai.ToolActionKey
		if edit {
			requires = []string{"get_user_avatar", "transform_image"}
			actionKey = imageEditActionKey
		}
		t.registry.Register(ai.ToolMeta{Name: name, Description: description, Permission: ai.PermMember, Requires: requires, ActionKey: actionKey, Triggers: []string{"画一张", "生成图片", "生图", "画个", "改图", "编辑图片", "重绘", "换风格", "抠图", "去背景", "透明底", "GIF", "gif"}, Params: params, ResultHandler: func(c *ai.ToolContext) ai.ToolResult {
			cfg := config.Current().AI
			if !cfg.Available(feature) {
				return outcome(ai.ToolRejected, "尚未配置对应的图片模型", false)
			}
			prompt := strings.TrimSpace(c.String("prompt"))
			if prompt == "" || len(prompt) > 16000 {
				return outcome(ai.ToolRejected, "图片描述为空或过长", false)
			}
			var sourceURLs []string
			if edit {
				ids, err := imageEditReferences(c.Params)
				if err != nil {
					return outcome(ai.ToolRejected, err.Error(), false)
				}
				urls, err := c.ImageURLs(ids)
				if err != nil {
					return outcome(ai.ToolRejected, "参考图不可用或已过期，请重新获取头像或附图。", false)
				}
				sourceURLs = urls
				if len(ids) > 1 {
					prompt += "\n参考图按以下编号顺序提交：" + strings.Join(ids, "、") + "。请按用户指定的各图用途结合这些参考生成新图。"
				}
			}
			c.React(bot.EmojiProcessing)
			var sources [][]byte
			for _, url := range sourceURLs {
				source, err := imageassets.Fetch(c.Context(), url)
				if err != nil {
					return outcome(ai.ToolFailed, "原图下载失败", false)
				}
				sources = append(sources, source)
			}
			client := llm.New(llm.ForFeature(cfg, feature))
			opts := llm.ImageOptions{TransparentBackground: c.Bool("transparent_background")}
			var data []byte
			var err error
			if edit {
				data, err = client.EditImages(c.Context(), prompt, sources, opts)
			} else {
				data, err = client.Image(c.Context(), prompt, nil, opts)
			}
			if err != nil {
				var failure *llm.Error
				if errors.As(err, &failure) && failure.Kind == llm.Configuration {
					return outcome(ai.ToolRejected, llm.UserMessage(err), false)
				}
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

func imageEditReferences(params map[string]any) ([]string, error) {
	_, single := params["image_id"]
	raw, multiple := params["image_ids"]
	if single && multiple {
		return nil, fmt.Errorf("image_id与image_ids只能选一种，参考图总数最多3张，头像也计入")
	}
	var ids []string
	if single {
		id, _ := params["image_id"].(string)
		ids = []string{id}
	} else if multiple {
		values, ok := raw.([]any)
		if !ok {
			return nil, fmt.Errorf("请用image_ids提供1至3个图片编号")
		}
		for _, value := range values {
			id, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("参考图必须是当前会话的图片编号")
			}
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 || len(ids) > llm.MaxImageReferences {
		return nil, fmt.Errorf("请提供1至3张参考图，头像也计入；超过3张请先选择，不能自动丢图")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if strings.TrimSpace(id) == "" || seen[id] {
			return nil, fmt.Errorf("参考图编号不能为空或重复，请选择1至3张不同图片")
		}
		seen[id] = true
	}
	return ids, nil
}

func imageEditActionKey(params map[string]any) string {
	ids, err := imageEditReferences(params)
	if err != nil {
		return ""
	}
	prompt, _ := params["prompt"].(string)
	transparent, _ := params["transparent_background"].(bool)
	data, _ := json.Marshal(map[string]any{"image_ids": ids, "prompt": strings.TrimSpace(prompt), "transparent_background": transparent})
	return string(data)
}
