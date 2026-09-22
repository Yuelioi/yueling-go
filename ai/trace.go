package ai

import (
	"context"
	"errors"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/chatsummary"
	"github.com/Yuelioi/yueling-go/services/llm"
	"github.com/Yuelioi/yueling-go/services/logx"
)

type requestTraceKey struct{}
type requestTrace struct{ bot, group, user, message int64 }

func withRequestTrace(ctx context.Context, event *bot.GroupMessageEvent) context.Context {
	return context.WithValue(ctx, requestTraceKey{}, requestTrace{event.SelfID, event.GroupID, event.UserID, int64(event.MessageID)})
}

// Trace only execution metadata. Provider bodies, prompts and tool arguments
// must never be included in stage logs.
func traceStage(ctx context.Context, stage string, started time.Time, err error) {
	request, ok := ctx.Value(requestTraceKey{}).(requestTrace)
	if !ok {
		return
	}
	status := "ok"
	var modelErr *llm.Error
	switch {
	case err == nil:
	case errors.Is(err, context.Canceled):
		status = "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		status = "timeout"
	case errors.Is(err, chatsummary.ErrNoRecords):
		status = "no_records"
	case errors.As(err, &modelErr):
		status = string(modelErr.Kind)
	default:
		status = "failed"
	}
	logx.Infof("[ai] stage=%s bot=%d group=%d user=%d message=%d status=%s http_status=%d duration_ms=%d error_type=%T",
		stage, request.bot, request.group, request.user, request.message, status, llm.Status(err), time.Since(started).Milliseconds(), err)
}
