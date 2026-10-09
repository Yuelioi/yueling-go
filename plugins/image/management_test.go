package image

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/imageassets"
)

func TestManagementUsesConfiguredSources(t *testing.T) {
	entries := []config.ImageEntry{
		{Folder: "我的 本地图", Call: []string{"自定义图"}, Add: "添加自定义"},
		{Folder: "菜单", Kind: config.KindGrid, Call: []string{"吃啥"}, Add: "添加菜"},
		{Folder: "网站", Kind: config.KindExternal, Call: []string{"随机网站图"}, URL: "https://example.com", Pick: "data[*].url", Base: "https://example.com"},
	}
	root := t.TempDir()
	library := imageassets.NewLibrary(root, assetCategories(entries), []int64{99})
	text := manageImageCommand(context.Background(), library, 99, "list", nil)
	for _, want := range []string{"我的 本地图（本地", "菜单（本地", "网站（外部网站", "语录（本地", "表情（本地"} {
		if !strings.Contains(text, want) {
			t.Fatal(text)
		}
	}
	if strings.Contains(text, "龙图") {
		t.Fatal("custom entries unexpectedly merged defaults")
	}
	denied := manageImageCommand(context.Background(), library, 1, "delete", []string{"我的 本地图", "anything"})
	if !strings.Contains(denied, "只有机器人超管") {
		t.Fatal(denied)
	}
	external := manageImageCommand(context.Background(), library, 99, "delete", []string{"随机网站图", "anything"})
	if !strings.Contains(external, "外部网站") {
		t.Fatal(external)
	}
	if err := os.MkdirAll(filepath.Join(root, "images", "我的 本地图"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "images", "我的 本地图", "小明.png"), []byte("fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	page, err := library.List(context.Background(), 99, "自定义图", "", 1)
	if err != nil || len(page.Assets) != 1 {
		t.Fatal(page, err)
	}
	reply := manageImageCommand(context.Background(), library, 99, "delete", []string{"我的", "本地图", page.Assets[0].ID})
	if !strings.Contains(reply, "已删除本地素材") {
		t.Fatal(reply)
	}
}
func TestManagementCommandsReservedAndHelpListed(t *testing.T) {
	for _, name := range []string{"图片列表", "删除图片", "删除语录", "删除表情"} {
		if err := validateEntries([]config.ImageEntry{{Folder: "test", Call: []string{name}}}); err == nil {
			t.Fatal("management command overridden", name)
		}
		if !strings.Contains(strings.Join(HelpAddCommands(), " "), name) {
			t.Fatal("missing help", name)
		}
	}
}
