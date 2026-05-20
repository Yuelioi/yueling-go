package random

import (
	"fmt"
	"math/rand"
	"sort"

	"github.com/Yuelioi/yueling-go/bot"
)

func RegisterMember(b *bot.Bot) {
	b.OnRegex(`抽(.*)群友(.*)|随机.*群友.*|来个.*群友.*|来点.*群友.*`).Handle(func(ctx *bot.GroupContext) error {
		members, err := ctx.GetGroupMemberList(ctx.GroupID())
		if err != nil {
			return ctx.Reply("获取群员列表失败")
		}
		if len(members) == 0 {
			return ctx.Reply("群里没有成员喵~")
		}

		sort.Slice(members, func(i, j int) bool {
			return members[i].LastSentTime < members[j].LastSentTime
		})

		limit := 25
		if len(members) < limit {
			limit = len(members)
		}
		recent := members[len(members)-limit:]
		picked := recent[rand.Intn(len(recent))]

		_, err = ctx.SendGroupMsg(ctx.GroupID(),
			bot.Msg().
				At(ctx.UserID()).
				Text(fmt.Sprintf("你抽到的是: %s\n", picked.Nickname)).
				Image(fmt.Sprintf("https://q.qlogo.cn/headimg_dl?dst_uin=%d&spec=640", picked.UserID)).
				Build(),
		)
		return err
	})
}
