package group

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/bot/perm"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
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

func formatJoinList(groupID int64) (string, error) {
	state, err := db.GetJoinReview(groupID)
	if err != nil {
		return "", err
	}
	allow, deny := state.Effective.Allow, state.Effective.Deny
	mode := map[string]string{db.JoinModeInherit: "继承全局", db.JoinModeOverride: "独立配置", db.JoinModeDisabled: "全部留人工"}[state.Config.Mode]
	show := func(s []string) string {
		if len(s) == 0 {
			return "（空）"
		}
		return strings.Join(s, "、")
	}
	return fmt.Sprintf("加群审核（本群 · %s）\n白名单（通过词）：%s\n黑名单（拒绝词）：%s\n用法：加群白名单 词1,词2（覆盖，留空清空）；加群黑名单 词1,词2；白名单填 * 表示任意非空理由放行",
		mode, show(allow), show(deny)), nil
}

func joinListHandler(action, label string) func(*bot.CommandContext) error {
	return func(ctx *bot.CommandContext) error {
		keywords := parseKeywords(strings.Join(ctx.Args, " "))
		if err := db.SetGroupJoinRules(ctx.GroupID(), action, keywords); err != nil {
			return ctx.Reply("操作失败：" + err.Error())
		}
		if len(keywords) == 0 {
			return ctx.Reply("已切换为本群独立配置，已清空" + label)
		}
		return ctx.Reply(fmt.Sprintf("已切换为本群独立配置，已设置%s为：%s", label, strings.Join(keywords, "、")))
	}
}

func RegisterJoinReview(b *bot.Bot) {
	b.OnRequest("group").Plugin(catalog.PluginJoinReview).Handle(func(ctx *bot.RequestContext) error {
		return reviewJoinRequest(ctx.BotAPI, ctx.Event)
	})

	b.OnCommand("加群审核").Plugin(catalog.PluginJoinReview).Where(perm.Admin).Handle(func(ctx *bot.CommandContext) error {
		text, err := formatJoinList(ctx.GroupID())
		if err != nil {
			return err
		}
		return ctx.Reply(text)
	})
	b.OnCommand("加群白名单").Plugin(catalog.PluginJoinReview).Where(perm.Admin).Handle(joinListHandler(db.JoinActionAllow, "白名单"))
	b.OnCommand("加群黑名单").Plugin(catalog.PluginJoinReview).Where(perm.Admin).Handle(joinListHandler(db.JoinActionDeny, "黑名单"))
}

type joinRequestAPI interface {
	SetGroupAddRequest(flag, subType string, approve bool, reason string) error
}

func reviewJoinRequest(api joinRequestAPI, e *bot.RequestEvent) error {
	if e.SubType != "add" || e.GroupID <= 0 {
		return nil
	}
	state, err := db.GetJoinReview(e.GroupID)
	if err != nil {
		return err
	}
	rule := state.Effective
	switch decideJoin(strings.ToLower(e.Comment), rule.Allow, rule.Deny) {
	case decisionReject:
		return api.SetGroupAddRequest(e.Flag, e.SubType, false, joinDenyReason)
	case decisionApprove:
		return api.SetGroupAddRequest(e.Flag, e.SubType, true, "")
	}
	return nil
}
