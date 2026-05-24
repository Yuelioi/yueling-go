package funny

import (
	"sync"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/bot/rule"
)

// repeater tracks consecutive identical messages per group and repeats on the 3rd.
type repeater struct {
	mu    sync.Mutex
	last  map[int64]string // groupID → last text
	count map[int64]int    // groupID → consecutive count
}

var rep = &repeater{
	last:  map[int64]string{},
	count: map[int64]int{},
}

func (r *repeater) check(groupID int64, text string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if text == "" || r.last[groupID] != text {
		r.last[groupID] = text
		r.count[groupID] = 1
		return false
	}
	r.count[groupID]++
	if r.count[groupID] == 3 {
		r.count[groupID] = 0 // reset to avoid repeated triggers
		return true
	}
	return false
}

func RegisterRepeater(b *bot.Bot) {
	b.OnGroupMessage().
		When(rule.NoCommand).
		Priority(1). // low priority, runs after everything else
		Handle(func(ctx *bot.GroupContext) error {
			text := ctx.Text()
			if text == "" {
				return nil
			}
			if rep.check(ctx.GroupID(), text) {
				_, err := ctx.SendGroupMsg(ctx.GroupID(), ctx.Message())
				return err
			}
			return nil
		})
}
