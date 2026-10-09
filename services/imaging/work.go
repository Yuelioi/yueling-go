package imaging

import "context"

// Image work is intentionally capped because bot dispatch itself is concurrent
// and decoded GIFs can occupy far more memory than their compressed input.
var workSlots = make(chan struct{}, 2)

// Run performs one bounded image job. Callers should include all decode,
// transform, render, and encode CPU work in the closure.
func Run[T any](ctx context.Context, work func() (T, error)) (T, error) {
	var zero T
	select {
	case workSlots <- struct{}{}:
	case <-ctx.Done():
		return zero, ctx.Err()
	}
	defer func() { <-workSlots }()
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return work()
}
