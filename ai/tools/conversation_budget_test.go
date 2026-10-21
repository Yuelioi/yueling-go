package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestCasualDispatchUsesCompactCatalogWithoutExtraModelCall(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	cfg.AI = config.AIConfig{Model: "fixture", DeepSeekKey: "synthetic"}
	config.Publish(cfg)
	var request openai.ChatCompletionRequest
	calls := 0
	engine := ai.NewEngine(ai.EngineDependencies{ModelBackend: imageRequestModel(func(req openai.ChatCompletionRequest) openai.ChatCompletionResponse {
		request = req
		calls++
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: "stop", Message: openai.ChatCompletionMessage{Role: "assistant", Content: "今天聊点开心的。"}}}}
	})})
	RegisterAll(engine.Registry, Dependencies{})
	event := &bot.GroupMessageEvent{GroupID: 987001, UserID: 987001, MessageID: 1, Message: bot.Msg().Text("今天心情不错").Build()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := engine.Dispatch(ctx, &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { cancel(); return nil }); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(request.Tools) != 2 {
		t.Fatalf("casual dispatch added model calls or lost semantic tools: calls=%d tools=%d", calls, len(request.Tools))
	}
	var oldEntries, newEntries strings.Builder
	for _, tool := range engine.Registry.AllTools() {
		data, _ := json.Marshal(map[string]string{"name": tool.Name, "description": tool.Description})
		oldEntries.Write(data)
		oldEntries.WriteByte('\n')
		found := false
		for _, line := range strings.Split(request.Messages[0].Content, "\n") {
			if strings.HasPrefix(line, tool.Name+"：") {
				newEntries.WriteString(line + "\n")
				found = true
				break
			}
		}
		if !found {
			t.Errorf("semantic capability missing from compact catalog: %s", tool.Name)
		}
	}
	before, after := utf8.RuneCountInString(oldEntries.String()), utf8.RuneCountInString(newEntries.String())
	if after >= before*3/5 {
		t.Fatalf("catalog did not shrink sufficiently: before=%d after=%d", before, after)
	}
	t.Logf("registered_tools=%d old_catalog_chars=%d compact_catalog_chars=%d (not tokens)", len(engine.Registry.AllTools()), before, after)
}
