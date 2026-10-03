package runtimeconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
)

var ErrConflict = errors.New("配置已更新，请刷新后重试")
var ErrPersistence = errors.New("配置保存失败，请稍后重试")

type Store interface {
	Load() ([]byte, int64, error)
	Seed([]byte) error
	Save([]byte, int64) error
}

type PostgresStore struct{}

func (PostgresStore) Load() ([]byte, int64, error) { return db.LoadRuntimeConfig() }
func (PostgresStore) Seed(p []byte) error          { return db.SeedRuntimeConfig(p) }
func (PostgresStore) Save(p []byte, r int64) error { return db.SaveRuntimeConfig(p, r) }

type Manager struct {
	mu       sync.Mutex
	store    Store
	revision int64
	validate func(config.Config) error
	apply    func(config.Config, config.Config, bool)
}

func New(store Store, validate func(config.Config) error) *Manager {
	return &Manager{store: store, validate: validate}
}

func (m *Manager) OnApply(fn func(config.Config, config.Config, bool)) { m.apply = fn }

func (m *Manager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current := config.Current()
	current.Bot.OwnerID = 0
	p, _, err := m.store.Load()
	if err != nil {
		return err
	}
	if len(p) == 0 {
		current.AI.ImportLegacyProfiles()
		if err = m.check(current); err != nil {
			return err
		}
		p, err = payload(current)
		if err != nil {
			return err
		}
		if err = m.store.Seed(p); err != nil {
			return err
		}
	}
	p, revision, err := m.store.Load()
	if err != nil {
		return err
	}
	candidate, err := merge(current, p)
	if err != nil {
		return fmt.Errorf("数据库配置格式无效")
	}
	migrated := candidate.AI.ImportLegacyProfiles()
	if err = m.check(candidate); err != nil {
		return err
	}
	if migrated {
		data, err := payload(candidate)
		if err != nil {
			return err
		}
		if err = m.store.Save(data, revision); err != nil {
			return ErrPersistence
		}
		revision++
	}
	config.Publish(candidate)
	m.revision = revision
	return nil
}

func (m *Manager) check(c config.Config) error {
	if err := Validate(c); err != nil {
		return err
	}
	if m.validate != nil {
		return m.validate(c)
	}
	return nil
}

type Update struct {
	Revision     int64                      `json:"revision"`
	Section      string                     `json:"section"`
	Values       map[string]json.RawMessage `json:"values"`
	ClearSecrets []string                   `json:"clear_secrets"`
}

func (m *Manager) Update(req Update) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if req.Revision != m.revision {
		return false, ErrConflict
	}
	old := config.Current()
	data, err := json.Marshal(old)
	if err != nil {
		return false, err
	}
	var document map[string]any
	if err = json.Unmarshal(data, &document); err != nil {
		return false, err
	}
	known := map[string]Field{}
	for _, section := range fields(old) {
		for _, f := range section.Fields {
			known[f.Path] = f
		}
	}
	for path := range legacyAIPaths {
		known[path] = Field{Path: path, Label: labels[path], Secret: secrets[path]}
	}
	if !slices.Contains(sectionOrder, req.Section) {
		return false, fmt.Errorf("未知配置分组")
	}
	if err := validateProfileUpdateMix(req); err != nil {
		return false, err
	}
	for _, path := range req.ClearSecrets {
		f, ok := known[path]
		if !ok || !f.Secret || !strings.HasPrefix(path, req.Section+".") {
			return false, fmt.Errorf("无效的清除凭据字段")
		}
		setPath(document, path, "")
	}
	for path, raw := range req.Values {
		f, ok := known[path]
		if !ok || !strings.HasPrefix(path, req.Section+".") {
			return false, fmt.Errorf("配置字段不可修改：%s", path)
		}
		if path == "ai.profiles" {
			profiles, err := decodeProfiles(raw, old.AI.Profiles)
			if err != nil {
				return false, err
			}
			setPath(document, path, profiles)
			continue
		}
		var value any
		if err = json.Unmarshal(raw, &value); err != nil {
			return false, fmt.Errorf("%s 格式无效", f.Label)
		}
		if f.Secret {
			text, ok := value.(string)
			if !ok {
				return false, fmt.Errorf("%s 必须是文本", f.Label)
			}
			if text == "" {
				continue
			}
		}
		setPath(document, path, value)
	}
	data, err = json.Marshal(document)
	if err != nil {
		return false, err
	}
	candidate, err := merge(old, data)
	if err != nil {
		return false, fmt.Errorf("配置类型无效，请检查数字、列表和开关")
	}
	if err = applyLegacyAIUpdate(&candidate.AI, req); err != nil {
		return false, err
	}
	candidate.AI = candidate.AI.Resolved()
	if err = m.check(candidate); err != nil {
		return false, err
	}
	data, err = payload(candidate)
	if err != nil {
		return false, err
	}
	if err = m.store.Save(data, m.revision); err != nil {
		return false, ErrPersistence
	}
	config.Publish(candidate)
	m.revision++
	reload := NeedsReload(old, candidate)
	if m.apply != nil {
		m.apply(old, candidate, reload)
	}
	return reload, nil
}

func merge(base config.Config, p []byte) (config.Config, error) {
	var c config.Config
	var document map[string]json.RawMessage
	if err := json.Unmarshal(p, &document); err != nil {
		return c, err
	}
	if raw, ok := document["ai"]; ok {
		var ai map[string]json.RawMessage
		if err := json.Unmarshal(raw, &ai); err != nil {
			return c, err
		}
		if profiles, ok := ai["profiles"]; !ok || bytes.Equal(bytes.TrimSpace(profiles), []byte("null")) {
			base.AI.Profiles = nil
			base.AI.ConversationProfile, base.AI.VisionProfile = "", ""
		}
	}
	copy, _ := json.Marshal(base)
	if err := json.Unmarshal(copy, &c); err != nil {
		return c, err
	}
	decoder := json.NewDecoder(bytes.NewReader(p))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&c)
	return c, err
}

func payload(c config.Config) ([]byte, error) {
	p, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if err = json.Unmarshal(p, &document); err != nil {
		return nil, err
	}
	if c.AI.Profiles != nil {
		ai := document["ai"].(map[string]any)
		for _, key := range []string{"deepseek_key", "base_url", "model", "vl"} {
			delete(ai, key)
		}
	}
	delete(document["database"].(map[string]any), "dsn")
	delete(document["bot"].(map[string]any), "data_dir")
	delete(document["bot"].(map[string]any), "owner_id")
	return json.Marshal(document)
}

func setPath(root map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	for _, part := range parts[:len(parts)-1] {
		root = root[part].(map[string]any)
	}
	root[parts[len(parts)-1]] = value
}

func NeedsReload(a, b config.Config) bool {
	return !reflect.DeepEqual(a.NapCat, b.NapCat) || !reflect.DeepEqual(a.HTTPAPI, b.HTTPAPI) || !reflect.DeepEqual(a.WebUI, b.WebUI) ||
		a.Tools.MemeServer != b.Tools.MemeServer || a.Bot.Timezone != b.Bot.Timezone || !reflect.DeepEqual(a.Image.Entry, b.Image.Entry) || a.Bilibili.Cookie != b.Bilibili.Cookie
}
