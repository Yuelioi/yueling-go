package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services"
)

func TestImageMutationsRequireLookupAndSuperUser(t *testing.T) {
	for _, action := range []string{"delete_image_asset", "move_image_asset", "rename_image_asset"} {
		t.Run(action, func(t *testing.T) {
			for _, mode := range []string{"lookup", "no_lookup", "member"} {
				t.Run(mode, func(t *testing.T) {
					oldRoot, oldConfig := services.DataDir, config.C
					t.Cleanup(func() { services.DataDir = oldRoot; config.C = oldConfig })
					services.DataDir = t.TempDir()
					dir := filepath.Join(services.DataDir, "images", "表情")
					if err := os.MkdirAll(dir, 0700); err != nil {
						t.Fatal(err)
					}
					file := filepath.Join(dir, "fixture.png")
					if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
						t.Fatal(err)
					}
					rows, err := currentImageLibrary().List(context.Background(), "表情", "", 1)
					if err != nil {
						t.Fatal(err)
					}
					params := map[string]any{"category": "表情", "asset_id": rows.Assets[0].ID, "destination": "语录", "name": "新名字"}
					if action != "move_image_asset" {
						delete(params, "destination")
					}
					if action == "delete_image_asset" {
						delete(params, "name")
					}
					raw, _ := json.Marshal(params)
					deleteTool, _ := testEngine.Registry.GetTool(action)
					listTool, _ := testEngine.Registry.GetTool("list_image_assets")
					original := deleteTool.ResultHandler
					t.Cleanup(func() { deleteTool.ResultHandler = original })
					deleteTool.ResultHandler = func(c *ai.ToolContext) ai.ToolResult {
						if mode == "lookup" {
							old := c.Params
							c.Params = map[string]any{"category": "表情"}
							result := listTool.ResultHandler(c)
							c.Params = old
							if result.Status != ai.ToolSucceeded {
								t.Fatal(result)
							}
						}
						return original(c)
					}
					if mode != "member" {
						config.C.Bot.SuperUsers = []int64{900000 + reminderDispatchSequence.Load() + 1}
					} else {
						config.C.Bot.SuperUsers = nil
						config.C.Bot.OwnerID = 0
					}
					_, results := runReminderDispatch(t, []string{string(raw)}, reminderDispatchOptions{toolName: action, input: "删除表情 fixture，转成语录，重命名", role: "member"})
					if len(results) != 1 {
						t.Fatal(results)
					}
					if mode == "lookup" {
						if results[0].Status != ai.ToolSucceeded {
							t.Fatal(results)
						}
						if _, err := os.Stat(file); !os.IsNotExist(err) {
							t.Fatal("asset remained active", err)
						}
						trash, _ := filepath.Glob(filepath.Join(services.DataDir, "trash", "images", "表情", "*", "fixture.png"))
						if len(trash) != 1 {
							t.Fatal("missing recoverable file")
						}
					} else {
						if results[0].Status != ai.ToolRejected {
							t.Fatal(results)
						}
						if _, err := os.Stat(file); err != nil {
							t.Fatal("rejected request changed file", err)
						}
					}
				})
			}
		})
	}
}
