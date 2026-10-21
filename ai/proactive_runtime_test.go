package ai

import (
	"context"
	"encoding/json"
	"github.com/Yuelioi/yueling-go/internal/testconfig"
	openai "github.com/sashabaranov/go-openai"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	cfg.AI = testconfig.AI("fixture", server.URL, "fixture")
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

func TestNewGroupMessageCancelsStaleProactive(t *testing.T) {
	old := config.Current()
	defer config.Publish(old)
	cfg := old
	cfg.AI.Affinity.Enabled = false
	config.Publish(cfg)
	for _, message := range []bot.Message{bot.Msg().Text("换个话题，晚上吃火锅").Build(), bot.Msg().Image("https://example.invalid/picture.png").Build()} {
		ctx, cancel := context.WithCancel(context.Background())
		other, stop := context.WithCancel(context.Background())
		p := &ProactiveManager{heats: map[int64]*groupHeat{1: {pending: &proactiveRun{ctx: ctx, cancel: cancel}}, 2: {pending: &proactiveRun{ctx: other, cancel: stop}}}}
		p.Feed(nil, &bot.GroupMessageEvent{GroupID: 1, UserID: 10, SelfID: 999, Message: message})
		if ctx.Err() == nil {
			t.Error("new message left obsolete proactive generation running")
		}
		if message.Text() == "" && (len(p.heats[1].messages) != 1 || p.heats[1].messages[0].Text != "[非文字消息，内容未读取]") {
			t.Error("non-text message disappeared from recent context")
		}
		if other.Err() != nil {
			t.Error("canceled a different group's generation")
		}
		cancel()
		stop()
	}
}

func TestProactiveFeedSendsLatestAnchoredContextAndAllowsSilence(t *testing.T) {
	old := config.Current()
	defer config.Publish(old)
	requests := make(chan openai.ChatCompletionRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		requests <- req
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"NO_REPLY"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()
	cfg := old
	cfg.AI = testconfig.AI("fixture", server.URL, "fixture")
	config.Publish(cfg)
	h := &groupHeat{score: 99, streak: 3, dailyDate: bot.Today()}
	for i := 0; i < 20; i++ {
		h.appendMessage(&bot.GroupMessageEvent{Sender: bot.Sender{Nickname: "旧话题群友"}}, "前面一直在聊显卡", time.Now())
	}
	p := &ProactiveManager{heats: map[int64]*groupHeat{1: h}}
	p.Feed(nil, &bot.GroupMessageEvent{GroupID: 1, UserID: 10, SelfID: 999, Sender: bot.Sender{Card: "阿岚"}, Message: bot.Msg().Text("先不聊显卡了，晚上吃火锅").Build()})
	select {
	case req := <-requests:
		var data struct {
			Latest     proactiveMessage   `json:"latest_message"`
			Recent     []proactiveMessage `json:"recent_context"`
			Background []proactiveMessage `json:"background_context"`
		}
		if err := json.Unmarshal([]byte(req.Messages[1].Content), &data); err != nil {
			t.Fatal(err)
		}
		if data.Latest.Speaker != "阿岚" || data.Latest.Text != "先不聊显卡了，晚上吃火锅" || len(data.Recent) != 2 || len(data.Background) != 5 {
			t.Fatalf("wrong latest context: %+v", data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("proactive request not started")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		done := h.pending == nil
		count := h.dailyCount
		last := h.lastSpeak
		p.mu.Unlock()
		if done {
			if count != 0 || !last.IsZero() {
				t.Fatal("silence consumed delivered-message quota")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("silent generation did not finish")
}

func TestProactiveContextExpiresOldTopicsAndBoundsMessages(t *testing.T) {
	now := time.Now()
	h := &groupHeat{score: 99, streak: 10}
	h.appendMessage(&bot.GroupMessageEvent{}, "昨天的话题", now.Add(-6*time.Minute))
	h.expireMessages(now)
	if len(h.messages) != 0 || h.score != 0 || h.streak != 0 {
		t.Fatal("old context and heat survived idle window")
	}
	h.appendMessage(&bot.GroupMessageEvent{}, strings.Repeat("长", 1000), now)
	if len([]rune(h.messages[0].Text)) > 401 {
		t.Fatal("single message dominated context budget")
	}
}
