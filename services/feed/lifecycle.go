package feed

import (
	"context"
	"time"

	"github.com/Yuelioi/yueling-go/services/logx"
)

func (m *Manager) Start(sender Sender) {
	if sender == nil {
		return
	}
	m.startMu.Lock()
	if m.stopped {
		m.startMu.Unlock()
		return
	}
	if m.cancel != nil {
		m.cancel()
		<-m.done
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	done := make(chan struct{})
	m.done = done
	m.startMu.Unlock()

	go func() {
		defer close(done)
		initial := time.NewTimer(initialPollDelay)
		defer initial.Stop()
		select {
		case <-ctx.Done():
			return
		case <-initial.C:
		}

		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for ctx.Err() == nil {
			result, err := m.pollAll(ctx, sender)
			if err != nil {
				logx.Errorf("[feed] polling failed: %v", err)
			} else if result.Updated > 0 || result.Delivered > 0 || result.Failed > 0 {
				logx.Infof("[feed] checked=%d updated=%d items=%d delivered=%d queued=%d failed=%d",
					result.Checked, result.Updated, result.Items, result.Delivered, result.Queued, result.Failed)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// Cancel closes polling admission without waiting for the in-flight poll.
func (m *Manager) Cancel() {
	m.startMu.Lock()
	defer m.startMu.Unlock()
	m.stopped = true
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *Manager) Stop() {
	m.Cancel()
	m.startMu.Lock()
	done := m.done
	m.startMu.Unlock()
	if done != nil {
		<-done
	}
}
