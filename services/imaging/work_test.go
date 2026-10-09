package imaging

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCanceledQueuedWorkDoesNotRun(t *testing.T) {
	for range cap(workSlots) {
		workSlots <- struct{}{}
	}
	t.Cleanup(func() {
		for range cap(workSlots) {
			<-workSlots
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := Run(ctx, func() (int, error) { t.Error("canceled image job ran"); return 0, nil })
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued image job ignored cancellation")
	}
}
