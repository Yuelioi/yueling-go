package random

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
)

var rollReplies = []string{
	"emmm，要不试试「%s」",
	"来试试「%s」吧",
	"月灵觉得「%s」不错哟",
	"就决定是你了！「%s」！",
	"月灵掷出了「%s」",
}

func RegisterRoll(b *bot.Bot) {
	b.OnCommand("roll").Handle(func(ctx *bot.CommandContext) error {
		args := ctx.Args

		// /roll → 1-100
		if len(args) == 0 {
			return ctx.Reply(fmt.Sprintf("roll: %d", rand.Intn(100)+1))
		}

		// /roll N → 1-N
		if len(args) == 1 {
			n, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil || n < 2 {
				// 单个非数字 → 直接回显（只有一个选项没意义）
				return ctx.Reply(fmt.Sprintf("roll: %s", args[0]))
			}
			return ctx.Reply(fmt.Sprintf("roll: %d", rand.Int63n(n)+1))
		}

		// /roll N M → N-M（两个数字）
		n, err1 := strconv.ParseInt(args[0], 10, 64)
		m, err2 := strconv.ParseInt(args[1], 10, 64)
		if err1 == nil && err2 == nil && len(args) == 2 {
			if n > m {
				n, m = m, n
			}
			return ctx.Reply(fmt.Sprintf("roll: %d", n+rand.Int63n(m-n+1)))
		}

		// /roll A B C … → 从选项中随机选一个
		pick := args[rand.Intn(len(args))]
		tmpl := rollReplies[rand.Intn(len(rollReplies))]
		_ = strings.Join // keep import
		return ctx.Reply(fmt.Sprintf(tmpl, pick))
	})
}
