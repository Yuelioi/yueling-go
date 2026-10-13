package tools

import (
	"encoding/json"
	"fmt"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/plugins/image"
	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/imageassets"
)

func currentImageLibrary() *imageassets.Library {
	return imageassets.NewLibrary(services.DataDir, image.AssetCategories(config.Current().Image.Entry))
}

func (t *toolset) registerImageLibraryTools() {
	t.registry.Register(ai.ToolMeta{Name: "list_image_categories", Description: "读取当前配置所有图片类目及别名、是否支持上传、名称是否必填；统一图片操作前据此选类目，外站不可保存或删除。", Triggers: []string{"图片", "素材", "类目", "添加", "保存"}, Permission: ai.PermMember, ReadOnly: true, ResultHandler: func(c *ai.ToolContext) ai.ToolResult {
		data, _ := json.Marshal(currentImageLibrary().Categories())
		return outcome(ai.ToolSucceeded, string(data), false)
	}})

	t.registry.Register(ai.ToolMeta{
		Name: "list_image_assets", Description: "查询本地图片素材类别或按类别、关键词分页查找精确素材编号。删除前先查询，不能猜编号或把附件编号当素材编号。多个结果不能擅自任选；目标不明确先追问。外部来源不可管理。",
		Triggers: []string{"删除", "移除", "删除图片", "删除表情", "删除语录", "移除图片", "素材", "图片库"}, Patterns: []string{`(删除|移除).{0,16}(图|表情|语录)`, `(图|表情|语录).{0,16}(删除|移除)`},
		Permission: ai.PermSuperUser, ReadOnly: true,
		Params: []ai.Param{{Name: "category", Type: "string", Description: "素材类别；留空列出类别"}, {Name: "query", Type: "string", Description: "文件名关键词；留空查询全部"}, {Name: "page", Type: "integer", Description: "页码，从1开始"}},
		ResultHandler: func(c *ai.ToolContext) ai.ToolResult {
			l := currentImageLibrary()
			var result any
			if c.String("category") == "" {
				result = l.Categories()
			} else {
				page := c.Int("page")
				if page == 0 {
					page = 1
				}
				rows, err := l.List(c.Context(), c.String("category"), c.String("query"), int(page))
				if err != nil {
					return outcome(ai.ToolRejected, err.Error(), false)
				}
				for _, asset := range rows.Assets {
					c.SetState("image_asset:"+rows.Category+":"+asset.ID, true)
					c.SetState("image_asset:"+c.String("category")+":"+asset.ID, true)
				}
				result = rows
			}
			data, _ := json.Marshal(result)
			return outcome(ai.ToolSucceeded, string(data), false)
		},
	})
	t.registry.Register(ai.ToolMeta{
		Name: "move_image_asset", Description: "将已存的本地图片从一个类目转移到另一个类目（包括语录转表情包）。先查询源图片精确编号并确认目标类目和必填名字；成功写入目标后才把原图移入回收站，不能先删除再重新下载。表情名字必填，用于关键词调用。多个源候选不可猜测。",
		Triggers: []string{"转成", "转为", "转到", "移到", "移动", "迁移", "改成", "转表情", "转语录"}, Requires: []string{"list_image_assets", "list_image_categories"}, Permission: ai.PermSuperUser, PluginID: catalog.PluginUploadAssets,
		Params: []ai.Param{{Name: "category", Type: "string", Required: true, Description: "源类别"}, {Name: "asset_id", Type: "string", Required: true, Description: "本轮查询得到的源素材编号"}, {Name: "destination", Type: "string", Required: true, Description: "目标类别或别名"}, {Name: "name", Type: "string", Description: "目标图片名称，语录和表情必填"}},
		ResultHandler: func(c *ai.ToolContext) ai.ToolResult {
			if _, ok := c.GetState("image_asset:" + c.String("category") + ":" + c.String("asset_id")); !ok {
				return outcome(ai.ToolRejected, "请先查询并确定要转移的源素材。", false)
			}
			result, err := currentImageLibrary().Move(c.Context(), c.String("category"), c.String("asset_id"), c.String("destination"), c.String("name"), c.GroupID())
			if err != nil {
				if result.Copied {
					return outcome(ai.ToolFailed, "目标图片已保存，但原图未能移入回收站；当前两份均保留，请重新查询核对。", true)
				}
				return outcome(ai.ToolRejected, err.Error(), false)
			}
			return outcome(ai.ToolSucceeded, fmt.Sprintf("已转移到%s：%s；原图片已移入回收站。", c.String("destination"), result.Destination), true)
		},
	})
	t.registry.Register(ai.ToolMeta{
		Name: "rename_image_asset", Description: "仅在用户要求重命名时，修改已存本地图片的名字，保留类目和语录所属群。必须先查询精确素材编号；多个候选不可猜测，新名称必填。旧文件移入回收站。随机图改名不改变随机调用方式。",
		Triggers: []string{"重命名", "改名", "改名字", "改个名", "名字改", "名称改"}, Requires: []string{"list_image_assets", "list_image_categories"}, Permission: ai.PermSuperUser, PluginID: catalog.PluginUploadAssets,
		Params: []ai.Param{{Name: "category", Type: "string", Required: true}, {Name: "asset_id", Type: "string", Required: true, Description: "本轮查询得到的素材编号"}, {Name: "name", Type: "string", Required: true, Description: "新名称"}},
		ResultHandler: func(c *ai.ToolContext) ai.ToolResult {
			if _, ok := c.GetState("image_asset:" + c.String("category") + ":" + c.String("asset_id")); !ok {
				return outcome(ai.ToolRejected, "请先查询并确定要重命名的素材。", false)
			}
			result, err := currentImageLibrary().Rename(c.Context(), c.String("category"), c.String("asset_id"), c.String("name"))
			if err != nil {
				if result.Copied {
					return outcome(ai.ToolFailed, "新名称已保存，但旧文件未能移入回收站；两份均保留，请重新查询核对。", true)
				}
				return outcome(ai.ToolRejected, err.Error(), false)
			}
			return outcome(ai.ToolSucceeded, "已重命名为："+result.Destination+"；旧文件已移入回收站。", true)
		},
	})
	t.registry.Register(ai.ToolMeta{
		Name: "delete_image_asset", Description: "仅在用户明确要求删除且已从list_image_assets确定唯一目标后，将一个本地素材移入回收站。asset_id必须是查询返回的真实素材编号，不能用附件编号、网址、路径或猜测；目标有歧义先追问。表情/普通图片为全局素材，语录文件名含所属群。",
		Triggers: []string{"删除", "移除", "删除图片", "删除表情", "删除语录"}, Patterns: []string{`(删除|移除).{0,16}(图|表情|语录)`, `(图|表情|语录).{0,16}(删除|移除)`}, Requires: []string{"list_image_assets"}, Permission: ai.PermSuperUser,
		Params: []ai.Param{{Name: "category", Type: "string", Required: true}, {Name: "asset_id", Type: "string", Required: true}},
		ResultHandler: func(c *ai.ToolContext) ai.ToolResult {
			if _, ok := c.GetState("image_asset:" + c.String("category") + ":" + c.String("asset_id")); !ok {
				return outcome(ai.ToolRejected, "请先在本轮查询素材，核实目标编号后再删除。", false)
			}
			name, err := currentImageLibrary().Delete(c.Context(), c.String("category"), c.String("asset_id"))
			if err != nil {
				return outcome(ai.ToolRejected, err.Error(), false)
			}
			return outcome(ai.ToolSucceeded, "已移入回收站："+name, true)
		},
	})
}
