package tools

import (
	"errors"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/internal/testdb"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestManagementReadOutcomesThroughDispatch(t *testing.T) {
	for _, tt := range []struct{ name, input, action, table string }{
		{"manage_user_context", "我的资料", "list_profile", "user_profiles"},
		{"manage_user_context", "我的资料", "list_memories", "semantic_memories"},
		{"manage_todo", "待办", "list", "todo_items"},
		{"manage_group_rules", "本群规则", "list", "procedural_memories"},
		{"manage_group_knowledge", "添加到知识库", "list", "group_knowledges"},
	} {
		for _, failure := range []bool{false, true} {
			t.Run(tt.name+"/"+tt.action+"/"+map[bool]string{false: "empty", true: "failure"}[failure], func(t *testing.T) {
				database, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1 user=fixture dbname=fixture sslmode=disable"}), &gorm.Config{DisableAutomaticPing: true, DryRun: true, Logger: logger.Default.LogMode(logger.Silent)})
				if err != nil {
					t.Fatal(err)
				}
				sqlDB, _ := database.DB()
				t.Cleanup(func() { sqlDB.Close() })
				reads := 0
				if err := database.Callback().Query().Before("gorm:query").Register("outcome_fixture", func(tx *gorm.DB) {
					if tx.Statement.Table == tt.table {
						reads++
						if failure {
							tx.AddError(errors.New("private database diagnostic"))
						}
					}
				}); err != nil {
					t.Fatal(err)
				}
				_, results := runReminderDispatch(t, []string{`{"action":"` + tt.action + `"}`}, reminderDispatchOptions{database: database, toolName: tt.name, input: tt.input, role: "admin"})
				want := ai.ToolEmpty
				if failure {
					want = ai.ToolFailed
				}
				if reads == 0 {
					t.Fatal("fixture did not exercise persistence boundary")
				}
				if got := results[0]; got.Status != want || got.PossibleSideEffects || strings.Contains(got.Content, "private") {
					t.Fatalf("outcome=%+v want=%s", got, want)
				}
			})
		}
	}
}

func TestManagementValidationHasNoEffects(t *testing.T) {
	for _, tt := range []struct {
		name   string
		params map[string]any
	}{
		{"manage_user_context", map[string]any{"action": "set_profile", "key": "bad"}},
		{"manage_user_context", map[string]any{"action": "remember", "content": "密码123"}},
		{"manage_todo", map[string]any{"action": "done"}},
		{"manage_group_rules", map[string]any{"action": "remove"}},
		{"manage_group_knowledge", map[string]any{"action": "set_shortcuts"}},
		{"manage_daily_digest", map[string]any{"action": "set", "message_count": float64(1)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tool, _ := testEngine.Registry.GetTool(tt.name)
			got := tool.ResultHandler(&ai.ToolContext{Params: tt.params})
			if got.Status != ai.ToolRejected || got.PossibleSideEffects {
				t.Fatalf("outcome=%+v", got)
			}
		})
	}
}

func TestManagementWriteFailuresThroughDispatch(t *testing.T) {
	for _, tt := range []struct{ name, input, args, table string }{
		{"manage_user_context", "我的资料", `{"action":"set_profile","key":"称呼","value":"测试"}`, "user_profiles"},
		{"manage_todo", "待办", `{"action":"add","content":"测试"}`, "todo_items"},
		{"manage_group_rules", "添加群规则", `{"action":"add","rule":"测试"}`, "procedural_memories"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			testdb.Init(t)
			database := testdb.Database
			writes := 0
			if err := database.Callback().Create().Before("gorm:create").Register("write_failure", func(tx *gorm.DB) {
				if tx.Statement.Table == tt.table {
					writes++
					tx.AddError(errors.New("private write diagnostic"))
				}
			}); err != nil {
				t.Fatal(err)
			}
			_, results := runReminderDispatch(t, []string{tt.args, tt.args}, reminderDispatchOptions{database: database, toolName: tt.name, input: tt.input, role: "admin"})
			if writes != 1 {
				t.Fatalf("write attempted %d times", writes)
			}
			for _, got := range results {
				if got.Status != ai.ToolFailed || !got.PossibleSideEffects || strings.Contains(got.Content, "private") {
					t.Fatalf("outcome=%+v", got)
				}
			}
		})
	}
}
