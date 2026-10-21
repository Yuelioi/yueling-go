package tools

import (
	"math"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services/imageassets"
	"github.com/Yuelioi/yueling-go/services/imaging"
)

func (t *toolset) registerImageTransform() {
	t.registry.Register(ai.ToolMeta{
		Name: "transform_image", Permission: ai.PermMember, PluginID: catalog.PluginImageOps,
		Description: "对附件、引用图或头像做本地基础处理，支持静态图和GIF全部帧，保留透明通道、帧时长和循环。不调用付费图片模型；缩放、翻转、镜像、旋转、灰度、反色优先使用。不能用于抠图、换背景或重绘内容。GIF最多200帧、总像素2400万。",
		Requires:    []string{"get_user_avatar"}, Triggers: []string{"缩放", "翻转", "镜像", "旋转", "灰度", "黑白", "反色", "GIF", "gif"},
		Params: []ai.Param{
			{Name: "image_id", Type: "string", Required: true, Description: "当前会话的图片编号"},
			{Name: "operation", Type: "string", Required: true, Enum: []string{"flip_horizontal", "flip_vertical", "mirror_left", "mirror_right", "mirror_top", "mirror_bottom", "rotate", "resize", "grayscale", "invert"}, Description: "基础操作"},
			{Name: "degrees", Type: "integer", Description: "旋转角度，仅支持90的倍数"},
			{Name: "width", Type: "integer", Description: "缩放目标宽度1至4096；仅传宽度时保持比例"},
			{Name: "height", Type: "integer", Description: "缩放目标高度1至4096；仅传高度时保持比例"},
			{Name: "scale", Type: "number", Description: "缩放倍率0.05至4，例如缩小到50%传0.5；与宽高互斥"},
		}, ResultHandler: transformImage,
	})
}

func transformImage(c *ai.ToolContext) ai.ToolResult {
	op := imaging.Operation{Kind: imaging.Kind(c.String("operation")), Degrees: int(c.Int("degrees")), Width: int(c.Int("width")), Height: int(c.Int("height")), Scale: c.Float("scale")}
	_, degrees := c.Params["degrees"]
	_, width := c.Params["width"]
	_, height := c.Params["height"]
	_, scale := c.Params["scale"]
	if op.Kind == imaging.Rotate && (!degrees || op.Degrees%90 != 0) || op.Kind != imaging.Rotate && degrees {
		return outcome(ai.ToolRejected, "旋转须提供90度倍数的degrees，其他操作不使用此参数。", false)
	}
	if op.Kind == imaging.Resize {
		if scale && (width || height) || !scale && !width && !height || width && (op.Width < 1 || op.Width > 4096) || height && (op.Height < 1 || op.Height > 4096) || scale && (math.IsNaN(op.Scale) || math.IsInf(op.Scale, 0) || op.Scale < .05 || op.Scale > 4) {
			return outcome(ai.ToolRejected, "缩放须提供1至4096的宽高或0.05至4的倍率，二者不能混用。", false)
		}
	} else if width || height || scale {
		return outcome(ai.ToolRejected, "仅缩放操作使用width、height或scale。", false)
	}
	urls, err := c.ImageURLs([]string{c.String("image_id")})
	if err != nil {
		return outcome(ai.ToolRejected, "图片编号不可用或已过期，请重新附图或获取头像。", false)
	}
	data, err := imageassets.Fetch(c.Context(), urls[0])
	if err != nil {
		return outcome(ai.ToolFailed, "原图下载失败", false)
	}
	result, err := imaging.Run(c.Context(), func() ([]byte, error) { return imaging.Process(data, op, imaging.DefaultLimits) })
	if err != nil {
		return outcome(ai.ToolRejected, "图片处理失败："+err.Error(), false)
	}
	if _, err := c.BotAPI().SendGroupMsg(c.GroupID(), bot.Msg().Reply(c.MessageID()).ImageBytes(result).Build()); err != nil {
		return outcome(ai.ToolFailed, "图片已处理，但发送未确认成功。", true)
	}
	return outcome(ai.ToolSucceeded, "处理后的图片已发送，GIF保留动画；未调用图片模型。", true)
}
