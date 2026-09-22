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
	if !strings.Contains(result.Content, "未被本轮请求匹配") {
		t.Fatalf("result=%+v", result)
	}
}

func TestExecuteToolValidatesArgumentsBeforeHandler(t *testing.T) {
	oldRegistry := global
	t.Cleanup(func() { global = oldRegistry })
	global = &registry{tools: map[string]*ToolMeta{}}
	executed := 0
	Register(ToolMeta{Name: "validate", Params: []Param{{Name: "count", Type: "integer", Required: true}, {Name: "mode", Type: "string", Enum: []string{"summary", "actions"}}}, Handler: func(*ToolContext) (string, error) { executed++; return "executed", nil }})
	for _, args := range []string{`{}`, `{"count":"two"}`, `{"count":1.5}`, `{"count":1,"mode":"unknown"}`, `{"count":1,"extra":true}`, `null`} {
		result := executeTool(context.Background(), nil, &bot.GroupMessageEvent{GroupID: 1, UserID: 2}, newSession(2, 1), PermMember, openai.ToolCall{Function: openai.FunctionCall{Name: "validate", Arguments: args}}, nil)
		if !strings.Contains(result.Content, "参数") {
			t.Errorf("args=%s result=%+v", args, result)
		}
	}
	if executed != 0 {
		t.Fatalf("executed invalid arguments %d times", executed)
	}
}

func TestExecuteToolDoesNotRepeatSameAction(t *testing.T) {
	old := global
	t.Cleanup(func() { global = old })
	global = &registry{tools: map[string]*ToolMeta{}}
	calls := 0
	Register(ToolMeta{Name: "action", Params: []Param{{Name: "target", Type: "integer"}}, Handler: func(*ToolContext) (string, error) { calls++; return "已执行", nil }})
	session := newSession(1, 2)
	for i := 0; i < 2; i++ {
		executeTool(context.Background(), nil, &bot.GroupMessageEvent{GroupID: 2, UserID: 1}, session, PermMember, openai.ToolCall{Function: openai.FunctionCall{Name: "action", Arguments: `{"target":1}`}}, nil)
	}
	if calls != 1 {
		t.Fatalf("action executed %d times", calls)
	}
}

func TestExecuteToolContainsPanicAndRemembersUncertainResult(t *testing.T) {
	old := global
	t.Cleanup(func() { global = old })
	global = &registry{tools: map[string]*ToolMeta{}}
	calls := 0
	Register(ToolMeta{Name: "panic", Handler: func(*ToolContext) (string, error) { calls++; panic("secret internal data") }})
	session := newSession(1, 2)
	for i := 0; i < 2; i++ {
		got := executeTool(context.Background(), nil, &bot.GroupMessageEvent{GroupID: 2, UserID: 1}, session, PermMember, openai.ToolCall{Function: openai.FunctionCall{Name: "panic", Arguments: `{}`}}, nil)
		if !strings.Contains(got.Content, "未能确认") || strings.Contains(got.Content, "secret") {
			t.Fatalf("result=%+v", got)
		}
	}
	if calls != 1 {
		t.Fatalf("uncertain action retried %d times", calls)
	}
}
