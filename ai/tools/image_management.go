package tools

import (
	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	imageplugin "github.com/Yuelioi/yueling-go/plugins/image"
)

func (t *toolset) registerImageManagementTools() {
	t.registry.Register(ai.ToolMeta{
		Name: "list_image_assets", Description: "仅机器人超管：列出配置中的图片类目，或查询本地素材的真实编号和文件名。external 外部网站类目不可删除。删除前先查询，不得猜测编号；文件名不能说明画面内容，选图不明确时询问用户编号。",
		Permission: ai.PermSuperUser, PluginID: catalog.PluginUploadAssets, ReadOnly: true,
		Triggers: []string{"图片列表", "本地图片", "素材", "语录", "表情"},
		Params:   []ai.Param{{Name: "category", Type: "string", Description: "留空列出全部配置类目；否则为类目名或命令别名，如语录、表情、龙图"}, {Name: "query", Type: "string", Description: "文件名关键词，仅筛选，不表示要删除全部匹配项"}, {Name: "page", Type: "integer", Description: "从1开始的页码，默认1"}},
		ResultHandler: func(c *ai.ToolContext) ai.ToolResult {
			library := imageplugin.AssetLibrary()
			if _, err := library.Categories(c.UserID()); err != nil {
				return outcome(ai.ToolRejected, imageplugin.ManagementError(err), false)
			}
			if c.String("category") == "" {
				return outcome(ai.ToolSucceeded, imageplugin.CategoryText(library, c.UserID()), false)
			}
			page := c.Int("page")
			if _, exists := c.Params["page"]; !exists {
				page = 1
			}
			result, err := library.List(c.Context(), c.UserID(), c.String("category"), c.String("query"), int(page))
			if err != nil {
				return outcome(ai.ToolRejected, imageplugin.ManagementError(err), false)
			}
			return outcome(ai.ToolSucceeded, imageplugin.AssetPageText(result), false)
		},
	})
	t.registry.Register(ai.ToolMeta{
		Name: "delete_image_asset", Description: "仅机器人超管：删除一个用户明确指定的本地素材（移入回收站）。必须先用list_image_assets取得真实素材编号；不接受图片网址、消息编号、本地路径或模糊名字。无法确定用户指的是哪张时询问，不能猜测。外部网站图片一律不支持删除。",
		Permission: ai.PermSuperUser, PluginID: catalog.PluginUploadAssets, Risk: ai.RiskMedium, Requires: []string{"list_image_assets"},
		Triggers: []string{"删除", "移除", "删掉"}, Patterns: []string{`(删除|移除|删).{0,16}(图|语录|表情|素材)`},
		Params: []ai.Param{{Name: "category", Type: "string", Required: true, Description: "查询结果中的本地素材类目"}, {Name: "asset_id", Type: "string", Required: true, Description: "list_image_assets返回的32位素材编号；只选用户明确要求删除的一张"}},
		ResultHandler: func(c *ai.ToolContext) ai.ToolResult {
			name, err := imageplugin.AssetLibrary().Delete(c.Context(), c.UserID(), c.String("category"), c.String("asset_id"))
			if err != nil {
				return outcome(ai.ToolRejected, imageplugin.ManagementError(err), false)
			}
			return outcome(ai.ToolSucceeded, "已删除本地素材「"+name+"」（已移入服务器回收站）。", true)
		},
	})
}
