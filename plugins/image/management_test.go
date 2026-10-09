package image

import (
	"github.com/Yuelioi/yueling-go/config"
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
