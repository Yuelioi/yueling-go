package feed

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
)

func TestStopClosesPollingAdmission(t *testing.T) {
	m := NewManager(db.NewFeedStore(testdb.Database), nil)
	m.Start(&recordingSender{})
	first := m.done
	m.Start(&recordingSender{})
	select {
	case <-first:
	default:
		t.Fatal("replacement left old poller running")
	}
	current := m.done
	m.Stop()
	m.Stop()
	select {
	case <-current:
	default:
		t.Fatal("stop did not wait")
	}
	m.Start(&recordingSender{})
	if m.done != current {
		t.Fatal("started poller after stop")
	}
}

func TestCanceledPollStopsFetchesBeforeWritingDatabase(t *testing.T) {
	m := NewManager(db.NewFeedStore(testdb.Database), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, maxConcurrentFetches)
	m.fetchContext = func(ctx context.Context, _ string) (*Feed, error) {
		started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	rows := make([]db.FeedSubscription, 8)
	for i := range rows {
		rows[i] = db.FeedSubscription{URL: fmt.Sprintf("https://example.com/%d", i)}
	}
	done := make(chan struct{})
	go func() { defer close(done); m.pollRowsContext(ctx, &recordingSender{}, rows, nil) }()
	for i := 0; i < maxConcurrentFetches; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("fetch did not start")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("fetch cancellation did not drain")
	}
	select {
	case <-started:
		t.Fatal("started queued fetch after cancel")
	default:
	}
}
