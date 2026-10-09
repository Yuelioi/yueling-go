package tools

import (
	"context"
	"encoding/json"
	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	imageplugin "github.com/Yuelioi/yueling-go/plugins/image"
	"github.com/Yuelioi/yueling-go/services"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeleteImageToolIsSuperuserOnly(t *testing.T) {
	tool, ok := testEngine.Registry.GetTool("delete_image_asset")
	if !ok {
		t.Fatal("图片删除工具未注册，删除请求无法执行")
	}
	if tool.Permission != ai.PermSuperUser {
		t.Fatal("图片删除必须仅超管可用")
	}
	if len(tool.Requires) != 1 || tool.Requires[0] != "list_image_assets" {
		t.Fatal("删除前必须能查询真实本地素材编号")
	}
	for _, input := range []string{"删除这张图片", "删除这个语录", "删掉表情", "删除吃的", "移除水果"} {
		if !containsRoutedTool(ai.Route(input, testEngine.Registry.AllTools()), tool.Name) {
			t.Fatalf("删除请求未路由: %s", input)
		}
	}
}

func TestDeleteImageRejectsGroupRolesThroughDispatch(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	config.C.Bot.SuperUsers = nil
	config.C.Bot.OwnerID = 0
	for _, role := range []string{"member", "admin", "owner"} {
		t.Run(role, func(t *testing.T) {
			_, results := runReminderDispatch(t, []string{`{"category":"语录","asset_id":"00000000000000000000000000000000"}`}, reminderDispatchOptions{toolName: "delete_image_asset", input: "删除这张语录", role: role})
			if results[0].Status != ai.ToolRejected || results[0].PossibleSideEffects || !strings.Contains(results[0].Content, "权限不足") {
				t.Fatal(results)
			}
		})
	}
}
func TestDeleteLocalImageThroughDispatch(t *testing.T) {
	oldCfg, oldDir := config.C, services.DataDir
	t.Cleanup(func() { config.C = oldCfg; services.DataDir = oldDir })
	services.DataDir = t.TempDir()
	config.C.Image.Entry = nil
	config.C.Bot.SuperUsers = []int64{99}
	dir := filepath.Join(services.DataDir, "images", "语录")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "100_小明_fixture.png")
	if err := os.WriteFile(file, []byte("fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	page, err := imageplugin.AssetLibrary().List(context.Background(), 99, "语录", "小明", 1)
	if err != nil || len(page.Assets) != 1 {
		t.Fatal(page, err)
	}
	args, _ := json.Marshal(map[string]string{"category": "语录", "asset_id": page.Assets[0].ID})
	_, results := runReminderDispatch(t, []string{string(args)}, reminderDispatchOptions{toolName: "delete_image_asset", input: "删除这张语录", role: "member", superUser: true})
	if results[0].Status != ai.ToolSucceeded || !results[0].PossibleSideEffects {
		t.Fatal(results)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("local image remained", err)
	}
	copies, err := filepath.Glob(filepath.Join(services.DataDir, "trash", "images", "语录", "*", "100_小明_fixture.png"))
	if err != nil || len(copies) != 1 {
		t.Fatalf("%v %v", copies, err)
	}
}
