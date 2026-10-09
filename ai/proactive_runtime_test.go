package ai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/internal/testdb"
)

func TestDirectReplyCancelsPendingProactiveInSameGroup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	other, stop := context.WithCancel(context.Background())
	defer stop()
	p := &ProactiveManager{heats: map[int64]*groupHeat{
		1: {pending: &proactiveRun{ctx: ctx, cancel: cancel}, score: 99, streak: 10},
		2: {pending: &proactiveRun{ctx: other, cancel: stop}, score: 99, streak: 10},
	}}
	p.OnBotReplied(1)
	if ctx.Err() == nil || other.Err() != nil {
		t.Fatal("cancellation crossed group boundary or missed current group")
	}
	if h := p.heats[1]; h.score != 0 || h.streak != 0 || time.Since(h.lastSpeak) > time.Second {
		t.Fatal("reply did not reset cooldown")
	}
}

func TestDirectTriggerCancelsProactiveBeforeAffinityOrModel(t *testing.T) {
	old := config.Current()
	defer config.Publish(old)
	cfg := old
	cfg.Bot.Name = "月灵"
	config.Publish(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &ProactiveManager{heats: map[int64]*groupHeat{1: {pending: &proactiveRun{ctx: ctx, cancel: cancel}}}}
	p.Feed(nil, &bot.GroupMessageEvent{GroupID: 1, SelfID: 999, Message: bot.Msg().Text("月灵 帮我一下").Build()})
	if ctx.Err() == nil {
		t.Fatal("direct conversation left proactive generation running")
	}
	if err := p.fire(ctx, nil, 1, "讨论内容", ""); err == nil {
		t.Fatal("canceled generation succeeded")
	}
}

func TestPendingProactiveGenerationHonorsCancellation(t *testing.T) {
	oldConfig, oldClient, oldDB := config.Current(), testEngine.modelBackend, testdb.Database
	t.Cleanup(func() { config.Publish(oldConfig); testEngine.modelBackend = oldClient; testdb.Database = oldDB })
	testEngine.modelBackend = nil
	testdb.Database = nil
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	cfg := oldConfig
	cfg.AI = config.AIConfig{DeepSeekKey: "fixture", BaseURL: server.URL, Model: "fixture"}
	config.Publish(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &ProactiveManager{heats: map[int64]*groupHeat{1: {pending: &proactiveRun{ctx: ctx, cancel: cancel}}}}
	done := make(chan error, 1)
	go func() { done <- p.fire(ctx, nil, 1, "讨论内容", "") }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("model request did not start")
	}
	p.OnBotReplied(1)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled generation succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("proactive generation ignored cancellation")
	}
}
