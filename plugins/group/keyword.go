package group

import (
	"strings"
	"sync"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
)

type replyCache struct {
	mu   sync.RWMutex
	data map[string]string // (keyword+group_suffix).lower → reply
}

var cache = &replyCache{data: make(map[string]string)}

func (c *replyCache) load() {
	rows, err := db.GetAllReplies()
	if err != nil {
		return
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		if r.Keyword == "" {
			continue
		}
		for _, kw := range strings.Split(r.Keyword, ",") {
			key := strings.ToLower(kw + r.Group)
			reply := r.Reply
			if strings.Contains(reply, "{}") {
				reply = strings.ReplaceAll(reply, "{}", kw)
			}
			m[key] = reply
		}
	}
	c.mu.Lock()
	c.data = m
	c.mu.Unlock()
}

func (c *replyCache) lookup(text string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[strings.ToLower(text)]
	return v, ok
}

func ReloadCache() { cache.load() }

func RegisterKeyword(b *bot.Bot) {
	cache.load()

	b.OnGroupMessage().Handle(func(ctx *bot.GroupContext) error {
		reply, ok := cache.lookup(ctx.Text())
		if !ok {
			return nil
		}
		return ctx.Reply(reply)
	})
}
