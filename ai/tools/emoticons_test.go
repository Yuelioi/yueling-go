package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services"
)

func emoticonFixture(t *testing.T) string {
	t.Helper()
	old := services.DataDir
	services.DataDir = t.TempDir()
	t.Cleanup(func() { services.DataDir = old })
	dir := filepath.Join(services.DataDir, "images", "表情")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("开心_%02d.png", i)), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "猫猫_01.gif"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestEmoticonSearchMatchesSpaceCommandAndPaginates(t *testing.T) {
	emoticonFixture(t)
	want, err := services.ListImageNames("表情", "开心")
	if err != nil {
		t.Fatal(err)
	}
	var all []string
	for page := 1; page <= 2; page++ {
		result := searchEmoticons(&ai.ToolContext{Params: map[string]any{"keyword": "开心", "page": page}})
		if result.Status != ai.ToolSucceeded || result.PossibleSideEffects {
			t.Fatal(result)
		}
		var response struct {
			Names       []string
			Total, Page int
			HasMore     bool `json:"has_more"`
		}
		if err := json.Unmarshal([]byte(result.Content), &response); err != nil {
			t.Fatal(err)
		}
		if response.Total != 12 || response.Page != page || response.HasMore != (page == 1) || len(response.Names) > 10 {
			t.Fatal(response)
		}
		all = append(all, response.Names...)
	}
	if !reflect.DeepEqual(all, want) {
		t.Fatalf("AI search=%v, space search=%v", all, want)
	}
	for _, params := range []map[string]any{{"keyword": "不存在"}, {"keyword": "开心", "page": 3}} {
		result := searchEmoticons(&ai.ToolContext{Params: params})
		var response struct{ Names []string }
		if result.Status != ai.ToolSucceeded || json.Unmarshal([]byte(result.Content), &response) != nil || response.Names == nil || len(response.Names) != 0 {
			t.Fatal(result)
		}
	}
}

func TestEmoticonSearchHandlesUnavailableLibrary(t *testing.T) {
	dir := emoticonFixture(t)
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	result := searchEmoticons(&ai.ToolContext{})
	if result.Status != ai.ToolSucceeded {
		t.Fatal(result)
	}
	if err := os.WriteFile(dir, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	result = searchEmoticons(&ai.ToolContext{})
	if result.Status != ai.ToolFailed || result.Content != "暂时无法读取表情库，请稍后再试。" {
		t.Fatal(result)
	}
	result = searchEmoticons(&ai.ToolContext{Params: map[string]any{"page": -1}})
	if result.Status != ai.ToolRejected {
		t.Fatal(result)
	}
}

func TestEmoticonSearchMemberAccessAndPluginGate(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("disabled=%v", disabled), func(t *testing.T) {
			emoticonFixture(t)
			old := config.C
			t.Cleanup(func() { config.C = old })
			config.C.Bot.SuperUsers = nil
			config.C.Bot.OwnerID = 0
			config.C.Plugins.Disabled = nil
			if disabled {
				config.C.Plugins.Disabled = []int{catalog.PluginEmoticon}
			}
			_, results := runReminderDispatch(t, []string{`{"keyword":"猫猫"}`}, reminderDispatchOptions{toolName: "search_emoticons", input: "有哪些猫猫表情", role: "member"})
			if len(results) != 1 {
				t.Fatal(results)
			}
			if disabled {
				if results[0].Status != ai.ToolRejected {
					t.Fatal(results)
				}
			} else {
				if results[0].Status != ai.ToolSucceeded {
					t.Fatal(results)
				}
				var response struct{ Names []string }
				if err := json.Unmarshal([]byte(results[0].Content), &response); err != nil || !reflect.DeepEqual(response.Names, []string{"猫猫_01"}) {
					t.Fatal(results)
				}
			}
		})
	}
}
