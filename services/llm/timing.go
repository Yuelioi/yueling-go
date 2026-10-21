package llm

import (
	"context"
	"time"
)

type timingObserverKey struct{}

func WithTimingObserver(ctx context.Context, observe func(string, time.Time, error)) context.Context {
	return context.WithValue(ctx, timingObserverKey{}, observe)
}
func observeTiming(ctx context.Context, stage string, started time.Time, err error) {
	if observe, ok := ctx.Value(timingObserverKey{}).(func(string, time.Time, error)); ok {
		observe(stage, started, err)
	}
}
