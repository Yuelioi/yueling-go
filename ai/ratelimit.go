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
	for start < len(ts) && ts[start].Before(cutoff) {
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

	now := time.Now()
	cutoff := now.Add(-l.window)
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

var (
	limiterOnce sync.Once
	limiterInst *aiLimiter
)

// limiter lazily builds the process-wide AI rate limiter from config (mirrors
// llm()'s lazy init), so config is loaded before first use.
func limiter() *aiLimiter {
	limiterOnce.Do(func() {
		rl := config.C.AI.RateLimit
		limiterInst = newAILimiter(rl.UserPerMin, rl.GroupPerMin, time.Minute)
	})
	return limiterInst
}

// AllowAICall gates a user-triggered AI call. It returns (false, hint) when the
// per-user or per-group minute limit is exceeded; the caller replies hint and
// returns. Background/bot-initiated calls (proactive, memory) do not call this.
func AllowAICall(userID, groupID int64) (bool, string) {
	return limiter().Allow(userID, groupID)
}
