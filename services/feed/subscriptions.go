package feed

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
)

func (m *Manager) Add(groupID, createdBy int64, rawURL, name string) (*db.FeedSubscription, *Feed, error) {
	m.runMu.Lock()
	defer m.runMu.Unlock()

	rawURL, err := normalizeSubscriptionURL(rawURL)
	if err != nil {
		return nil, nil, err
	}
	name = cleanFeedText(name, 64)
	count, err := m.store.CountFeedSubscriptions(groupID)
	if err != nil {
		return nil, nil, err
	}
	if count >= MaxSubscriptionsPerGroup {
		return nil, nil, ErrSubscriptionLimit
	}
	existing, err := m.store.ListFeedSubscriptions(groupID)
	if err != nil {
		return nil, nil, err
	}
	for _, row := range existing {
		if row.URL == rawURL {
			return nil, nil, ErrDuplicateSubscription
		}
	}

	setting, err := m.store.GetFeedGroupSetting(groupID)
	if err != nil {
		return nil, nil, err
	}
	interval := time.Duration(setting.PollIntervalMinutes) * time.Minute
	parsed, err := m.fetch(rawURL)
	if err != nil {
		return nil, nil, fmt.Errorf("读取订阅源失败: %s", cleanFeedText(err.Error(), 200))
	}
	if name == "" {
		name = parsed.Title
	}
	if name == "" {
		name = "未命名订阅"
	}
	lastItemID := ""
	if len(parsed.Items) > 0 {
		lastItemID = parsed.Items[0].Key
	}
	row, err := m.store.CreateFeedSubscription(groupID, createdBy, rawURL, name, lastItemID)
	if err != nil {
		return nil, nil, err
	}
	checked := m.now()
	if _, err := m.store.RecordFeedFetchSuccess(row.ID, lastItemID, checked.Unix(), checked.Add(interval).Unix(), nil); err != nil {
		_ = m.store.DeleteFeedSubscription(row.ID, groupID)
		return nil, nil, err
	}
	row.LastCheckedAt = checked.Unix()
	row.LastSuccessAt = checked.Unix()
	row.NextCheckAt = checked.Add(interval).Unix()
	row.UpdatedAt = checked.Unix()
	return row, parsed, nil
}

func (m *Manager) AddPlatform(groupID, createdBy int64, kind PlatformKind, target, name string) (*db.FeedSubscription, *Feed, error) {
	rawURL, err := BuildPlatformURL(config.Current().Feed.RSSHubBase, kind, target)
	if err != nil {
		return nil, nil, err
	}
	return m.Add(groupID, createdBy, rawURL, name)
}

func (m *Manager) Remove(id uint, groupID int64) error {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	return m.store.DeleteFeedSubscription(id, groupID)
}

func (m *Manager) SetEnabled(id uint, groupID int64, enabled bool) (*db.FeedSubscription, error) {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	return m.store.SetFeedSubscriptionEnabled(id, groupID, enabled)
}

func (m *Manager) SetTranslation(id uint, groupID int64, translateToChinese bool) (*db.FeedSubscription, error) {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	return m.store.SetFeedSubscriptionTranslation(id, groupID, translateToChinese)
}

func (m *Manager) List(groupID int64) ([]db.FeedSubscription, error) {
	return m.store.ListFeedSubscriptions(groupID)
}

func normalizeSubscriptionURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if len(rawURL) > 1024 {
		return "", fmt.Errorf("订阅地址过长")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("请输入有效的 http/https 订阅地址")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("订阅地址不能包含用户信息")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	return parsed.String(), nil
}

func (m *Manager) ListAll() ([]db.FeedSubscription, error) { return m.store.ListAllFeedSubscriptions() }

func (m *Manager) GroupDisabled(groupID int64) (bool, error) {
	return m.store.IsGroupPluginDisabled(groupID, catalog.PluginFeedSubscription)
}
