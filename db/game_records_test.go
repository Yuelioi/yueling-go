package db

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestCheckInConcurrentRequestsRewardOnce(t *testing.T) {
	initPostgresForTest(t)
	store := NewGameStore(DB)
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			gained, _, _, already, err := store.CheckIn(1, 2, "群友")
			if err != nil {
				t.Error(err)
				return
			}
			if !already {
				successes.Add(1)
				if gained != 10 {
					t.Errorf("gained=%d", gained)
				}
			}
		})
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("successful check-ins=%d", successes.Load())
	}
	row, err := store.GetOrCreateGameRecord(1, 2, "新昵称")
	if err != nil {
		t.Fatal(err)
	}
	if row.Score != 10 || row.MonthlyCheckIn != 1 || row.Streak != 1 || row.Nickname != "新昵称" {
		t.Fatalf("row=%+v", row)
	}
}
