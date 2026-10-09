package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
	openai "github.com/sashabaranov/go-openai"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func captureConfirmation(parent context.Context, gctx *bot.GroupContext) (string, bool, error) {
	var reply string
	handled, err := testEngine.handleConfirmation(parent, gctx, func(ctx context.Context, text string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		reply = text
		return nil
	})
	return reply, handled, err
}

func TestHandleConfirmationExecutesPendingTool(t *testing.T) {
	oldRegistry := testEngine.Registry
	oldConfirms := testEngine.Confirms
	oldSessions := testEngine.Sessions
	oldName := config.C.Bot.Name
	t.Cleanup(func() {
		testEngine.Registry = oldRegistry
		testEngine.Confirms = oldConfirms
		testEngine.Sessions = oldSessions
		config.C.Bot.Name = oldName
	})
	testEngine.Registry = &Registry{tools: map[string]*ToolMeta{}}
	testEngine.Confirms = &ConfirmManager{pending: map[string]*PendingAction{}}
	testEngine.Sessions = &SessionManager{sessions: map[string]*Session{}}
	config.C.Bot.Name = "月灵"

	executed := false
	testEngine.Registry.Register(ToolMeta{
		Name:            "confirmed_test",
		ConfirmRequired: true,
		Handler: func(ctx *ToolContext) (string, error) {
			executed = ctx.String("target") == "ok"
			return "操作已执行", nil
		},
	})
	actionID, code := testEngine.Confirms.Store(42, 100, "confirmed_test", map[string]any{"target": "ok"})
	event := &bot.GroupMessageEvent{
		SelfID: 1, GroupID: 100, UserID: 42,
		Message: bot.Msg().Text("月灵 确认 " + code + " " + actionID).Build(),
		Sender:  bot.Sender{Role: "member"},
	}
	reply, handled, err := captureConfirmation(context.Background(), &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}})
	if err != nil || !handled || reply != "操作已执行" || !executed {
		t.Fatalf("reply=%q handled=%v executed=%v err=%v", reply, handled, executed, err)
	}
	if session := testEngine.Sessions.Get(100, 42); len(session.Messages) != 2 || session.Messages[1].Content != "操作已执行" {
		t.Fatalf("confirmed result was not recorded in session: %#v", session.Messages)
	}
	if _, ok := testEngine.Confirms.Verify(actionID, code, 42, 100); ok {
		t.Fatal("confirmed action could be reused")
	}
}

func TestResetStopsActiveConfirmationAndDiscardsItsLateReply(t *testing.T) {
	oldRegistry, oldConfirms, oldSessions := testEngine.Registry, testEngine.Confirms, testEngine.Sessions
	t.Cleanup(func() {
		testEngine.Registry, testEngine.Confirms, testEngine.Sessions = oldRegistry, oldConfirms, oldSessions
	})
	testEngine.Registry = &Registry{tools: map[string]*ToolMeta{}}
	testEngine.Confirms = &ConfirmManager{pending: map[string]*PendingAction{}}
	testEngine.Sessions = &SessionManager{}
	started := make(chan struct{})
	testEngine.Registry.Register(ToolMeta{Name: "reset_confirmation", Handler: func(ctx *ToolContext) (string, error) {
		close(started)
		<-ctx.Context().Done()
		// Simulate an API returning a late success after cancellation.
		return "旧操作的迟到回复", nil
	}})
	actionID, code := testEngine.Confirms.Store(42, 100, "reset_confirmation", nil)
	event := &bot.GroupMessageEvent{
		SelfID: 1, GroupID: 100, UserID: 42,
		Message: bot.Msg().Text("确认 " + code + " " + actionID).Build(),
		Sender:  bot.Sender{Role: "member"},
	}
	finished := make(chan string, 1)
	go func() {
		reply, _, _ := captureConfirmation(context.Background(), &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}})
		finished <- reply
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("confirmation did not start")
	}
	oldSession := testEngine.Sessions.Get(100, 42)
	if _, handled := testEngine.handleLocalControl(100, 42, "新对话"); !handled {
		t.Fatal("reset command was not handled")
	}
	fresh := testEngine.Sessions.Get(100, 42)
	select {
	case reply := <-finished:
		if reply != "" || len(oldSession.Messages) != 0 || len(fresh.Messages) != 0 {
			t.Fatalf("reset leaked confirmed reply or history: %q", reply)
		}
	case <-time.After(time.Second):
		t.Fatal("reset did not cancel the confirmed tool context")
	}
}

func TestResetDuringConfirmationPermissionCheckKeepsOriginalSession(t *testing.T) {
	for _, queryFailed := range []bool{false, true} {
		name := "allowed"
		if queryFailed {
			name = "query_failed"
		}
		t.Run(name, func(t *testing.T) {
			oldRegistry, oldConfirms, oldSessions, oldDB := testEngine.Registry, testEngine.Confirms, testEngine.Sessions, testdb.Database
			t.Cleanup(func() {
				testEngine.Registry, testEngine.Confirms, testEngine.Sessions, testdb.Database = oldRegistry, oldConfirms, oldSessions, oldDB
			})
			testEngine.Registry = &Registry{tools: map[string]*ToolMeta{}}
			testEngine.Confirms = &ConfirmManager{pending: map[string]*PendingAction{}}
			testEngine.Sessions = &SessionManager{}
			original := testEngine.Sessions.Get(100, 42)
			executed := false
			testEngine.Registry.Register(ToolMeta{Name: "reset_during_confirmation_check", PluginID: 123, Handler: func(*ToolContext) (string, error) {
				executed = true
				return "旧操作已执行", nil
			}})
			actionID, code := testEngine.Confirms.Store(42, 100, "reset_during_confirmation_check", nil)

			// DryRun exercises the real authorization query without opening a DB connection.
			database, err := gorm.Open(postgres.New(postgres.Config{
				DSN: "host=127.0.0.1 port=1 user=fixture dbname=fixture sslmode=disable",
			}), &gorm.Config{DisableAutomaticPing: true, DryRun: true})
			if err != nil {
				t.Fatal(err)
			}
			testdb.Database = database
			oldGroups := testEngine.Groups
			testEngine.Groups = db.NewGroupStore(database)
			t.Cleanup(func() { testEngine.Groups = oldGroups })
			oldAffinity := testEngine.Affinity
			testEngine.Affinity = NewAffinityManager(db.NewAffinityStore(database))
			t.Cleanup(func() { testEngine.Affinity = oldAffinity })
			var fresh *Session
			if err := database.Callback().Query().Before("gorm:query").Register("reset_confirmation_session", func(query *gorm.DB) {
				testEngine.Sessions.Delete(100, 42)
				fresh = testEngine.Sessions.Get(100, 42)
				if queryFailed {
					query.AddError(errors.New("fixture plugin check failed"))
				}
			}); err != nil {
				t.Fatal(err)
			}

			reply, handled, err := captureConfirmation(context.Background(), confirmationGroup(actionID, code))
			if fresh == nil || fresh == original || !original.invalidated() || fresh.invalidated() {
				t.Fatal("authorization query did not replace the original session with an independent lifetime")
			}
			if !handled || err != nil || executed || reply != "" || len(original.Messages) != 0 || len(fresh.Messages) != 0 {
				t.Fatalf("reset leaked confirmation: handled=%v err=%v executed=%v reply=%q original_history=%d fresh_history=%d",
					handled, err, executed, reply, len(original.Messages), len(fresh.Messages))
			}
		})
	}
}

func TestNewConfirmationReplacesPreviousMatchingAction(t *testing.T) {
	manager := &ConfirmManager{pending: map[string]*PendingAction{}}
	oldID, oldCode := manager.Store(42, 100, "ban", map[string]any{"user_id": "1"})
	newID, newCode := manager.Store(42, 100, "ban", map[string]any{"user_id": "2"})
	if _, ok := manager.Verify(oldID, oldCode, 42, 100); ok {
		t.Fatal("superseded confirmation remained valid")
	}
	if _, ok := manager.Verify(newID, newCode, 42, 100); !ok {
		t.Fatal("latest confirmation is not valid")
	}
}

func TestExecuteToolCreatesDescriptiveConfirmation(t *testing.T) {
	oldRegistry := testEngine.Registry
	oldConfirms := testEngine.Confirms
	oldName := config.C.Bot.Name
	t.Cleanup(func() {
		testEngine.Registry = oldRegistry
		testEngine.Confirms = oldConfirms
		config.C.Bot.Name = oldName
	})
	testEngine.Registry = &Registry{tools: map[string]*ToolMeta{}}
	testEngine.Confirms = &ConfirmManager{pending: map[string]*PendingAction{}}
	config.C.Bot.Name = "月灵"
	testEngine.Registry.Register(ToolMeta{
		Name:            "dangerous_test",
		Params:          []Param{{Name: "target", Type: "string", Required: true}},
		Description:     "测试危险操作",
		ConfirmRequired: true,
	})

	session := newSession(42, 100)
	event := &bot.GroupMessageEvent{GroupID: 100, UserID: 42}
	reply := testEngine.executeTool(context.Background(), nil, event, session, PermMember, openai.ToolCall{
		Function: openai.FunctionCall{Name: "dangerous_test", Arguments: `{"target":"ok"}`},
	}, nil)
	if !strings.Contains(reply.Content, "测试危险操作") || !strings.Contains(reply.Content, "月灵 确认") {
		t.Fatalf("confirmation reply = %+v", reply)
	}
	if session.canCall("dangerous_test") {
		t.Fatal("high-risk tool could be requested repeatedly in one turn")
	}
}

func TestConfirmationIsBoundToGroup(t *testing.T) {
	manager := &ConfirmManager{pending: map[string]*PendingAction{}}
	actionID, code := manager.Store(42, 100, "test", nil)
	if _, ok := manager.Verify(actionID, code, 42, 200); ok {
		t.Fatal("confirmation succeeded in another group")
	}
	if _, ok := manager.Verify(actionID, code, 42, 100); !ok {
		t.Fatal("valid confirmation was consumed by wrong-group attempt")
	}
}

func TestConfirmationPreservesOriginalMessageContext(t *testing.T) {
	oldRegistry, oldConfirms, oldSessions := testEngine.Registry, testEngine.Confirms, testEngine.Sessions
	t.Cleanup(func() {
		testEngine.Registry, testEngine.Confirms, testEngine.Sessions = oldRegistry, oldConfirms, oldSessions
	})
	testEngine.Registry = &Registry{tools: map[string]*ToolMeta{}}
	testEngine.Confirms = &ConfirmManager{pending: map[string]*PendingAction{}}
	testEngine.Sessions = &SessionManager{sessions: map[string]*Session{}}

	var mentioned []int64
	testEngine.Registry.Register(ToolMeta{Name: "context_confirmation", Handler: func(ctx *ToolContext) (string, error) {
		mentioned = ctx.MentionedUserIDs()
		value, _ := ctx.GetState("history")
		return value.(string), nil
	}})
	original := &bot.GroupMessageEvent{
		SelfID: 1, GroupID: 100, UserID: 42,
		Message: bot.Msg().At(123).Text("禁言").Build(),
	}
	actionID, code := testEngine.Confirms.StoreWithContext(42, 100, "context_confirmation", map[string]any{}, original, map[string]any{"history": "kept"}, testEngine.Sessions.Get(100, 42))
	confirmation := &bot.GroupMessageEvent{
		SelfID: 1, GroupID: 100, UserID: 42,
		Message: bot.Msg().Text("月灵 确认 " + code + " " + actionID).Build(),
		Sender:  bot.Sender{Role: "member"},
	}
	reply, handled, err := captureConfirmation(context.Background(), &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: confirmation}})
	if err != nil || !handled || reply != "kept" || len(mentioned) != 1 || mentioned[0] != 123 {
		t.Fatalf("reply=%q handled=%v mentioned=%v err=%v", reply, handled, mentioned, err)
	}
}

func TestCanceledConfirmationDoesNotConsumeCode(t *testing.T) {
	oldRegistry, oldConfirms, oldSessions := testEngine.Registry, testEngine.Confirms, testEngine.Sessions
	t.Cleanup(func() {
		testEngine.Registry, testEngine.Confirms, testEngine.Sessions = oldRegistry, oldConfirms, oldSessions
	})
	testEngine.Registry = &Registry{tools: map[string]*ToolMeta{}}
	testEngine.Confirms = &ConfirmManager{pending: map[string]*PendingAction{}}
	testEngine.Sessions = &SessionManager{}
	testEngine.Registry.Register(ToolMeta{Name: "cancel_before_confirmation", Handler: func(*ToolContext) (string, error) {
		t.Fatal("pre-canceled confirmation executed")
		return "", nil
	}})
	actionID, code := testEngine.Confirms.Store(42, 100, "cancel_before_confirmation", nil)
	gctx := confirmationGroup(actionID, code)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reply, handled, err := captureConfirmation(ctx, gctx)
	if !handled || reply != "" || !errors.Is(err, context.Canceled) {
		t.Fatalf("reply=%q handled=%v err=%v", reply, handled, err)
	}
	if _, ok := testEngine.Confirms.Verify(actionID, code, 42, 100); !ok {
		t.Fatal("pre-canceled confirmation consumed its code")
	}
}

func TestParentCancellationStopsActiveConfirmation(t *testing.T) {
	oldRegistry, oldConfirms, oldSessions := testEngine.Registry, testEngine.Confirms, testEngine.Sessions
	t.Cleanup(func() {
		testEngine.Registry, testEngine.Confirms, testEngine.Sessions = oldRegistry, oldConfirms, oldSessions
	})
	testEngine.Registry = &Registry{tools: map[string]*ToolMeta{}}
	testEngine.Confirms = &ConfirmManager{pending: map[string]*PendingAction{}}
	testEngine.Sessions = &SessionManager{}
	started := make(chan struct{})
	testEngine.Registry.Register(ToolMeta{Name: "cancel_active_confirmation", Handler: func(ctx *ToolContext) (string, error) {
		close(started)
		<-ctx.Context().Done()
		return "迟到成功", nil
	}})
	actionID, code := testEngine.Confirms.Store(42, 100, "cancel_active_confirmation", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type outcome struct {
		reply   string
		handled bool
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		reply, handled, err := captureConfirmation(ctx, confirmationGroup(actionID, code))
		done <- outcome{reply, handled, err}
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("confirmation did not start")
	}
	cancel()
	select {
	case got := <-done:
		if got.reply != "" || !got.handled || !errors.Is(got.err, context.Canceled) {
			t.Fatalf("outcome=%+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("parent cancellation did not reach confirmed handler")
	}
	if len(testEngine.Sessions.Get(100, 42).Messages) != 0 {
		t.Fatal("canceled confirmation wrote a late success into history")
	}
}

func TestResetCancelsConfirmationDuringDelivery(t *testing.T) {
	oldRegistry, oldConfirms, oldSessions := testEngine.Registry, testEngine.Confirms, testEngine.Sessions
	t.Cleanup(func() {
		testEngine.Registry, testEngine.Confirms, testEngine.Sessions = oldRegistry, oldConfirms, oldSessions
	})
	testEngine.Registry = &Registry{tools: map[string]*ToolMeta{}}
	testEngine.Confirms = &ConfirmManager{pending: map[string]*PendingAction{}}
	testEngine.Sessions = &SessionManager{}
	testEngine.Registry.Register(ToolMeta{Name: "cancel_confirmation_delivery", Handler: func(*ToolContext) (string, error) {
		return "已执行", nil
	}})
	actionID, code := testEngine.Confirms.Store(42, 100, "cancel_confirmation_delivery", nil)
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := testEngine.handleConfirmation(context.Background(), confirmationGroup(actionID, code), func(ctx context.Context, text string) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("confirmation delivery did not start")
	}
	testEngine.Sessions.Delete(100, 42)
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("delivery did not observe reset: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("confirmation delivery escaped the conversation lifetime")
	}
	if len(testEngine.Sessions.Get(100, 42).Messages) != 0 {
		t.Fatal("confirmed history crossed reset")
	}
}

func TestConfirmationFailureDoesNotExposeInternalError(t *testing.T) {
	oldRegistry, oldConfirms, oldSessions := testEngine.Registry, testEngine.Confirms, testEngine.Sessions
	t.Cleanup(func() {
		testEngine.Registry, testEngine.Confirms, testEngine.Sessions = oldRegistry, oldConfirms, oldSessions
	})
	testEngine.Registry = &Registry{tools: map[string]*ToolMeta{}}
	testEngine.Confirms = &ConfirmManager{pending: map[string]*PendingAction{}}
	testEngine.Sessions = &SessionManager{}
	testEngine.Registry.Register(ToolMeta{Name: "failed_confirmation", Handler: func(*ToolContext) (string, error) {
		return "", errors.New("sensitive-provider-body")
	}})
	actionID, code := testEngine.Confirms.Store(42, 100, "failed_confirmation", nil)
	reply, handled, err := captureConfirmation(context.Background(), confirmationGroup(actionID, code))
	if err != nil || !handled || !strings.Contains(reply, "未能确认完成") || strings.Contains(reply, "sensitive") {
		t.Fatalf("reply=%q handled=%v err=%v", reply, handled, err)
	}
	if len(testEngine.Sessions.Get(100, 42).Messages) != 0 {
		t.Fatal("failed confirmation stored successful history")
	}
}

func confirmationGroup(actionID, code string) *bot.GroupContext {
	return &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: &bot.GroupMessageEvent{
		SelfID: 1, GroupID: 100, UserID: 42,
		Message: bot.Msg().Text("确认 " + code + " " + actionID).Build(),
		Sender:  bot.Sender{Role: "member"},
	}}}
}
