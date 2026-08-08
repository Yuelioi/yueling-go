package funny

import (
	"errors"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
)

type memeOutputContextSpy struct {
	groupID int64
	replies []string
	sent    []bot.Message
}

func (c *memeOutputContextSpy) GroupID() int64 { return c.groupID }

func (c *memeOutputContextSpy) Reply(text string) error {
	c.replies = append(c.replies, text)
	return nil
}

func (c *memeOutputContextSpy) SendGroupMsg(groupID int64, msg bot.Message) (int32, error) {
	if groupID != c.groupID {
		return 0, errors.New("unexpected group")
	}
	c.sent = append(c.sent, msg)
	return 1, nil
}

func TestGenerateFailureIsSilent(t *testing.T) {
	ctx := &memeOutputContextSpy{groupID: 100}
	wantErr := errors.New("Text number mismatch: expected between 0 and 0, got 1")
	generate := func(string, [][]byte, []string, map[string]any) ([]byte, string, error) {
		return nil, "", wantErr
	}

	err := generateAndSendMeme(ctx, generate, "test", nil, []string{"unexpected"}, "test", func(data []byte) bot.Message {
		return bot.Msg().ImageBytes(data).Build()
	})
	if err != nil {
		t.Fatalf("generateAndSendMeme() error = %v", err)
	}
	if len(ctx.replies) != 0 {
		t.Fatalf("generateAndSendMeme() sent replies = %q, want none", ctx.replies)
	}
	if len(ctx.sent) != 0 {
		t.Fatalf("generateAndSendMeme() sent %d messages after failure, want none", len(ctx.sent))
	}
}

func TestGenerateSuccessSendsOneMessage(t *testing.T) {
	ctx := &memeOutputContextSpy{groupID: 100}
	generate := func(string, [][]byte, []string, map[string]any) ([]byte, string, error) {
		return []byte("image"), "image/png", nil
	}

	err := generateAndSendMeme(ctx, generate, "test", nil, nil, "test", func([]byte) bot.Message {
		return bot.Msg().Text("generated").Build()
	})
	if err != nil {
		t.Fatalf("generateAndSendMeme() error = %v", err)
	}
	if len(ctx.replies) != 0 {
		t.Fatalf("generateAndSendMeme() sent replies = %q, want none", ctx.replies)
	}
	if len(ctx.sent) != 1 || ctx.sent[0].Text() != "generated" {
		t.Fatalf("generateAndSendMeme() sent = %#v, want one generated message", ctx.sent)
	}
}

func TestResolveMemeTextsIgnoresArgumentsWhenTemplateAcceptsNoText(t *testing.T) {
	texts, ok := resolveMemeTexts([]string{"unexpected"}, 0, 0, nil)
	if !ok {
		t.Fatal("resolveMemeTexts() rejected a no-text template")
	}
	if len(texts) != 0 {
		t.Fatalf("resolveMemeTexts() = %q, want no texts", texts)
	}
}

func TestResolveMemeTextsJoinsSingleTextArgument(t *testing.T) {
	texts, ok := resolveMemeTexts([]string{"hello", "world"}, 1, 1, nil)
	if !ok {
		t.Fatal("resolveMemeTexts() rejected valid text")
	}
	if len(texts) != 1 || texts[0] != "hello world" {
		t.Fatalf("resolveMemeTexts() = %q, want [\"hello world\"]", texts)
	}
}
