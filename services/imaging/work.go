package imaging

// Image work is intentionally capped because bot dispatch itself is concurrent
// and decoded GIFs can occupy far more memory than their compressed input.
var workSlots = make(chan struct{}, 2)

// Run performs one bounded image job. Callers should include all decode,
// transform, render, and encode CPU work in the closure.
func Run[T any](work func() (T, error)) (T, error) {
	workSlots <- struct{}{}
	defer func() { <-workSlots }()
	return work()
}
