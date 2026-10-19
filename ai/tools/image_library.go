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
		Name: "list_image_assets", Description: "查询本地图片素材。用户回复或附图要求改名、转移、删除时，传image_id按图片内容定位，不需要用户提供hash或旧名字；可不指定类别跨本地类目查找。也支持按类别、文件名关键词分页查询。返回真实素材编号后才能修改；多个结果不可任选，应先确认类别或名称。外部来源不可管理。",
		Triggers: []string{"删除", "移除", "删除图片", "删除表情", "删除语录", "移除图片", "素材", "图片库"}, Patterns: []string{`(删除|移除).{0,16}(图|表情|语录)`, `(图|表情|语录).{0,16}(删除|移除)`},
		Permission: ai.PermSuperUser, ReadOnly: true,
		Params: []ai.Param{{Name: "category", Type: "string", Description: "素材类别；无image_id时留空列出类别，有image_id时留空跨类目匹配"}, {Name: "image_id", Type: "string", Description: "附件或引用图片编号，用于按内容定位已存素材，不是素材编号或hash"}, {Name: "query", Type: "string", Description: "文件名关键词；留空查询全部"}, {Name: "page", Type: "integer", Description: "页码，从1开始"}},
		ResultHandler: func(c *ai.ToolContext) ai.ToolResult {
			l := currentImageLibrary()
			var result any
			page := c.Int("page")
			if page == 0 {
				page = 1
			}
			if id := c.String("image_id"); id != "" {
				urls, err := c.ImageURLs([]string{id})
				if err != nil {
					return outcome(ai.ToolRejected, err.Error(), false)
				}
				data, err := imageassets.Fetch(c.Context(), urls[0])
				if err != nil {
					return outcome(ai.ToolFailed, "引用图片下载失败，请重新附图或引用图片消息。", false)
				}
				cfg := config.Current().Image
				rows, err := l.MatchImage(c.Context(), data, c.String("category"), c.String("query"), int(page), imageassets.Options{Convert: cfg.Convert, MinBytes: cfg.ConvertMinKB * 1024, Quality: cfg.ConvertQuality})
				if err != nil {
					return outcome(ai.ToolFailed, "图片素材匹配失败："+err.Error(), false)
				}
				for _, asset := range rows.Assets {
					c.SetState("image_asset:"+asset.Category+":"+asset.ID, true)
					if category := c.String("category"); category != "" {
						c.SetState("image_asset:"+category+":"+asset.ID, true)
					}
				}
				result = rows
			} else if c.String("category") == "" {
				result = l.Categories()
			} else {
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
		Name: "rename_image_asset", Description: "仅在用户要求重命名时，直接修改已存本地图片的文件名，保留内容hash、类目和语录所属群，不复制或回收原图。回复或附图改名时先用list_image_assets的image_id按内容定位，取得精确素材编号；多个候选不可猜测，新名称必填。随机图改名不改变随机调用方式。",
		Triggers: []string{"重命名", "改名", "改名字", "改个名", "名字改", "名称改", "修改名字", "修改名称"}, Requires: []string{"list_image_assets", "list_image_categories"}, Permission: ai.PermSuperUser, PluginID: catalog.PluginUploadAssets,
		Params: []ai.Param{{Name: "category", Type: "string", Required: true}, {Name: "asset_id", Type: "string", Required: true, Description: "本轮查询得到的素材编号"}, {Name: "name", Type: "string", Required: true, Description: "新名称"}},
		ResultHandler: func(c *ai.ToolContext) ai.ToolResult {
			if _, ok := c.GetState("image_asset:" + c.String("category") + ":" + c.String("asset_id")); !ok {
				return outcome(ai.ToolRejected, "请先查询并确定要重命名的素材。", false)
			}
			result, err := currentImageLibrary().Rename(c.Context(), c.String("category"), c.String("asset_id"), c.String("name"))
			if err != nil {
				return outcome(ai.ToolRejected, err.Error(), false)
			}
			return outcome(ai.ToolSucceeded, "已重命名为："+result.Destination, true)
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
