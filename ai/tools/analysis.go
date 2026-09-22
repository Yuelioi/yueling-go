package tools

import (
	"errors"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services/chatsummary"
)

func init() { registerSummarizeChat() }

func registerSummarizeChat() {
	ai.Register(ai.ToolMeta{
		Name:        "summarize_chat",
		ReadOnly:    true,
		Description: "读取群聊资料供你总结、提取决策、待办或未解决问题。根据返回记录直接完成用户请求，说明取样范围，不再调用其他总结工具；不自动创建待办或提醒。",
		Tags:        []string{"上下文", "群聊"},
		Triggers:    []string{"总结群聊", "群聊总结", "群聊摘要", "聊天总结", "讨论结论", "群聊待办", "未解决问题", "决策整理"},
		Patterns:    []string{`(聊了|在聊)(什么|啥)`, `(总结|回顾|整理|提取).{0,12}(群聊|聊天|讨论|决策|待办|行动项)`, `(今天|昨天|本周).{0,8}(结论|要点)`},
		Slots:       []string{"聊天总结", "话题总结", "讨论要点", "行动项"},
		PluginID:    catalog.PluginDailyDigest,
		Params: []ai.Param{
			{Name: "count", Type: "integer", Description: "记录条数（10-100，默认见配置）"},
			{Name: "period", Type: "string", Description: "recent 为最近消息，其他范围查询本地记录", Enum: []string{"recent", "today", "yesterday", "week", "7days"}},
			{Name: "mode", Type: "string", Description: "summary 总结；decisions 决策；actions 待办；questions 未解决问题", Enum: []string{"summary", "decisions", "actions", "questions"}},
			{Name: "focus", Type: "string", Description: "需要关注的主题，可选"},
		},
		Handler: summarizeChat,
	})
}

func summarizeChat(ctx *ai.ToolContext) (string, error) {
	query, err := chatsummary.Normalize(chatsummary.Query{
		Count: int(ctx.Int("count")), Period: ctx.String("period"),
		Mode: ctx.String("mode"), Focus: ctx.String("focus"),
	}, config.C.AI.Context.Summary)
	if err != nil {
		return chatsummary.UserMessage(err), nil
	}
	material, err := chatsummary.Read(ctx.Context(),
		chatsummary.NewGroupReader(ctx.BotAPI(), ctx.GroupID(), ctx.MessageID()), query)
	if err != nil {
		if errors.Is(err, chatsummary.ErrNoRecords) {
			return chatsummary.UserMessage(err), nil
		}
		return "", err
	}
	return material.JSON(), nil
}
