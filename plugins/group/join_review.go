package group

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/bot/perm"
	"github.com/Yuelioi/yueling-go/db"
)

type joinDecision int

const (
	decisionNone joinDecision = iota
	decisionApprove
	decisionReject
)

func decideJoin(comment string, allow, deny []string) joinDecision {
	if comment == "" {
		return decisionNone
	}
	for _, kw := range deny {
		if kw != "" && strings.Contains(comment, kw) {
			return decisionReject
		}
	}
	for _, kw := range allow {
		if kw == "*" || (kw != "" && strings.Contains(comment, kw)) {
			return decisionApprove
		}
	}
	return decisionNone
}

func parseKeywordArg(raw string) (add bool, keywords []string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, nil, false
	}
	switch raw[0] {
	case '+':
		add = true
	case '-':
		add = false
	default:
		return false, nil, false
	}
	rest := strings.ReplaceAll(raw[1:], "，", ",")
	for _, p := range strings.Split(rest, ",") {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			keywords = append(keywords, p)
		}
	}
	if len(keywords) == 0 {
		return false, nil, false
	}
	return add, keywords, true
}

const joinDenyReason = "申请未通过审核"

type joinRule struct {
	allow []string
	deny  []string
}

type joinCache struct {
	mu   sync.RWMutex
	data map[int64]*joinRule
}

var jcache = &joinCache{data: map[int64]*joinRule{}}

func (c *joinCache) load() {
	rows, err := db.GetAllGroupJoinRules()
	if err != nil {
		return
	}
	m := make(map[int64]*joinRule)
	for _, r := range rows {
		jr := m[r.GroupID]
		if jr == nil {
			jr = &joinRule{}
			m[r.GroupID] = jr
		}
		switch r.Action {
		case db.JoinActionAllow:
			jr.allow = append(jr.allow, r.Keyword)
		case db.JoinActionDeny:
			jr.deny = append(jr.deny, r.Keyword)
		}
	}
	c.mu.Lock()
	c.data = m
	c.mu.Unlock()
}

func (c *joinCache) get(groupID int64) *joinRule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data[groupID]
}

func formatJoinList(groupID int64) string {
	rule := jcache.get(groupID)
	var allow, deny []string
	if rule != nil {
		allow, deny = rule.allow, rule.deny
	}
	show := func(s []string) string {
		if len(s) == 0 {
			return "（空）"
		}
		return strings.Join(s, "、")
	}
	return fmt.Sprintf("加群审核（本群）\n通过词：%s\n拒绝词：%s\n用法：加群白名单 +词1,词2 / -词；加群黑名单 +词 / -词；白名单加 * 表示任意理由放行",
		show(allow), show(deny))
}

func joinListHandler(action, label string) func(*bot.CommandContext) error {
	return func(ctx *bot.CommandContext) error {
		raw := strings.Join(ctx.Args, " ")
		if strings.TrimSpace(raw) == "" {
			return ctx.Reply(formatJoinList(ctx.GroupID()))
		}
		add, keywords, ok := parseKeywordArg(raw)
		if !ok {
			return ctx.Reply("用法：加群" + label + " +词1,词2  添加；加群" + label + " -词  删除")
		}
		n := 0
		for _, kw := range keywords {
			var changed bool
			var err error
			if add {
				changed, err = db.AddGroupJoinRule(ctx.GroupID(), action, kw)
			} else {
				changed, err = db.DeleteGroupJoinRule(ctx.GroupID(), action, kw)
			}
			if err != nil {
				return ctx.Reply("操作失败：" + err.Error())
			}
			if changed {
				n++
			}
		}
		jcache.load()
		verb := "添加"
		if !add {
			verb = "删除"
		}
		return ctx.Reply(fmt.Sprintf("已%s %d 个%s关键词", verb, n, label))
	}
}

func RegisterJoinReview(b *bot.Bot) {
	jcache.load()

	b.OnRequest("group").Handle(func(ctx *bot.RequestContext) error {
		e := ctx.Event
		if e.SubType != "add" {
			return nil
		}
		rule := jcache.get(e.GroupID)
		if rule == nil {
			return nil
		}
		switch decideJoin(strings.ToLower(e.Comment), rule.allow, rule.deny) {
		case decisionReject:
			return ctx.BotAPI.SetGroupAddRequest(e.Flag, e.SubType, false, joinDenyReason)
		case decisionApprove:
			return ctx.BotAPI.SetGroupAddRequest(e.Flag, e.SubType, true, "")
		}
		return nil
	})

	b.OnCommand("加群审核").Where(perm.Admin).Handle(func(ctx *bot.CommandContext) error {
		return ctx.Reply(formatJoinList(ctx.GroupID()))
	})
	b.OnCommand("加群白名单").Where(perm.Admin).Handle(joinListHandler(db.JoinActionAllow, "白名单"))
	b.OnCommand("加群黑名单").Where(perm.Admin).Handle(joinListHandler(db.JoinActionDeny, "黑名单"))
}
