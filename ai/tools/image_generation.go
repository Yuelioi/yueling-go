package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
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
			description = "参考1至3张图片重绘、改图或抠图；头像先获取编号，也计入上限。prompt说明各图用途；用途明确时直接改图，无需逐张识图。超出3张先让用户选择。支持单个GIF逐帧改内容，可配静态参考；最多12帧/1200万像素，每帧可能计费并可能闪烁。缩放等基础变换请用群命令。"
			feature = "image_edit"
			params = append(params,
				ai.Param{Name: "image_ids", Type: "array", ItemsType: "string", Required: true, MinItems: 1, MaxItems: 3, Description: "1至3个不同图片编号，按参考用途顺序排列；包括头像"},
			)
		}
		description += " 已有GIF直接编辑并自动合成发送；静图或纯文字制作新GIF时一次填写animation_frames，不要逐张调用工具或让用户拼接。"
		params = append(params,
			ai.Param{Name: "animation_frames", Type: "array", ItemsType: "string", MinItems: 2, MaxItems: llm.MaxAIEditFrames, Description: "仅新建GIF时填写2至12个动作帧描述；每帧可能计费。已有GIF逐帧修改时省略。"},
			ai.Param{Name: "frame_duration_ms", Type: "integer", Description: "新GIF每帧20至5000毫秒，默认400；仅与animation_frames同时使用"},
		)
		var requires []string
		var actionKey ai.ToolActionKey
		if edit {
			requires = []string{"get_user_avatar"}
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
			opts := llm.ImageOptions{TransparentBackground: c.Bool("transparent_background"), AnimationFrames: c.StringSlice("animation_frames"), FrameDurationMS: int(c.Int("frame_duration_ms"))}
			if _, specified := c.Params["frame_duration_ms"]; specified && opts.FrameDurationMS == 0 {
				return outcome(ai.ToolRejected, "动画每帧时长需要20至5000毫秒，默认时请省略该参数。", false)
			}
			if err := opts.Validate(); err != nil {
				return outcome(ai.ToolRejected, llm.UserMessage(err), false)
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
			var sources [][]byte
			for _, url := range sourceURLs {
				source, err := imageassets.Fetch(c.Context(), url)
				if err != nil {
					return outcome(ai.ToolFailed, "原图下载失败", false)
				}
				sources = append(sources, source)
			}
			client := llm.New(llm.ForFeature(cfg, feature))
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
			messageID, err := c.BotAPI().SendGroupMsg(c.GroupID(), bot.Msg().Reply(c.MessageID()).ImageBytes(data).Build())
			if err != nil {
				return outcome(ai.ToolFailed, "图片已生成，但发送未确认成功，请勿自动重复生成。", true)
			}
			_, format, _ := image.DecodeConfig(bytes.NewReader(data))
			caption := prompt
			if len(opts.AnimationFrames) > 0 {
				caption += "；GIF动作：" + strings.Join(opts.AnimationFrames, "、")
			}
			id, err := c.GeneratedImage(messageID, caption, format)
			if err != nil {
				return outcome(ai.ToolSucceeded, "图片已发送，但消息编号未确认，后续编辑请引用图片消息；不要重新生成。", true)
			}
			completed := "图片已发送"
			if format == "gif" {
				completed = "GIF动画已合成并发送，无需用户拼接"
			}
			return outcome(ai.ToolSucceeded, fmt.Sprintf("%s，format=%s，image_id=%s；可在后续请求中继续编辑。", completed, format, id), true)
		}})
	}
}

func imageEditReferences(params map[string]any) ([]string, error) {
	if _, exists := params["image_id"]; exists {
		return nil, fmt.Errorf("请使用image_ids数组提供参考图")
	}
	values, ok := params["image_ids"].([]any)
	if !ok {
		return nil, fmt.Errorf("请用image_ids提供1至3个图片编号")
	}
	ids := make([]string, 0, len(values))
	for _, value := range values {
		id, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("参考图必须是当前会话的图片编号")
		}
		ids = append(ids, id)
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
	frames, _ := params["animation_frames"].([]any)
	duration, _ := params["frame_duration_ms"].(float64)
	if len(frames) > 0 && duration == 0 {
		duration = 400
	}
	data, _ := json.Marshal(map[string]any{"image_ids": ids, "prompt": strings.TrimSpace(prompt), "transparent_background": transparent, "animation_frames": frames, "frame_duration_ms": duration})
	return string(data)
}
