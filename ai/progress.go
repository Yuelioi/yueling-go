package ai

import (
	"context"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
)

// startProgress keeps acknowledgement independent of model and attachment work.
// Joining on exit prevents a reset from leaving a delayed reaction behind.
func startProgress(ctx context.Context, api *bot.BotAPI, event *bot.GroupMessageEvent, accepted time.Time) func() {
	if api == nil || event.GroupID <= 0 || event.MessageID == 0 {
		return func() {}
	}
	progressCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := api.WithContext(progressCtx).SetMsgEmojiLike(event.MessageID, bot.EmojiProcessing, true)
		traceStage(progressCtx, "progress_reaction", accepted, err)
	}()
	return func() { cancel(); <-done }
}
