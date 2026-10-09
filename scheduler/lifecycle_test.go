package scheduler

import (
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
)

func TestStopDrainsTimerCallbacksAndRejectsRetries(t *testing.T) {
	testScheduler.mu.Lock()
	testScheduler.oneShotSeq[42] = 9
	testScheduler.mu.Unlock()
	if !testScheduler.beginCallback(42, 9) {
		t.Fatal("callback rejected")
	}
	done := make(chan struct{})
	go func() { testScheduler.Stop(); close(done) }()
	// Synchronize on admission closure rather than timing the callback.
	deadline := time.After(3 * time.Second)
	for {
		testScheduler.mu.Lock()
		closed := testScheduler.stopped
		testScheduler.mu.Unlock()
		if closed {
			break
		}
		select {
		case <-deadline:
			t.Fatal("stop did not close admission")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if testScheduler.beginCallback(42, 9) {
		t.Fatal("stale callback admitted")
	}
	select {
	case <-done:
		t.Fatal("stop failed to wait")
	default:
	}
	testScheduler.mu.Lock()
	testScheduler.scheduleOneShotAttempt(nil, db.Reminder{ID: 42}, time.Hour)
	if len(testScheduler.oneShots) != 0 {
		t.Error("retry scheduled after stop")
	}
	testScheduler.mu.Unlock()
	testScheduler.callbacks.Done()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stop did not drain")
	}
	testScheduler.Stop()
	testScheduler.mu.Lock()
	testScheduler.stopped = false
	testScheduler.retired = nil
	testScheduler.mu.Unlock()
}

func TestManagersDoNotShareTimerState(t *testing.T) {
	first, second := New(nil), New(nil)
	first.mu.Lock()
	first.scheduleOneShotAttempt(nil, db.Reminder{ID: 1}, time.Hour)
	first.mu.Unlock()
	second.mu.Lock()
	second.scheduleOneShotAttempt(nil, db.Reminder{ID: 1}, time.Hour)
	second.mu.Unlock()
	defer second.Stop()
	first.Stop()
	second.mu.Lock()
	defer second.mu.Unlock()
	if second.stopped || len(second.oneShots) != 1 {
		t.Fatal("stopping another scheduler removed this timer")
	}
}

func TestManagersKeepTheirOwnReminderStore(t *testing.T) {
	testdb.Init(t)
	first := New(db.NewScheduleStore(testdb.Database))
	if _, err := first.Add(nil, 2, 1, "0 9 * * *", "first"); err != nil {
		t.Fatal(err)
	}
	testdb.Init(t)
	second := New(db.NewScheduleStore(testdb.Database))
	if _, err := second.Add(nil, 2, 1, "0 9 * * *", "second"); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		manager *Manager
		message string
	}{{first, "first"}, {second, "second"}} {
		rows, err := tt.manager.List(2, 1)
		if err != nil || len(rows) != 1 || rows[0].Message != tt.message {
			t.Fatalf("store crossed instance: %#v %v", rows, err)
		}
	}
}
