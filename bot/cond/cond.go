package cond

import (
	"slices"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
)

func isSuperUser(msg *bot.MsgCtx) bool {
	return slices.Contains(config.C.Bot.SuperUsers, msg.UserID())
}

var Admin bot.Condition = bot.CondFn(func(_ *bot.BotAPI, msg *bot.MsgCtx) bool {
	r := msg.Role()
	return r == "admin" || r == "owner" || isSuperUser(msg)
})

var Owner bot.Condition = bot.CondFn(func(_ *bot.BotAPI, msg *bot.MsgCtx) bool {
	return msg.Role() == "owner" || isSuperUser(msg)
})

func SuperUser(ids ...int64) bot.Condition {
	return bot.CondFn(func(_ *bot.BotAPI, msg *bot.MsgCtx) bool {
		for _, id := range ids {
			if msg.UserID() == id {
				return true
			}
		}
		return false
	})
}

var NoReply bot.Condition = bot.CondFn(func(_ *bot.BotAPI, msg *bot.MsgCtx) bool {
	_, ok := msg.Message().ReplyID()
	return !ok
})

var NoAt bot.Condition = bot.CondFn(func(_ *bot.BotAPI, msg *bot.MsgCtx) bool {
	return !msg.Message().HasType("at")
})

var NoCommand bot.Condition = bot.CondFn(func(_ *bot.BotAPI, msg *bot.MsgCtx) bool {
	if bot.CmdPrefix == "" {
		return true
	}
	return !strings.HasPrefix(strings.TrimSpace(msg.Text()), bot.CmdPrefix)
})
