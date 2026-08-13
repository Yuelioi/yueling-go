package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
	openai "github.com/sashabaranov/go-openai"
)

func TestToolSchemaSupportsEnumsAndArrays(t *testing.T) {
	tool := ToolMeta{Name: "schema_test", Params: []Param{
		{Name: "action", Type: "string", Enum: []string{"add", "remove"}, Required: true},
		{Name: "weekdays", Type: "array", ItemsType: "integer"},
	}}
	var schema map[string]any
	raw, ok := tool.schema().Function.Parameters.(json.RawMessage)
	if !ok {
		t.Fatalf("parameters type=%T", tool.schema().Function.Parameters)
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	action := properties["action"].(map[string]any)
	if len(action["enum"].([]any)) != 2 {
		t.Fatalf("action enum=%#v", action["enum"])
	}
	weekdays := properties["weekdays"].(map[string]any)
	if weekdays["items"].(map[string]any)["type"] != "integer" {
		t.Fatalf("weekdays schema=%#v", weekdays)
	}
}

func TestFilterByGroupPluginRemovesDisabledTools(t *testing.T) {
	cleanupAIConfigAndDB(t)
	initAffinityTestDB(t)
	if err := db.SetGroupPluginDisabled(100, 13, true); err != nil {
		t.Fatal(err)
	}
	always := &ToolMeta{Name: "always"}
	reminder := &ToolMeta{Name: "reminder", PluginID: 13}
	filtered := filterByGroupPlugin([]*ToolMeta{always, reminder}, 100)
	if len(filtered) != 1 || filtered[0].Name != "always" {
		t.Fatalf("filtered=%+v", filtered)
	}
}

func TestExecuteToolRejectsToolNotExposedThisTurn(t *testing.T) {
	oldRegistry := global
	t.Cleanup(func() { global = oldRegistry })
	global = &registry{tools: map[string]*ToolMeta{}}
	Register(ToolMeta{Name: "hidden_test", Handler: func(*ToolContext) (string, error) { return "executed", nil }})

	event := &bot.GroupMessageEvent{GroupID: 100, UserID: 42}
	result := executeTool(context.Background(), nil, event, newSession(42, 100), PermMember, openai.ToolCall{
		Function: openai.FunctionCall{Name: "hidden_test", Arguments: `{}`},
	}, map[string]bool{})
	if !strings.Contains(result, "未被本轮请求匹配") {
		t.Fatalf("result=%q", result)
	}
}
