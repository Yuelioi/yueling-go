package image

import (
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/imageassets"
	"strings"
	"testing"
)

func TestManagementConfiguredSources(t *testing.T) {
	entries := []config.ImageEntry{{Folder: "自定义", Call: []string{"别名"}}, {Folder: "网站", Kind: config.KindExternal, Call: []string{"随机网站图"}, URL: "https://example.com", Pick: "data[*].url"}}
	categories := AssetCategories(entries)
	if len(categories) != 4 || categories[2].Name != "自定义" || categories[2].External || !categories[3].External {
		t.Fatal(categories)
	}
}
func TestNoImageManagementChatCommands(t *testing.T) {
	for _, cmd := range HelpAddCommands() {
		if strings.HasPrefix(cmd, "删除") || cmd == "图片列表" {
			t.Fatal("管理应仅在后台", cmd)
		}
	}
}

func TestUnifiedUploadUsesConfiguredCategories(t *testing.T) {
	label := true
	entries := []config.ImageEntry{
		{Folder: "自定义", Call: []string{"别名"}, Add: "添加自定义"},
		{Folder: "饭菜", Call: []string{"吃饭"}, Add: "添加饭菜", Kind: config.KindGrid},
		{Folder: "标签图", Call: []string{"标签"}, Add: "添加标签", Arg: &label},
		{Folder: "只读", Call: []string{"只读"}},
		{Folder: "外站", Call: []string{"外图"}, Add: "添加外图", Kind: config.KindExternal},
	}
	library := imageassets.NewLibrary(t.TempDir(), AssetCategories(entries))
	for _, tc := range []struct{ category, name, suffix string }{
		{"添加自定义", "", "hash"}, {"吃饭", "麻辣烫", "麻辣烫_hash"}, {"标签", "猫", "猫_hash"},
		{"表情包", "开心", "开心_hash"}, {"表情", "开心", "开心_hash"}, {"语录", "小明", "42_小明_hash"},
	} {
		_, fn, err := library.UploadTarget(tc.category, tc.name, 42)
		if err != nil || fn("hash") != tc.suffix {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
	for _, category := range []string{"龙图", "外图", "只读", "../escape", "饭菜", "语录", "表情", "表情包"} {
		if _, _, err := library.UploadTarget(category, "", 42); err == nil {
			t.Fatal("accepted invalid upload", category)
		}
	}
}
