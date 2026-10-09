package bot

import "context"

func (b *Bot) Context() context.Context {
	if b.lifetime == nil {
		return context.Background()
	}
	return b.lifetime
}

// Go admits owned background work until Stop closes admission.
func (b *Bot) Go(fn func()) bool {
	if !b.beginTask() {
		return false
	}
	go func() { defer b.tasks.Done(); fn() }()
	return true
}

func (b *Bot) beginTask() bool {
	b.taskMu.Lock()
	if b.stopping {
		b.taskMu.Unlock()
		return false
	}
	b.tasks.Add(1)
	b.taskMu.Unlock()
	return true
}

func (b *Bot) Stop() {
	b.taskMu.Lock()
	b.stopping = true
	if b.cancel != nil {
		b.cancel()
	}
	b.taskMu.Unlock()
}

// Wait must follow Stop; no new tasks can race the wait.
func (b *Bot) Wait() { b.tasks.Wait() }

func (a *BotAPI) Context() context.Context {
	if a != nil {
		if a.ctx != nil {
			return a.ctx
		}
		if a.parent != nil {
			return a.parent.Context()
		}
	}
	return context.Background()
}

func (a *BotAPI) Go(fn func()) bool {
	if a != nil {
		root := a.Connection()
		if root.owner != nil {
			return root.owner.Go(fn)
		}
	}
	go fn()
	return true
}
