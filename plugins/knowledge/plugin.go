package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/bot/perm"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	knowledgeservice "github.com/Yuelioi/yueling-go/services/knowledge"
	"gorm.io/gorm"
)

func Register(b *bot.Bot) {
	b.OnGroupMessage(shortcutMatcher{}).
		Plugin(catalog.PluginGroupKnowledge).
		Priority(20).
		Block().
		Handle(handleShortcut)
	b.OnCommand("知识添加").Plugin(catalog.PluginGroupKnowledge).Where(perm.Admin).Handle(handleAdd)
	b.OnCommand("知识导入").Plugin(catalog.PluginGroupKnowledge).Where(perm.Admin).Handle(handleImport)
	b.OnCommand("知识列表").Plugin(catalog.PluginGroupKnowledge).Handle(handleList)
	b.OnCommand("知识删除").Plugin(catalog.PluginGroupKnowledge).Where(perm.Admin).Handle(handleDelete)
	b.OnCommand("知识问").Plugin(catalog.PluginGroupKnowledge).Handle(handleAsk)
}

type shortcutMatcher struct{}

func (shortcutMatcher) Match(ctx *bot.MsgCtx) bot.MatchResult {
	row, err := knowledgeservice.FindShortcut(ctx.GroupID(), ctx.Text())
	return bot.MatchResult{Matched: err == nil && row != nil}
}

func handleShortcut(ctx *bot.GroupContext) error {
	row, err := knowledgeservice.FindShortcut(ctx.GroupID(), ctx.Text())
	if err != nil || row == nil {
		return nil
	}
	return ctx.Reply(knowledgeservice.ShortcutResponse(*row, ctx.Text()))
}

func handleAdd(ctx *bot.CommandContext) error {
	raw := strings.TrimSpace(strings.Join(ctx.Args, " "))
	title, content := splitKnowledgeInput(raw)
	if replied, ok := ctx.RepliedMessage(); ok && !strings.Contains(raw, "|") {
		title = raw
		content = strings.TrimSpace(replied.Text())
	}
	if content == "" {
		return ctx.Reply("用法：知识添加 [标题] | <内容>\n也可以回复一条文字消息发送：知识添加 [标题]")
	}
	row, err := knowledgeservice.AddText(ctx.GroupID(), ctx.UserID(), title, content)
	if err != nil {
		return ctx.Reply("添加知识失败：" + err.Error())
	}
	return ctx.Reply(fmt.Sprintf("知识已添加（ID: %d）：%s", row.ID, row.Title))
}

func handleImport(ctx *bot.CommandContext) error {
	if len(ctx.Args) == 0 {
		return ctx.Reply("用法：知识导入 <公网网页URL> [标题]")
	}
	ctx.React(bot.EmojiProcessing)
	title := ""
	if len(ctx.Args) > 1 {
		title = strings.Join(ctx.Args[1:], " ")
	}
	row, err := knowledgeservice.AddURL(ctx.GroupID(), ctx.UserID(), title, ctx.Args[0])
	if err != nil {
		return ctx.Reply("导入知识失败：" + err.Error())
	}
	return ctx.Reply(fmt.Sprintf("网页已导入知识库（ID: %d）：%s", row.ID, row.Title))
}

func handleList(ctx *bot.CommandContext) error {
	rows, err := knowledgeservice.ListAvailable(ctx.GroupID())
	if err != nil {
		return ctx.Reply("读取知识库失败。")
	}
	if len(rows) == 0 {
		return ctx.Reply("本群和共享知识库都还是空的。")
	}
	return ctx.Reply(formatKnowledgeList(rows))
}

func handleDelete(ctx *bot.CommandContext) error {
	if len(ctx.Args) == 0 {
		return ctx.Reply("用法：知识删除 <ID>")
	}
	id, err := strconv.ParseUint(ctx.Args[0], 10, 64)
	if err != nil || id == 0 {
		return ctx.Reply("知识 ID 格式错误。")
	}
	if err := knowledgeservice.Remove(uint(id), ctx.GroupID()); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Reply("没有找到本群的这条知识。")
		}
		return ctx.Reply("删除知识失败。")
	}
	return ctx.Reply(fmt.Sprintf("知识 %d 已删除。", id))
}

func handleAsk(ctx *bot.CommandContext) error {
	question := strings.TrimSpace(strings.Join(ctx.Args, " "))
	if question == "" {
		return ctx.Reply("用法：知识问 <问题>")
	}
	rows, err := knowledgeservice.Search(ctx.GroupID(), question, 5)
	if err != nil {
		return ctx.Reply("检索知识库失败。")
	}
	if len(rows) == 0 {
		return ctx.Reply("知识库里暂时没有找到相关资料。")
	}
	if ok, hint := ai.AllowAICall(ctx.UserID(), ctx.GroupID()); !ok {
		return ctx.Reply(hint)
	}
	ctx.React(bot.EmojiProcessing)
	answerCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	answer, err := ai.AnswerGroupKnowledge(answerCtx, question, rows)
	if err != nil {
		return ctx.Reply("知识库回答失败，请稍后重试。")
	}
	return ctx.Reply(answer)
}

func splitKnowledgeInput(raw string) (string, string) {
	parts := strings.SplitN(raw, "|", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return "", strings.TrimSpace(raw)
}

func formatKnowledgeList(rows []db.GroupKnowledge) string {
	lines := make([]string, 0, min(len(rows), 20))
	for _, row := range rows[:min(len(rows), 20)] {
		source := "文本"
		if row.SourceURL != "" {
			source = "网页"
		}
		shortcutValues := make([]string, 0, len(row.Shortcuts))
		for _, shortcut := range row.Shortcuts {
			shortcutValues = append(shortcutValues, shortcut.Trigger)
		}
		shortcutText := ""
		if len(shortcutValues) > 0 {
			shortcutText = " · 快捷词 " + strings.Join(shortcutValues, "、")
		}
		scope := "本群"
		if row.GroupID == db.SharedKnowledgeGroupID {
			scope = "共享"
		}
		lines = append(lines, fmt.Sprintf("ID %d · %s · %s · %s%s", row.ID, scope, row.Title, source, shortcutText))
	}
	suffix := ""
	if len(rows) > 20 {
		suffix = fmt.Sprintf("\n…另有 %d 条，请在 WebUI 查看", len(rows)-20)
	}
	return fmt.Sprintf("本群可用知识（%d 条，含共享）：\n%s%s", len(rows), strings.Join(lines, "\n"), suffix)
}
