package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/logx"
)

// runtime owns serving goroutines and drains all database users before Run closes DB.
type runtime struct {
	ctx           context.Context
	cancel        context.CancelFunc
	bot           *bot.Bot
	running       sync.WaitGroup
	mu            sync.Mutex
	failures      []error
	cancelWorkers func()
	waitWorkers   func()
}

func newRuntime(parent context.Context, b *bot.Bot) *runtime {
	ctx, cancel := context.WithCancel(parent)
	return &runtime{ctx: ctx, cancel: cancel, bot: b, cancelWorkers: func() {}, waitWorkers: func() {}}
}

func (r *runtime) start(name string, run func() error) {
	r.running.Add(1)
	go func() {
		defer r.running.Done()
		err := run()
		if err == nil && r.ctx.Err() == nil {
			err = errors.New("service stopped unexpectedly")
		}
		if err != nil {
			r.mu.Lock()
			r.failures = append(r.failures, fmt.Errorf("%s: %w", name, err))
			r.mu.Unlock()
			r.cancel()
		}
	}()
}

func (r *runtime) wait(reload <-chan struct{}, timeout time.Duration) (bool, error) {
	select {
	case <-r.ctx.Done():
	case <-reload:
		logx.Infof("[app] restart requested; exiting for supervisor restart (manual restart required without supervisor)")
	}
	r.cancel()
	r.bot.Stop()
	logx.Infof("[bot] shutting down...")
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.cancelWorkers()
		r.running.Wait()
		r.bot.Wait()
		r.waitWorkers()
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		return false, fmt.Errorf("shutdown exceeded %s; database left open until process exit", timeout)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	result := errors.Join(r.failures...)
	return !errors.Is(result, context.DeadlineExceeded), result
}
