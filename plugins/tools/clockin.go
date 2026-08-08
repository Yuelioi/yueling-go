package tools

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
)

var trailingNum = regexp.MustCompile(`^(.+?)(\d+)$`)

var clockinEncouragements = [][]string{
	{
		"第 %d 次到手，开张啦～",
		"这个月的第 %d 枚打卡，稳稳收下！",
		"第 %d 次起步成功，明天顺手再来一下～",
	},
	{
		"都第 %d 次了，节奏开始有了～",
		"第 %d 次拿下，慢慢攒，月底回头看肯定很爽",
		"今天也没漏，第 %d 次稳稳的",
		"第 %d 次！再这样下去打卡区都认识你了",
	},
	{
		"嚯，都第 %d 次了，有点稳啊",
		"第 %d 次拿下，一周的量已经有了～",
		"第 %d 次了，你这不是路过，是常驻吧",
		"坚持到第 %d 次，月底的你会来夸现在的你",
	},
	{
		"第 %d 次了，这出勤率有点狠",
		"都第 %d 次了，打卡区该给你留专座",
		"第 %d 次拿下，你是真能坚持，服了服了",
		"稳到第 %d 次，继续冲就完事了",
	},
	{
		"第 %d 次！这个月的打卡区快被你承包了",
		"都第 %d 次了，全勤味儿已经出来了",
		"坚持到第 %d 次，必须给你鼓个掌 👏",
		"第 %d 次拿下，月底王者非你莫属～",
	},
}

func clockinEncouragement(monthly int) string {
	return pickClockinEncouragement(monthly, rand.Intn)
}

func pickClockinEncouragement(monthly int, pick func(int) int) string {
	tier := 0
	switch {
	case monthly >= 25:
		tier = 4
	case monthly >= 15:
		tier = 3
	case monthly >= 7:
		tier = 2
	case monthly >= 2:
		tier = 1
	}

	replies := clockinEncouragements[tier]
	return fmt.Sprintf(replies[pick(len(replies))], monthly)
}

func RegisterClockin(b *bot.Bot) {
	b.OnCommand("打卡").Plugin(catalog.PluginClockIn).Handle(func(ctx *bot.CommandContext) error {
		_, _, monthly, already, err := db.CheckIn(ctx.UserID(), ctx.GroupID(), ctx.Nickname())
		if err != nil {
			return ctx.Reply("打卡失败，请稍后再试。")
		}
		if already {
			return ctx.Reply("今天打过卡了，明天再来吧~")
		}
		encouragement := clockinEncouragement(monthly)

		// 尝试把群名片末尾数字 +1
		info, err := ctx.GetGroupMemberInfo(ctx.GroupID(), ctx.UserID())
		if err == nil {
			card := info.Card
			if card == "" {
				card = info.Nickname
			}
			if m := trailingNum.FindStringSubmatch(card); m != nil {
				n, _ := strconv.Atoi(m[2])
				newCard := m[1] + strconv.Itoa(n+1)
				ctx.SetGroupCard(ctx.GroupID(), ctx.UserID(), newCard)
				return ctx.Reply(fmt.Sprintf("打卡成功～%s → %s\n%s", card, newCard, encouragement))
			}
		}
		return ctx.Reply("打卡成功～\n" + encouragement)
	})
}
