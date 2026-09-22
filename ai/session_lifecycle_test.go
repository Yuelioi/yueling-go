package ai

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestSessionResetCancelsQueuedTurn(t *testing.T) {
	manager := &SessionManager{}
	session := manager.Get(100, 42)
	if err := session.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer session.release()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waiting := make(chan error, 1)
	go func() {
		err := session.acquire(ctx)
		if err == nil {
			session.release()
		}
		waiting <- err
	}()
	manager.Delete(100, 42)
	select {
	case err := <-waiting:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("queued turn after reset: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("reset left a queued request waiting for the old conversation")
	}
}

func TestSessionResetCancelsActiveTurnAndKeepsOtherSessions(t *testing.T) {
	manager := &SessionManager{}
	old := manager.Get(100, 42)
	ctx, stop := old.turnContext(context.Background())
	defer stop()
	if err := old.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	defer old.release()
	otherGroup := manager.Get(999, 42)
	otherCtx, stopOther := otherGroup.turnContext(context.Background())
	defer stopOther()
	otherUser := manager.Get(100, 43)
	userCtx, stopUser := otherUser.turnContext(context.Background())
	defer stopUser()

	if !manager.Delete(100, 42) {
		t.Fatal("reset did not find active conversation")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("running request was not canceled by reset")
	}
	if !old.invalidated() || otherCtx.Err() != nil || userCtx.Err() != nil {
		t.Fatal("reset invalidated the wrong conversation scope")
	}

	fresh := manager.Get(100, 42)
	freshCtx, stopFresh := fresh.turnContext(context.Background())
	defer stopFresh()
	if fresh == old || freshCtx.Err() != nil || fresh.invalidated() {
		t.Fatal("new conversation inherited old cancellation")
	}
	if err := fresh.acquire(freshCtx); err != nil {
		t.Fatalf("new conversation blocked on old in-flight request: %v", err)
	}
	fresh.release()
	stop()
	if freshCtx.Err() != nil {
		t.Fatal("old request cleanup canceled the new conversation")
	}
}

func TestSessionExpiredLifetimeCannotPublishIntoReplacement(t *testing.T) {
	manager := &SessionManager{}
	old := manager.Get(100, 42)
	ctx, stop := old.turnContext(context.Background())
	defer stop()
	manager.mu.Lock()
	old.expiresAt = time.Now().Add(-time.Second)
	manager.mu.Unlock()
	fresh := manager.Get(100, 42)
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("expired conversation still runs after replacement")
	}
	if fresh == old || fresh.invalidated() {
		t.Fatal("expired conversation was reused")
	}
	if err := old.acquire(context.Background()); !errors.Is(err, context.Canceled) {
		if err == nil {
			old.release()
		}
		t.Fatalf("retired conversation accepted a new request: %v", err)
	}
}

func TestSessionTurnCleanupDoesNotCancelConversation(t *testing.T) {
	session := newSession(42, 100)
	parent, cancelParent := context.WithCancel(context.Background())
	ctx, stop := session.turnContext(parent)
	cancelParent()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("caller cancellation did not reach request")
	}
	stop()
	followup, stopFollowup := session.turnContext(context.Background())
	defer stopFollowup()
	if session.invalidated() || followup.Err() != nil {
		t.Fatal("one canceled request retired the entire conversation")
	}
}

func TestSessionConcurrentResetAndAcquire(t *testing.T) {
	manager := &SessionManager{}
	var workers sync.WaitGroup
	for i := 0; i < 12; i++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			group := int64(worker % 3)
			for j := 0; j < 40; j++ {
				session := manager.Get(group, 42)
				ctx, stop := session.turnContext(context.Background())
				if err := session.acquire(ctx); err == nil {
					session.release()
				} else if !errors.Is(err, context.Canceled) {
					t.Errorf("acquire failed: %v", err)
				}
				stop()
				manager.Delete(group, 42)
			}
		}(i)
	}
	workers.Wait()
}
