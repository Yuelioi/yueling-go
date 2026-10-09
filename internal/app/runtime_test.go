package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/internal/httpserver"
)

func TestStartupFailureDrainsOtherServicesAndWorkers(t *testing.T) {
	b := bot.New()
	r := newRuntime(context.Background(), b)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	r.start("http", func() error {
		close(started)
		return httpserver.Serve(r.ctx, &http.Server{Handler: http.NotFoundHandler()}, listener)
	})
	<-started
	workDone := make(chan struct{})
	b.Go(func() { <-b.Context().Done(); close(workDone) })
	canceled, waited := false, false
	r.cancelWorkers = func() { canceled = true }
	r.waitWorkers = func() {
		if !canceled {
			t.Error("wait preceded cancel")
		}
		<-workDone
		waited = true
	}
	bindFailure := errors.New("bind failure")
	r.start("failed", func() error { return bindFailure })
	drained, err := r.wait(nil, time.Second)
	if !drained || !errors.Is(err, bindFailure) || !waited {
		t.Fatalf("drained=%v err=%v waited=%v", drained, err, waited)
	}
	// A successful drain permits the caller to close DB; all listeners are released.
	rebound, err := net.Listen("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	rebound.Close()
}

func TestShutdownDeadlineDoesNotPermitDatabaseClose(t *testing.T) {
	b := bot.New()
	r := newRuntime(context.Background(), b)
	release := make(chan struct{})
	b.Go(func() { <-release })
	r.cancel()
	drained, err := r.wait(nil, 10*time.Millisecond)
	close(release)
	b.Wait()
	if drained || err == nil {
		t.Fatal("timed-out work was reported drained")
	}
}

func TestReloadCancelsServices(t *testing.T) {
	r := newRuntime(context.Background(), bot.New())
	r.start("service", func() error { <-r.ctx.Done(); return nil })
	reload := make(chan struct{}, 1)
	reload <- struct{}{}
	drained, err := r.wait(reload, time.Second)
	if !drained || err != nil {
		t.Fatalf("drained=%v err=%v", drained, err)
	}
}

func TestShutdownDeadlineIncludesCancellationHooks(t *testing.T) {
	r := newRuntime(context.Background(), bot.New())
	release, finished := make(chan struct{}), make(chan struct{})
	r.cancelWorkers = func() { <-release }
	r.waitWorkers = func() { close(finished) }
	r.cancel()
	drained, err := r.wait(nil, 10*time.Millisecond)
	close(release)
	<-finished
	if drained || err == nil {
		t.Fatal("blocked cancellation bypassed shutdown deadline")
	}
}
