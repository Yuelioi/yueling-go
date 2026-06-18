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

// parseKeywords splits a comma-separated (半/全角) argument into lower-cased,
// de-spaced keywords. Empty input yields an empty slice — the caller treats that
// as "clear the list".
func parseKeywords(raw string) []string {
	raw = strings.ReplaceAll(raw, "，", ",")
	var keywords []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			keywords = append(keywords, p)
		}
	}
	return keywords
}

const joinDenyReason = "申请未通过机器人审核"

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
	return fmt.Sprintf("加群审核（本群）\n白名单（通过词）：%s\n黑名单（拒绝词）：%s\n用法：加群白名单 词1,词2（覆盖，留空清空）；加群黑名单 词1,词2；白名单填 * 表示任意理由放行",
		show(allow), show(deny))
}

func joinListHandler(action, label string) func(*bot.CommandContext) error {
	return func(ctx *bot.CommandContext) error {
		keywords := parseKeywords(strings.Join(ctx.Args, " "))
		if err := db.SetGroupJoinRules(ctx.GroupID(), action, keywords); err != nil {
			return ctx.Reply("操作失败：" + err.Error())
		}
		jcache.load()
		if len(keywords) == 0 {
			return ctx.Reply("已清空" + label)
		}
		return ctx.Reply(fmt.Sprintf("已设置%s为：%s", label, strings.Join(keywords, "、")))
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
