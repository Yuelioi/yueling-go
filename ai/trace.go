package ai

import (
	"context"
	"errors"
	"fmt"
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
	diagnostic := ""
	if errors.As(err, &modelErr) {
		if modelErr.Detail != "" {
			diagnostic = " detail=" + modelErr.Detail
		}
		response := modelErr.Response
		if response.Attempts > 0 {
			diagnostic += fmt.Sprintf(" finish=%s max_tokens=%d completion_tokens=%d content_chars=%d reasoning_chars=%d attempts=%d",
				response.FinishReason, response.MaxTokens, response.CompletionTokens, response.ContentChars, response.ReasoningChars, response.Attempts)
		}
	}
	logx.Infof("[ai] stage=%s bot=%d group=%d user=%d message=%d status=%s http_status=%d duration_ms=%d error_type=%T%s",
		stage, request.bot, request.group, request.user, request.message, status, llm.Status(err), time.Since(started).Milliseconds(), err, diagnostic)
}

func (e *Engine) withModelUsage(ctx context.Context, groupID int64) context.Context {
	return llm.WithUsageObserver(ctx, func(usage llm.UsageRecord) {
		logx.Infof("[ai] stage=model_usage group=%d model=%q profile=%q feature=%q prompt_tokens=%d completion_tokens=%d cache_reported=%t cached_tokens=%d duration_ms=%d", groupID, usage.Model, usage.ProfileID, usage.Feature, usage.PromptTokens, usage.CompletionTokens, usage.CacheReported, usage.CachedTokens, usage.DurationMS)
		saveCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := e.ModelUsage.Record(saveCtx, groupID, usage); err != nil {
			logx.Warnf("[ai] usage persistence failed group=%d error=%v", groupID, err)
		}
	})
}
