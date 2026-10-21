package ai

import (
	"context"
	"github.com/Yuelioi/yueling-go/internal/testconfig"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

type isolatedBackend string

func (b isolatedBackend) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: string(b) + ":" + req.Model}}}}, nil
}

func TestEngineModelBackendsAreIndependent(t *testing.T) {
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			engine := NewEngine(EngineDependencies{ModelBackend: isolatedBackend(name)})
			result, err := engine.modelClientFor(testconfig.AI(name, "", "")).Complete(context.Background(), openai.ChatCompletionRequest{})
			if err != nil || len(result.Choices) != 1 || result.Choices[0].Message.Content != name+":"+name {
				t.Fatalf("transport isolation failed: %v", err)
			}
		})
	}
}

func TestEnginesIsolateSessionsConfirmationsAndToolRegistration(t *testing.T) {
	first, second := NewEngine(EngineDependencies{}), NewEngine(EngineDependencies{})
	first.Registry.Register(ToolMeta{Name: "private"})
	first.Registry.Register(ToolMeta{Name: "private", Description: "replacement"})
	if len(first.Registry.AllTools()) != 1 {
		t.Fatal("replacement duplicated route entries")
	}
	if _, ok := second.Registry.GetTool("private"); ok {
		t.Fatal("registry leaked across engines")
	}
	session := first.Sessions.Get(1, 2)
	id, code := first.Confirms.StoreWithContext(2, 1, "private", nil, nil, nil, session)
	if _, ok := second.Confirms.Verify(id, code, 2, 1); ok {
		t.Fatal("confirmation leaked across engines")
	}
	secondSession := second.Sessions.Get(1, 2)
	first.Sessions.Delete(1, 2)
	if !session.invalidated() || secondSession.invalidated() {
		t.Fatal("reset crossed engine ownership")
	}
	if _, ok := first.Confirms.Verify(id, code, 2, 1); ok {
		t.Fatal("reset confirmation remained executable")
	}
	if first.Proactive == second.Proactive {
		t.Fatal("proactive state is shared")
	}
}
