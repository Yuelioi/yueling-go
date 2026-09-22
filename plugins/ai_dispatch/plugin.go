// Package ai_dispatch wires the AI dispatcher into the bot's event pipeline.
// Import this package with a blank import to activate it.
package ai_dispatch

import (
	"context"
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
)

// Register wires the AI dispatch handler onto the given Bot.
// Call this from main() after all AI tools have been registered.
func Register(b *bot.Bot) {
	b.OnGroupMessage(aiTrigger{}).
		Plugin(catalog.PluginAIAssistant).
		Priority(1). // lower than specific plugins so they take precedence
		Handle(func(ctx *bot.GroupContext) error {
			return ai.Dispatch(context.Background(), ctx, func(turn context.Context, reply string) error {
				if err := ctx.BotAPI.WithContext(turn).ReplyGroup(ctx.Event, reply); err != nil {
					return err
				}
				ai.Proactive.OnBotReplied(ctx.GroupID())
				return nil
			})
		})
}

// aiTrigger fires when the message @-mentions the bot or starts with its name.
type aiTrigger struct{}

func (aiTrigger) Match(ctx *bot.MsgCtx) bot.MatchResult {
	selfIDStr := fmt.Sprintf("%d", ctx.SelfID())

	for _, target := range ctx.Message().AtTargets() {
		if target == selfIDStr {
			return bot.MatchResult{Matched: true}
		}
	}

	name := config.C.Bot.Name
	if name != "" && strings.HasPrefix(strings.TrimSpace(ctx.Text()), name) {
		return bot.MatchResult{Matched: true}
	}

	return bot.MatchResult{}
}
