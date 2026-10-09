package ai

import (
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/config"
)

const (
	msgUserTooFrequent  = "你发消息太频繁了，稍后再试吧。"
	msgGroupTooFrequent = "本群 AI 用得太频繁了，稍后再试吧。"
)

// aiLimiter is a per-user + per-group sliding-window rate limiter. A limit of 0
// disables that window (unlimited). Both windows are checked before either is
// recorded, so a blocked call never consumes a slot in the other window.
type aiLimiter struct {
	mu           sync.Mutex
	userWindows  map[int64][]time.Time
	groupWindows map[int64][]time.Time
	userLimit    int
	groupLimit   int
	lastSweep    time.Time
	window       time.Duration
}

func newAILimiter(userLimit, groupLimit int, window time.Duration) *aiLimiter {
	return &aiLimiter{
		userWindows:  map[int64][]time.Time{},
		groupWindows: map[int64][]time.Time{},
		userLimit:    userLimit,
		groupLimit:   groupLimit,
		window:       window,
	}
}

// prune drops timestamps that fell out of the window and returns the remainder.
func prune(ts []time.Time, cutoff time.Time) []time.Time {
	start := 0
	for start < len(ts) && !ts[start].After(cutoff) {
		start++
	}
	return ts[start:]
}

// Allow reports whether an AI call from userID in groupID is within both the
// per-user and per-group windows, recording it when allowed. A blocked call
// returns a user-facing hint and records nothing. groupID <= 0 (private chat)
// skips the group window.
func (l *aiLimiter) Allow(userID, groupID int64) (bool, string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.allow(userID, groupID, time.Now())
}

func (l *aiLimiter) allow(userID, groupID int64, now time.Time) (bool, string) {
	cutoff := now.Add(-l.window)
	if l.lastSweep.IsZero() || now.Sub(l.lastSweep) >= l.window {
		for _, windows := range []map[int64][]time.Time{l.userWindows, l.groupWindows} {
			for id, timestamps := range windows {
				live := prune(timestamps, cutoff)
				if len(live) == 0 {
					delete(windows, id)
				} else {
					windows[id] = live
				}
			}
		}
		l.lastSweep = now
	}
	if l.userLimit <= 0 {
		clear(l.userWindows)
	}
	if l.groupLimit <= 0 {
		clear(l.groupWindows)
	}
	checkGroup := l.groupLimit > 0 && groupID > 0

	if l.userLimit > 0 {
		u := prune(l.userWindows[userID], cutoff)
		l.userWindows[userID] = u
		if len(u) >= l.userLimit {
			return false, msgUserTooFrequent
		}
	}
	if checkGroup {
		g := prune(l.groupWindows[groupID], cutoff)
		l.groupWindows[groupID] = g
		if len(g) >= l.groupLimit {
			return false, msgGroupTooFrequent
		}
	}

	if l.userLimit > 0 {
		l.userWindows[userID] = append(l.userWindows[userID], now)
	}
	if checkGroup {
		l.groupWindows[groupID] = append(l.groupWindows[groupID], now)
	}
	return true, ""
}

func (e *Engine) AllowAICall(userID, groupID int64) (bool, string) {
	rl := config.Current().AI.RateLimit
	e.limiter.mu.Lock()
	defer e.limiter.mu.Unlock()
	e.limiter.userLimit, e.limiter.groupLimit = rl.UserPerMin, rl.GroupPerMin
	return e.limiter.allow(userID, groupID, time.Now())
}
