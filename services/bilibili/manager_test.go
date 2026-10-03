package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/config"
)

type memoryStore struct {
	data []byte
	fail bool
}

func (s *memoryStore) Load(int64) ([]byte, error) { return slices.Clone(s.data), nil }
func (s *memoryStore) Save(_ int64, data []byte) error {
	if s.fail {
		return fmt.Errorf("disk unavailable")
	}
	s.data = slices.Clone(data)
	return nil
}

type fakeBackend struct {
	sessions []Session
	messages []PrivateMessage
	sent     []string
	failSend bool
}

func (b *fakeBackend) AccountID() int64                                   { return 10 }
func (b *fakeBackend) Sessions(context.Context, int64) ([]Session, error) { return b.sessions, nil }
func (b *fakeBackend) Messages(context.Context, int64, int64) ([]PrivateMessage, error) {
	return b.messages, nil
}
func (b *fakeBackend) SendText(_ context.Context, uid int64, text string) error {
	b.sent = append(b.sent, fmt.Sprintf("%d:%s", uid, text))
	if b.failSend {
		return fmt.Errorf("send timeout")
	}
	return nil
}
func incoming(seq int64) PrivateMessage {
	return PrivateMessage{SenderID: 20, ReceiverID: 10, ReceiverType: 1, Seq: seq, Type: 1, Timestamp: 1001, Content: `{"content":"你好"}`}
}
func managerFixture() *Manager {
	backend := &fakeBackend{sessions: []Session{{TalkerID: 20, Type: 1, LastMessage: &PrivateMessage{Seq: 1}}}}
	m := NewManager(backend, &memoryStore{}, config.BilibiliConfig{ReplyMode: "off", ReplyCooldownSeconds: 60, ReplyPerMinute: 5}, []int64{99, 98}, nil)
	m.now = func() time.Time { return time.Unix(1000, 0) }
	return m
}
func bootstrap(t *testing.T, m *Manager) {
	t.Helper()
	if n, err := m.Poll(context.Background(), func(int64, string) error { t.Fatal("historical notification"); return nil }); err != nil || n != 0 {
		t.Fatalf("baseline n=%d err=%v", n, err)
	}
}

func TestFirstPollAndRestartDoNotReplay(t *testing.T) {
	m := managerFixture()
	bootstrap(t, m)
	b := m.backend.(*fakeBackend)
	b.messages = []PrivateMessage{incoming(2)}
	b.sessions[0].LastMessage = &b.messages[0]
	notifications := 0
	notify := func(int64, string) error { notifications++; return nil }
	if n, err := m.Poll(context.Background(), notify); err != nil || n != 1 {
		t.Fatalf("poll n=%d err=%v", n, err)
	}
	restarted := NewManager(b, m.store, m.cfg, m.notifyUsers, nil)
	restarted.now = m.now
	if n, err := restarted.Poll(context.Background(), notify); err != nil || n != 0 {
		t.Fatalf("restart n=%d err=%v", n, err)
	}
	if notifications != 2 || len(b.sent) != 0 {
		t.Fatalf("notifications=%d auto=%v", notifications, b.sent)
	}
}

func TestFailedRecipientResumesWithoutRepeatingSuccessfulRecipient(t *testing.T) {
	m := managerFixture()
	bootstrap(t, m)
	b := m.backend.(*fakeBackend)
	b.messages = []PrivateMessage{incoming(2)}
	b.sessions[0].LastMessage = &b.messages[0]
	var recipients []int64
	if _, err := m.Poll(context.Background(), func(qq int64, _ string) error {
		recipients = append(recipients, qq)
		if qq == 98 {
			return fmt.Errorf("QQ disconnected")
		}
		return nil
	}); err == nil {
		t.Fatal("failed recipient accepted")
	}
	restarted := NewManager(b, m.store, m.cfg, m.notifyUsers, nil)
	restarted.now = m.now
	if n, err := restarted.Poll(context.Background(), func(qq int64, _ string) error { recipients = append(recipients, qq); return nil }); err != nil || n != 1 {
		t.Fatalf("retry=%d err=%v", n, err)
	}
	if !slices.Equal(recipients, []int64{99, 98, 98}) {
		t.Fatalf("retry recipients=%v", recipients)
	}
}

func TestHybridKeywordPriorityCooldownAndModePersistence(t *testing.T) {
	m := managerFixture()
	bootstrap(t, m)
	if err := m.SetRule(config.BilibiliKeyword{Keyword: "你好", Reply: "你好呀"}); err != nil {
		t.Fatal(err)
	}
	if err := m.SetMode("hybrid"); err != nil {
		t.Fatal(err)
	}
	aiCalls := 0
	m.aiReply = func(context.Context, string) (string, error) { aiCalls++; return "AI reply", nil }
	b := m.backend.(*fakeBackend)
	b.messages = []PrivateMessage{incoming(2), incoming(3)}
	b.sessions[0].LastMessage = &b.messages[1]
	if _, err := m.Poll(context.Background(), func(int64, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if aiCalls != 0 || !slices.Equal(b.sent, []string{"20:你好呀"}) {
		t.Fatalf("AI=%d sent=%v", aiCalls, b.sent)
	}
	m.now = func() time.Time { return time.Unix(1070, 0) }
	msg := incoming(4)
	msg.Content = `{"content":"新问题"}`
	b.messages = []PrivateMessage{msg}
	b.sessions[0].LastMessage = &msg
	if _, err := m.Poll(context.Background(), func(int64, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if aiCalls != 1 || len(b.sent) != 2 {
		t.Fatalf("fallback AI=%d sent=%v", aiCalls, b.sent)
	}
	restarted := NewManager(b, m.store, m.cfg, m.notifyUsers, nil)
	rules, err := restarted.Rules()
	if err != nil || len(rules) != 1 || restarted.state.Mode != "hybrid" {
		t.Fatalf("restored rules=%v mode=%s err=%v", rules, restarted.state.Mode, err)
	}
}

func TestOutgoingWithdrawnAndAutomaticMessagesAreNotRepliedOrForwarded(t *testing.T) {
	m := managerFixture()
	bootstrap(t, m)
	b := m.backend.(*fakeBackend)
	outgoing := incoming(2)
	outgoing.SenderID = 10
	outgoing.ReceiverID = 20
	withdrawn := incoming(3)
	withdrawn.Status = 1
	automatic := incoming(4)
	automatic.Source = 9
	b.messages = []PrivateMessage{outgoing, withdrawn, automatic}
	b.sessions[0].LastMessage = &automatic
	if n, err := m.Poll(context.Background(), func(int64, string) error { t.Fatal("ignored message forwarded"); return nil }); err != nil || n != 0 {
		t.Fatalf("ignored poll n=%d err=%v", n, err)
	}
}

func TestUncertainAutomaticSendDoesNotRetryAfterRestart(t *testing.T) {
	m := managerFixture()
	bootstrap(t, m)
	_ = m.SetMode("keyword")
	_ = m.SetRule(config.BilibiliKeyword{Keyword: "你好", Reply: "hello"})
	b := m.backend.(*fakeBackend)
	b.failSend = true
	b.messages = []PrivateMessage{incoming(2)}
	b.sessions[0].LastMessage = &b.messages[0]
	if _, err := m.Poll(context.Background(), func(int64, string) error { return nil }); err == nil {
		t.Fatal("uncertain send not reported")
	}
	restarted := NewManager(b, m.store, m.cfg, m.notifyUsers, nil)
	restarted.now = m.now
	if _, err := restarted.Poll(context.Background(), func(int64, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(b.sent) != 1 {
		t.Fatalf("duplicate automatic sends=%v", b.sent)
	}
}

func TestCheckpointFailureStopsBeforeSending(t *testing.T) {
	m := managerFixture()
	bootstrap(t, m)
	_ = m.SetMode("keyword")
	_ = m.SetRule(config.BilibiliKeyword{Keyword: "你好", Reply: "hello"})
	b := m.backend.(*fakeBackend)
	b.messages = []PrivateMessage{incoming(2)}
	b.sessions[0].LastMessage = &b.messages[0]
	m.store.(*memoryStore).fail = true
	if _, err := m.Poll(context.Background(), func(int64, string) error { return nil }); err == nil {
		t.Fatal("checkpoint failure accepted")
	}
	if len(b.sent) != 0 {
		t.Fatal("sent despite checkpoint failure")
	}
}

func TestAutomaticReplyGlobalQuotaAndNonTextFilter(t *testing.T) {
	m := managerFixture()
	m.cfg.ReplyPerMinute = 1
	m.cfg.ReplyCooldownSeconds = 0
	bootstrap(t, m)
	_ = m.SetMode("keyword")
	_ = m.SetRule(config.BilibiliKeyword{Keyword: "你好", Reply: "hello"})
	b := m.backend.(*fakeBackend)
	image := incoming(2)
	image.Type = 2
	image.Content = `{"url":"https://i0.hdslb.com/test.png"}`
	b.messages = []PrivateMessage{image, incoming(3), incoming(4)}
	b.sessions[0].LastMessage = &b.messages[2]
	if _, err := m.Poll(context.Background(), func(int64, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(b.sent) != 1 {
		t.Fatalf("rate limit sends=%v", b.sent)
	}
}

func TestKeywordExactAndFirstMatch(t *testing.T) {
	rules := []config.BilibiliKeyword{{Keyword: "hi", Reply: "exact", Exact: true}, {Keyword: "hi", Reply: "contains"}, {Keyword: "hi", Reply: "later"}}
	if got := MatchKeyword(rules, " hi "); got != "exact" {
		t.Fatal(got)
	}
	if got := MatchKeyword(rules, "HI there"); got != "contains" {
		t.Fatal(got)
	}
}

func TestPersistedAttemptSurvivesFailureToSaveFinalCursor(t *testing.T) {
	m := managerFixture()
	bootstrap(t, m)
	_ = m.SetMode("keyword")
	_ = m.SetRule(config.BilibiliKeyword{Keyword: "你好", Reply: "hello"})
	b := m.backend.(*fakeBackend)
	b.messages = []PrivateMessage{incoming(2)}
	b.sessions[0].LastMessage = &b.messages[0]
	store := m.store.(*memoryStore)
	store.data = []byte(`{"since":1000000000,"started_at":1000,"mode":"keyword","rules":[{"keyword":"你好","reply":"hello"}],"cursors":{"20":{"seq":1,"delivery_seq":2,"delivered_to":[99,98],"auto_seq":2,"last_reply":1000}}}`)
	m.loaded = false
	if _, err := m.Poll(context.Background(), func(int64, string) error { t.Fatal("already delivered notification repeated"); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(b.sent) != 0 {
		t.Fatal("persisted send attempt repeated")
	}
	var stored checkpoint
	if err := json.Unmarshal(store.data, &stored); err != nil || stored.Cursors[20].Seq != 2 {
		t.Fatalf("cursor not finished: %v %v", stored, err)
	}
}
