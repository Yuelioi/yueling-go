package runtimeconfig

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

type memoryStore struct {
	data     []byte
	revision int64
	fail     bool
	seeds    int
}

func (s *memoryStore) Load() ([]byte, int64, error) { return s.data, s.revision, nil }
func (s *memoryStore) Seed(p []byte) error {
	s.seeds++
	s.data = append([]byte(nil), p...)
	s.revision = 1
	return nil
}
func (s *memoryStore) Save(p []byte, r int64) error {
	if s.fail {
		return errors.New("backend contains private details")
	}
	if r != s.revision {
		return ErrConflict
	}
	s.data = append([]byte(nil), p...)
	s.revision++
	return nil
}
func fixture(t *testing.T) (*Manager, *memoryStore) {
	t.Helper()
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	t.Setenv("YUELING_DATABASE_DSN", "postgres://test@localhost/test")
	t.Setenv("YUELING_WEBUI_PASSWORD", "fixture-secret")
	if err := config.LoadBootstrap(filepath.Join(t.TempDir(), "absent.toml")); err != nil {
		t.Fatal(err)
	}
	store := &memoryStore{}
	m := New(store, nil)
	if err := m.Initialize(); err != nil {
		t.Fatal(err)
	}
	return m, store
}
func update(m *Manager, section, path string, value any) (bool, error) {
	raw, _ := json.Marshal(value)
	return m.Update(Update{Revision: m.View().Revision, Section: section, Values: map[string]json.RawMessage{path: raw}})
}

func TestImportOnceDatabaseAuthorityAndBootstrapIsolation(t *testing.T) {
	m, s := fixture(t)
	if reload, err := update(m, "ai", "ai.model", "new-model"); err != nil || reload {
		t.Fatalf("update: %v %v", reload, err)
	}
	stale := config.Current()
	stale.AI.Model = "stale-file"
	stale.Database.DSN = "postgres://newhost@localhost/other"
	config.Publish(stale)
	restarted := New(s, nil)
	if err := restarted.Initialize(); err != nil {
		t.Fatal(err)
	}
	if config.Current().AI.Model != "new-model" || s.seeds != 1 || config.Current().Database.DSN != stale.Database.DSN {
		t.Fatal("database did not win, or bootstrap was persisted")
	}
	var document map[string]map[string]any
	json.Unmarshal(s.data, &document)
	if _, ok := document["database"]["dsn"]; ok {
		t.Fatal("DSN persisted")
	}
	if _, ok := document["bot"]["data_dir"]; ok {
		t.Fatal("data root persisted")
	}
	for _, path := range []string{"database.dsn", "bot.data_dir", "bot.owner_id"} {
		section := strings.Split(path, ".")[0]
		if _, err := update(restarted, section, path, "bad"); err == nil {
			t.Fatalf("bootstrap mutable: %s", path)
		}
	}
}

func TestReasoningEffortIsManagedOnlineAndSurvivesRestart(t *testing.T) {
	m, store := fixture(t)
	if reload, err := update(m, "ai", "ai.reasoning_effort", "high"); err != nil || reload {
		t.Fatalf("online update: reload=%v err=%v", reload, err)
	}
	stale := config.Current()
	stale.AI.ReasoningEffort = "low"
	config.Publish(stale)
	if err := New(store, nil).Initialize(); err != nil {
		t.Fatal(err)
	}
	if got := config.Current().AI.ReasoningEffort; got != "high" {
		t.Fatalf("database setting replaced by bootstrap: %q", got)
	}
}

func TestSecretsValidationConflictAndFailedSave(t *testing.T) {
	m, s := fixture(t)
	if _, err := update(m, "ai", "ai.deepseek_key", "private-ai-key"); err != nil {
		t.Fatal(err)
	}
	view, _ := json.Marshal(m.View())
	if strings.Contains(string(view), "private-ai-key") || strings.Contains(string(view), "fixture-secret") {
		t.Fatal("secret exposed")
	}
	if _, err := update(m, "ai", "ai.deepseek_key", ""); err != nil || config.Current().AI.DeepSeekKey != "private-ai-key" {
		t.Fatal("blank erased secret")
	}
	if _, err := m.Update(Update{Revision: m.View().Revision, Section: "ai", ClearSecrets: []string{"ai.deepseek_key"}}); err != nil || config.Current().AI.DeepSeekKey != "" {
		t.Fatal("explicit clear failed")
	}
	for _, request := range []Update{
		{Section: "ai", Values: map[string]json.RawMessage{"webui.password": json.RawMessage(`"bad"`)}},
		{Section: "ai", Values: map[string]json.RawMessage{"ai.max_tokens": json.RawMessage(`0`)}},
		{Section: "ai", Values: map[string]json.RawMessage{"ai.max_tokens": json.RawMessage(`1.5`)}},
		{Section: "bot", Values: map[string]json.RawMessage{"bot.superusers": json.RawMessage(`[-1]`)}},
		{Section: "bot", ClearSecrets: []string{"bot.name"}},
	} {
		request.Revision = m.View().Revision
		if _, err := m.Update(request); err == nil {
			t.Fatalf("invalid accepted: %+v", request)
		}
	}
	if _, err := m.Update(Update{Revision: 0, Section: "ai"}); !errors.Is(err, ErrConflict) {
		t.Fatal("revision conflict not reported")
	}
	before := config.Current()
	revision := m.View().Revision
	s.fail = true
	if _, err := update(m, "ai", "ai.model", "failed-model"); !errors.Is(err, ErrPersistence) {
		t.Fatalf("persistence failure: %v", err)
	}
	if config.Current().AI.Model != before.AI.Model || m.View().Revision != revision {
		t.Fatal("failed save changed runtime")
	}
}

func TestLiveApplyReloadAndConcurrentSnapshot(t *testing.T) {
	m, _ := fixture(t)
	calls := 0
	reloaded := false
	m.OnApply(func(_, next config.Config, reload bool) {
		calls++
		reloaded = reload
		if config.Current().AI.Model != next.AI.Model {
			t.Error("callback precedes snapshot")
		}
	})
	var wg sync.WaitGroup
	wg.Add(1)
	done := make(chan struct{})
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				_ = config.Current().AI.Model
			}
		}
	}()
	for i := 0; i < 20; i++ {
		if _, err := update(m, "ai", "ai.model", "model"); err != nil {
			t.Fatal(err)
		}
	}
	close(done)
	wg.Wait()
	if reloaded || calls != 20 {
		t.Fatal("live change reloads service")
	}
	if reload, err := update(m, "napcat", "napcat.token", "new-token"); err != nil || !reload || !reloaded {
		t.Fatal("connection reload missing")
	}
}

func TestManagersKeepConfigurationAndRevisionTogether(t *testing.T) {
	first, _ := fixture(t)
	if _, err := update(first, "bot", "bot.name", "first"); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(first.View())
	second := New(&memoryStore{}, nil)
	if err := second.Initialize(); err != nil {
		t.Fatal(err)
	}
	if _, err := update(second, "bot", "bot.name", "second"); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(first.View())
	if string(before) != string(after) {
		t.Fatal("another manager changed this revision's view")
	}
	if _, err := update(first, "pack", "pack.max_images", 40); err != nil {
		t.Fatal(err)
	}
	if config.Current().Bot.Name != "first" {
		t.Fatal("update merged another manager's configuration")
	}
}

func TestStoreRevisionConflictDoesNotPublish(t *testing.T) {
	m, store := fixture(t)
	previous, _ := json.Marshal(m.View())
	store.revision++
	_, err := update(m, "bot", "bot.name", "stale")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("error=%v want conflict", err)
	}
	current, _ := json.Marshal(m.View())
	if string(previous) != string(current) {
		t.Fatal("conflict changed manager snapshot")
	}
}

func TestViewAndApplyCallbackCannotMutateStoredSnapshot(t *testing.T) {
	m, _ := fixture(t)
	m.OnApply(func(_, next config.Config, _ bool) { next.Bot.SuperUsers[0] = 999 })
	if _, err := update(m, "bot", "bot.superusers", []int64{42}); err != nil {
		t.Fatal(err)
	}
	for _, section := range m.View().Sections {
		for _, field := range section.Fields {
			if field.Path == "bot.superusers" {
				field.Value.([]int64)[0] = 1000
			}
		}
	}
	if config.Current().Bot.SuperUsers[0] != 42 {
		t.Fatal("callback modified published configuration")
	}
	for _, section := range m.View().Sections {
		for _, field := range section.Fields {
			if field.Path == "bot.superusers" && field.Value.([]int64)[0] != 42 {
				t.Fatal("view modified manager configuration")
			}
		}
	}
}

func TestRestartSettingsStaySavedUntilNextInitialize(t *testing.T) {
	m, store := fixture(t)
	before := config.Current()
	applied := m.View().AppliedRevision
	reload, err := update(m, "napcat", "napcat.token", "saved-new-token")
	if err != nil || !reload {
		t.Fatalf("save: reload=%v err=%v", reload, err)
	}
	if config.Current().NapCat.Token != before.NapCat.Token {
		t.Fatal("startup setting changed before restart")
	}
	if !m.View().RestartPending || m.View().AppliedRevision != applied {
		t.Fatal("missing pending state")
	}
	if _, err := update(m, "bot", "bot.name", "live-name"); err != nil {
		t.Fatal(err)
	}
	if config.Current().Bot.Name != "live-name" || !m.View().RestartPending {
		t.Fatal("hot apply lost pending restart")
	}
	store.fail = true
	revision := m.View().Revision
	if _, err := update(m, "napcat", "napcat.token", "failed-token"); err == nil {
		t.Fatal("save should fail")
	}
	if m.View().Revision != revision {
		t.Fatal("failed save changed revision")
	}
	store.fail = false
	restarted := New(store, nil)
	if err := restarted.Initialize(); err != nil {
		t.Fatal(err)
	}
	if config.Current().NapCat.Token != "saved-new-token" {
		t.Fatal("saved token not applied on restart")
	}
	view := restarted.View()
	if view.RestartPending || view.AppliedRevision != view.Revision {
		t.Fatal("restart did not acknowledge revision")
	}
}

func TestBlacklistHotUpdateValidationAndPersistence(t *testing.T) {
	m, store := fixture(t)
	if reload, err := update(m, "bot", "bot.blacklist", []int64{12345}); err != nil || reload {
		t.Fatalf("blacklist update: reload=%v err=%v", reload, err)
	}
	if !config.UserBlocked(12345) || config.UserBlocked(54321) {
		t.Fatal("blacklist was not applied")
	}
	for _, ids := range [][]int64{{0}, {-1}} {
		if _, err := update(m, "bot", "bot.blacklist", ids); err == nil {
			t.Fatal("invalid QQ accepted")
		}
	}
	if err := New(store, nil).Initialize(); err != nil {
		t.Fatal(err)
	}
	if !config.UserBlocked(12345) {
		t.Fatal("blacklist lost after restart")
	}
	found := false
	for _, section := range m.View().Sections {
		for _, field := range section.Fields {
			if field.Path == "bot.blacklist" {
				found = field.Kind == "numbers" && field.Label == "全局黑名单 QQ" && !field.Reload
			}
		}
	}
	if !found {
		t.Fatal("blacklist missing from editable settings")
	}
	if _, err := update(m, "bot", "bot.blacklist", []int64{}); err != nil {
		t.Fatal(err)
	}
	if config.UserBlocked(12345) {
		t.Fatal("clearing blacklist did not unblock account")
	}
}
