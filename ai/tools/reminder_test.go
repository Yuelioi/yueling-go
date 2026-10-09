package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
	"github.com/Yuelioi/yueling-go/scheduler"
	"github.com/Yuelioi/yueling-go/services/knowledge"
	openai "github.com/sashabaranov/go-openai"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestParseReminderTimeRequiresRFC3339Timezone(t *testing.T) {
	if _, err := parseReminderTime("明天上午九点"); err == nil {
		t.Fatal("parseReminderTime should reject non-RFC3339 values passed by the model")
	}
	got, err := parseReminderTime("2026-08-14T09:00:00+08:00")
	if err != nil || got.Hour() != 9 {
		t.Fatalf("parseReminderTime() = %v, %v", got, err)
	}
}

func TestReminderToolOwnsNaturalReminderRoutes(t *testing.T) {
	tool, ok := testEngine.Registry.GetTool("manage_reminder")
	if !ok {
		t.Fatal("manage_reminder tool is not registered")
	}
	for _, text := range []string{
		"提醒我30分钟后关火",
		"明天早上叫我喝水",
		"工作日提醒我签到",
		"我的提醒",
		"推迟提醒到下午三点",
	} {
		if routed := ai.Route(text, []*ai.ToolMeta{tool}); len(routed) != 1 {
			t.Fatalf("Route(%q) = %#v", text, routed)
		}
	}
}

func TestReminderEquivalentCreatesExecuteOnceThroughDispatch(t *testing.T) {
	args := []string{
		`{"action":"create","content":"周四测试","trigger_at":"2026-10-01T09:00:00+08:00"}`,
		`{"action":"create","content":"周四测试","trigger_at":"2026-10-01T09:00:00+08:00","repeat":"none"}`,
		`{"action":"create","content":"  周四测试\n","trigger_at":" 2026-10-01T01:00:00Z ","repeat":"none"}`,
		`{"action":"create","content":"周四测试","trigger_at":"2026-10-01T01:00:00.1Z"}`,
		`{"action":"create","content":"周四测试","trigger_at":"2026-10-01T09:00:00.999999999+08:00"}`,
		`{"action":"create","content":"周五发布","trigger_at":"2026-10-01T01:00:00Z"}`,
	}
	calls, results := runReminderDispatch(t, args)
	if len(calls) != 2 || calls[0]["content"] != "周四测试" || calls[1]["content"] != "周五发布" {
		t.Fatalf("equivalent creates consumed executions or a distinct action was lost: %+v", calls)
	}
	for i := 1; i < len(results)-1; i++ {
		if results[0] != results[i] {
			t.Fatalf("equivalent action %d did not share the original receipt: %+v", i, results)
		}
	}
	if results[0] == results[len(results)-1] {
		t.Fatalf("different content shared the same receipt: %+v", results)
	}
	if _, added := calls[0]["repeat"]; added {
		t.Fatal("action identity changed the arguments passed to the handler")
	}
}

func TestReminderDifferentInstantsRemainDistinctThroughDispatch(t *testing.T) {
	for _, test := range []struct {
		name   string
		first  string
		second string
	}{
		{"different_offsets", "2026-10-01T09:00:00+08:00", "2026-10-01T09:00:00Z"},
		{"adjacent_seconds", "2026-10-01T01:00:00.999999999Z", "2026-10-01T01:00:01.1Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls, results := runReminderDispatch(t, []string{
				fmt.Sprintf(`{"action":"create","content":"测试","trigger_at":%q}`, test.first),
				fmt.Sprintf(`{"action":"create","content":"测试","trigger_at":%q}`, test.second),
			})
			if len(calls) != 2 || results[0] == results[1] {
				t.Fatalf("different stored seconds were merged: calls=%+v results=%+v", calls, results)
			}
		})
	}
}

func TestReminderUpdateDoesNotDefaultRepeatThroughDispatch(t *testing.T) {
	calls, _ := runReminderDispatch(t, []string{
		`{"action":"update","reminder_id":42,"content":"更新内容"}`,
		`{"action":"update","reminder_id":42,"content":"更新内容","repeat":"none"}`,
	})
	if len(calls) != 2 {
		t.Fatalf("update defaults were treated as an explicit schedule change: %+v", calls)
	}
	current := &db.Reminder{Recurring: true, CronExpr: "0 9 * * *", Message: "原内容"}
	cron, _, recurring, err := testTools.requestedSchedule(&ai.ToolContext{Params: calls[0]}, current)
	if err != nil || !recurring || cron != current.CronExpr {
		t.Fatalf("omitted schedule should preserve existing schedule: %q %t %v", cron, recurring, err)
	}
	if _, _, _, err := testTools.requestedSchedule(&ai.ToolContext{Params: calls[1]}, current); err == nil {
		t.Fatal("explicit repeat=none without trigger_at unexpectedly preserved existing schedule")
	}
}

var reminderDispatchSequence atomic.Int64

// Exercises the actual registered schema, routing and executor with a counting
// handler at the persistence boundary; it does not claim real database writes.
type reminderDispatchOptions struct {
	database              *gorm.DB
	toolName, input, role string
}

func runReminderDispatch(t *testing.T, args []string, actual ...reminderDispatchOptions) ([]map[string]any, []ai.ToolResult) {
	t.Helper()
	name, input, role := "manage_reminder", "提醒我按安排执行", "member"
	if len(actual) > 0 {
		if actual[0].toolName != "" {
			name = actual[0].toolName
		}
		if actual[0].input != "" {
			input = actual[0].input
		}
		if actual[0].role != "" {
			role = actual[0].role
		}
	}
	tool, ok := testEngine.Registry.GetTool(name)
	if !ok {
		t.Fatal("manage_reminder is not registered")
	}
	oldHandler, oldResultHandler, oldConfig, oldSessions, oldDB := tool.Handler, tool.ResultHandler, config.C, testEngine.Sessions, testdb.Database
	t.Cleanup(func() {
		tool.Handler, tool.ResultHandler, config.C, testEngine.Sessions, testdb.Database = oldHandler, oldResultHandler, oldConfig, oldSessions, oldDB
	})
	var calls []map[string]any
	if len(actual) == 0 {
		tool.ResultHandler = nil
		tool.Handler = func(ctx *ai.ToolContext) (string, error) {
			calls = append(calls, ctx.Params)
			return fmt.Sprintf("receipt-%d", len(calls)), nil
		}
	}
	oldMemory := testTools.memory
	oldEngineMemory := testEngine.Memory
	t.Cleanup(func() { testTools.memory = oldMemory; testEngine.Memory = oldEngineMemory })
	oldTodos := testTools.todos
	t.Cleanup(func() { testTools.todos = oldTodos })
	oldKnowledge := testTools.knowledge
	t.Cleanup(func() { testTools.knowledge = oldKnowledge })
	oldSchedules := testTools.schedules
	t.Cleanup(func() { testTools.schedules = oldSchedules })
	testdb.Database = nil
	if len(actual) > 0 {
		testdb.Database = actual[0].database
	}
	testTools.schedules = scheduler.New(db.NewScheduleStore(testdb.Database))
	testTools.knowledge = knowledge.New(db.NewKnowledgeStore(testdb.Database))
	oldGroups := testEngine.Groups
	testEngine.Groups = db.NewGroupStore(testdb.Database)
	t.Cleanup(func() { testEngine.Groups = oldGroups })
	testTools.todos = db.NewTodoStore(testdb.Database)
	testTools.memory = ai.NewMemoryManager(testdb.Database)
	testEngine.Memory = testTools.memory
	testEngine.Sessions = &ai.SessionManager{}
	resultChannel := make(chan []ai.ToolResult, 1)
	var requests atomic.Int32
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		message := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "提醒操作已处理。"}
		if requests.Add(1) == 1 {
			message.Content = ""
			for i, raw := range args {
				message.ToolCalls = append(message.ToolCalls, openai.ToolCall{ID: fmt.Sprintf("call-%d", i), Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: tool.Name, Arguments: raw}})
			}
		} else {
			var results []ai.ToolResult
			for _, message := range req.Messages {
				if message.Role == openai.ChatMessageRoleTool {
					var result ai.ToolResult
					if err := json.Unmarshal([]byte(message.Content), &result); err != nil {
						t.Error(err)
					}
					results = append(results, result)
				}
			}
			resultChannel <- results
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: message, FinishReason: openai.FinishReasonStop}}})
	}))
	defer model.Close()
	config.C.AI = config.AIConfig{DeepSeekKey: "fixture", BaseURL: model.URL, Model: "fixture"}
	id := 900_000 + reminderDispatchSequence.Add(1)
	event := &bot.GroupMessageEvent{GroupID: id, UserID: id, MessageID: 1, Message: bot.Msg().Text(input).Build(), Sender: bot.Sender{Role: role}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := testEngine.Dispatch(ctx, &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}}, func(_ context.Context, reply string) error {
		if reply != "提醒操作已处理。" {
			t.Errorf("unexpected reply %q", reply)
		}
		// Prevent the unrelated memory task from outliving test global cleanup.
		cancel()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var results []ai.ToolResult
	select {
	case results = <-resultChannel:
	case <-time.After(time.Second):
		t.Fatal("model did not receive tool results")
	}
	if requests.Load() != 2 || len(results) != len(args) {
		t.Fatalf("model requests=%d results=%d, want 2 and %d", requests.Load(), len(results), len(args))
	}
	for i, call := range calls {
		// Every handler call must retain one original argument object.
		found := false
		for _, raw := range args {
			var original map[string]any
			_ = json.Unmarshal([]byte(raw), &original)
			if reflect.DeepEqual(call, original) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("handler call %d was mutated: %+v", i, call)
		}
	}
	return calls, results
}

func TestReminderValidationOutcomeThroughDispatch(t *testing.T) {
	for _, args := range []string{`{"action":"update"}`, `{"action":"remove"}`, `{"action":"create","content":"喝水","trigger_at":"明天"}`, `{"action":"create","content":"喝水","repeat":"daily","time":"bad"}`} {
		_, results := runReminderDispatch(t, []string{args}, reminderDispatchOptions{})
		for _, result := range results {
			if result.Status != ai.ToolRejected || result.PossibleSideEffects {
				t.Errorf("invalid reminder request must be rejected without side effects: %+v", result)
			}
		}
	}
}

func TestReminderReadOutcomesThroughDispatch(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		failure error
		rows    []db.Reminder
		status  ai.ToolStatus
		args    string
	}{
		{"database_failure", errors.New("private database diagnostic"), nil, ai.ToolFailed, `{"action":"list"}`},
		{"empty_list", nil, nil, ai.ToolEmpty, `{"action":"list"}`},
		{"nonempty_list", nil, []db.Reminder{{ID: 42, Message: "喝水"}}, ai.ToolSucceeded, `{"action":"list"}`},
		{"lookup_failure", errors.New("private database diagnostic"), nil, ai.ToolFailed, `{"action":"update","reminder_id":42}`},
		{"missing_reminder", gorm.ErrRecordNotFound, nil, ai.ToolRejected, `{"action":"update","reminder_id":42}`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			database, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1 user=fixture dbname=fixture sslmode=disable"}), &gorm.Config{DisableAutomaticPing: true, DryRun: true, Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := database.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			err = database.Callback().Query().Before("gorm:query").Register("fixture_reminders", func(tx *gorm.DB) {
				if tx.Statement.Table != "reminders" {
					return
				}
				if scenario.failure != nil {
					tx.AddError(scenario.failure)
					return
				}
				if rows, ok := tx.Statement.Dest.(*[]db.Reminder); ok {
					*rows = scenario.rows
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			_, results := runReminderDispatch(t, []string{scenario.args}, reminderDispatchOptions{database: database})
			result := results[0]
			if result.Status != scenario.status || result.PossibleSideEffects || strings.Contains(result.Content, "private") {
				t.Fatalf("incorrect query outcome: %+v", result)
			}
		})
	}
}
