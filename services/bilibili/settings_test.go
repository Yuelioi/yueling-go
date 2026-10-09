package bilibili

import (
	"context"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Yuelioi/yueling-go/config"
)

func TestSettingsPersistAndCommandsShareSavedRules(t *testing.T) {
	m := managerFixture()
	bootstrap(t, m)
	s, err := m.Settings()
	if err != nil {
		t.Fatal(err)
	}
	s.Enabled, s.PollSeconds, s.NotifyUsers = false, 300, []int64{77}
	s.ReplyMode = "keyword"
	s.Keywords = []config.BilibiliKeyword{{Keyword: "hi", Reply: "hello", Exact: true}}
	s.AIPrompt = "custom prompt"
	if err := m.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if m.CanManage(99) || !m.CanManage(77) {
		t.Fatal("notification permissions not updated")
	}
	if err := m.SetMode("off"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetRule(config.BilibiliKeyword{Keyword: "next", Reply: "reply"}); err != nil {
		t.Fatal(err)
	}
	restarted := NewManager(m.backend, m.store, m.cfg, []int64{99}, nil)
	got, err := restarted.Settings()
	if err != nil || got.Enabled || got.PollSeconds != 300 || got.ReplyMode != "off" || len(got.Keywords) != 2 || got.AIPrompt != "custom prompt" || !restarted.CanManage(77) {
		t.Fatalf("settings not restored: %+v err=%v", got, err)
	}
	if len(restarted.state.Cursors) == 0 || restarted.state.StartedAt == 0 {
		t.Fatal("settings replaced cursors")
	}
}

func TestInvalidOrFailedSettingsDoNotTakeEffect(t *testing.T) {
	m := managerFixture()
	bootstrap(t, m)
	s, _ := m.Settings()
	s.PollSeconds = 5
	s.Enabled = false
	if err := m.SaveSettings(s); err == nil {
		t.Fatal("disabled settings bypassed validation")
	}
	s.PollSeconds = 300
	m.store.(*memoryStore).fail = true
	if err := m.SaveSettings(s); err == nil {
		t.Fatal("failed save accepted")
	}
	m.store.(*memoryStore).fail = false
	got, err := m.Settings()
	if err != nil || got.PollSeconds == 300 {
		t.Fatalf("failed save effective: %+v %v", got, err)
	}
}

func TestSettingsOwnCallerSlices(t *testing.T) {
	m := managerFixture()
	bootstrap(t, m)
	settings, err := m.Settings()
	if err != nil {
		t.Fatal(err)
	}
	settings.NotifyUsers = []int64{77}
	settings.Keywords = []config.BilibiliKeyword{{Keyword: "hello", Reply: "original"}}
	if err := m.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	settings.NotifyUsers[0] = 88
	settings.Keywords[0].Reply = "changed"
	if m.state.Settings.NotifyUsers[0] != 77 || m.state.Settings.Keywords[0].Reply != "original" {
		t.Fatal("caller mutated persisted settings")
	}
}

func TestAutomaticPollingUsesTimerAndLiveSwitch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := managerFixture()
		bootstrap(t, m)
		s, _ := m.Settings()
		s.Enabled = true
		s.PollSeconds = 60
		if err := m.SaveSettings(s); err != nil {
			t.Fatal(err)
		}
		// The update predates the worker and is already represented by its initial poll.
		<-m.changed
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		connected := true
		go m.Run(ctx, func() bool { return connected }, func(int64, string) error { return nil }, func(err error) { t.Error(err) })
		synctest.Wait()
		lastCheck := func() time.Time { m.mu.Lock(); defer m.mu.Unlock(); return m.lastCheck }
		setNow := func(at int64) { m.mu.Lock(); defer m.mu.Unlock(); m.now = func() time.Time { return time.Unix(at, 0) } }
		first := lastCheck()
		time.Sleep(59 * time.Second)
		synctest.Wait()
		if !lastCheck().Equal(first) {
			t.Fatal("polled early")
		}
		setNow(2000)
		time.Sleep(time.Second)
		synctest.Wait()
		if lastCheck().Unix() != 2000 {
			t.Fatal("timer did not poll without a command")
		}
		s.Enabled = false
		s.PollSeconds = 300
		if err := m.SaveSettings(s); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		setNow(3000)
		time.Sleep(600 * time.Second)
		synctest.Wait()
		if lastCheck().Unix() != 2000 {
			t.Fatal("disabled worker continued polling")
		}
		s.Enabled = true
		if err := m.SaveSettings(s); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		time.Sleep(299 * time.Second)
		synctest.Wait()
		if lastCheck().Unix() != 2000 {
			t.Fatal("new interval not respected")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if lastCheck().Unix() != 3000 {
			t.Fatal("enabled worker did not resume")
		}
	})
}

func TestReplyUsesStableContactNumbersAndSingleContactShortcut(t *testing.T) {
	m := managerFixture()
	bootstrap(t, m)
	if _, _, _, err := m.ResolveReply("收到"); err == nil {
		t.Fatal("historical baseline used as a reply contact")
	}
	b := m.backend.(*fakeBackend)
	msg := incoming(2)
	b.messages = []PrivateMessage{msg}
	b.sessions[0].LastMessage = &msg
	var notification string
	if _, err := m.Poll(context.Background(), func(_ int64, text string) error { notification = text; return nil }); err != nil {
		t.Fatal(err)
	}
	uid, n, text, err := m.ResolveReply("收到\n第二行  空格")
	if err != nil || uid != 20 || n != 1 || text != "收到\n第二行  空格" || !strings.Contains(notification, "B站回复 1") {
		t.Fatalf("direct reply %d %d %q %v", uid, n, text, err)
	}
	second := incoming(3)
	second.SenderID = 30
	b.messages = []PrivateMessage{second}
	b.sessions = []Session{{TalkerID: 30, Type: 1, LastMessage: &second}}
	if _, err := m.Poll(context.Background(), func(int64, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := m.ResolveReply("收到"); err == nil {
		t.Fatal("ambiguous target accepted")
	}
	uid, n, text, err = m.ResolveReply("1 你好\n保持  空格")
	if err != nil || uid != 20 || n != 1 || text != "你好\n保持  空格" {
		t.Fatalf("numbered %d %d %q %v", uid, n, text, err)
	}
	if _, _, _, err := m.ResolveReply("20 hello"); err == nil {
		t.Fatal("UID treated as an unlisted target")
	}
	restarted := NewManager(b, m.store, m.cfg, m.notifyUsers, nil)
	uid, n, _, err = restarted.ResolveReply("2 hi")
	if err != nil || uid != 30 || n != 2 {
		t.Fatalf("restart numbering %d %d %v", uid, n, err)
	}
}
