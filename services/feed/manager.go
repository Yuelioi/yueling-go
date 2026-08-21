package feed

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services/logx"
)

const (
	MaxSubscriptionsPerGroup = 10
	pollInterval             = 10 * time.Minute
	initialPollDelay         = 30 * time.Second
	maxItemsPerNotification  = 5
	maxPendingPerDelivery    = 12
	maxConcurrentFetches     = 4
	maxFailureBackoff        = 6 * time.Hour
)

var (
	ErrSubscriptionLimit     = errors.New("本群订阅数量已达上限")
	ErrDuplicateSubscription = errors.New("本群已经订阅过这个地址")
)

type Sender interface {
	SendGroupText(groupID int64, text string) error
}

type Fetcher func(rawURL string) (*Feed, error)

type CheckResult struct {
	Checked   int `json:"checked"`
	Updated   int `json:"updated"`
	Items     int `json:"items"`
	Delivered int `json:"delivered"`
	Queued    int `json:"queued"`
	Failed    int `json:"failed"`
}

type Manager struct {
	fetch Fetcher
	now   func() time.Time

	startMu sync.Mutex
	cancel  context.CancelFunc
	runMu   sync.Mutex
}

func NewManager(fetcher Fetcher) *Manager {
	if fetcher == nil {
		fetcher = Fetch
	}
	return &Manager{fetch: fetcher, now: time.Now}
}

var DefaultManager = NewManager(Fetch)

func (m *Manager) Add(groupID, createdBy int64, rawURL, name string) (*db.FeedSubscription, *Feed, error) {
	m.runMu.Lock()
	defer m.runMu.Unlock()

	rawURL, err := normalizeSubscriptionURL(rawURL)
	if err != nil {
		return nil, nil, err
	}
	name = cleanFeedText(name, 64)
	count, err := db.CountFeedSubscriptions(groupID)
	if err != nil {
		return nil, nil, err
	}
	if count >= MaxSubscriptionsPerGroup {
		return nil, nil, ErrSubscriptionLimit
	}
	existing, err := db.ListFeedSubscriptions(groupID)
	if err != nil {
		return nil, nil, err
	}
	for _, row := range existing {
		if row.URL == rawURL {
			return nil, nil, ErrDuplicateSubscription
		}
	}

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
	row, err := db.CreateFeedSubscription(groupID, createdBy, rawURL, name, lastItemID)
	if err != nil {
		return nil, nil, err
	}
	checked := m.now()
	if _, err := db.RecordFeedFetchSuccess(row.ID, lastItemID, checked.Unix(), checked.Add(pollInterval).Unix(), nil); err != nil {
		_ = db.DeleteFeedSubscription(row.ID, groupID)
		return nil, nil, err
	}
	row.LastCheckedAt = checked.Unix()
	row.LastSuccessAt = checked.Unix()
	row.NextCheckAt = checked.Add(pollInterval).Unix()
	row.UpdatedAt = checked.Unix()
	return row, parsed, nil
}

func (m *Manager) AddPlatform(groupID, createdBy int64, kind PlatformKind, target, name string) (*db.FeedSubscription, *Feed, error) {
	rawURL, err := BuildPlatformURL(config.C.Feed.RSSHubBase, kind, target)
	if err != nil {
		return nil, nil, err
	}
	return m.Add(groupID, createdBy, rawURL, name)
}

func (m *Manager) Remove(id uint, groupID int64) error {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	return db.DeleteFeedSubscription(id, groupID)
}

func (m *Manager) SetEnabled(id uint, groupID int64, enabled bool) (*db.FeedSubscription, error) {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	return db.SetFeedSubscriptionEnabled(id, groupID, enabled)
}

func (m *Manager) List(groupID int64) ([]db.FeedSubscription, error) {
	return db.ListFeedSubscriptions(groupID)
}

func (m *Manager) CheckGroup(sender Sender, groupID int64) (CheckResult, error) {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	rows, err := db.ListActiveFeedSubscriptions(groupID)
	if err != nil {
		return CheckResult{}, err
	}
	return m.pollRows(sender, rows, []int64{groupID}), nil
}

func (m *Manager) PollAll(sender Sender) (CheckResult, error) {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	now := m.now().Unix()
	rows, err := db.ListDueFeedSubscriptions(now)
	if err != nil {
		return CheckResult{}, err
	}
	groupIDs, err := db.ListFeedPendingGroupIDs()
	if err != nil {
		return CheckResult{}, err
	}
	for _, row := range rows {
		groupIDs = append(groupIDs, row.GroupID)
	}
	return m.pollRows(sender, rows, uniqueGroupIDs(groupIDs)), nil
}

func (m *Manager) Start(sender Sender) {
	if sender == nil {
		return
	}
	m.startMu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.startMu.Unlock()

	go func() {
		initial := time.NewTimer(initialPollDelay)
		defer initial.Stop()
		select {
		case <-ctx.Done():
			return
		case <-initial.C:
		}

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			result, err := m.PollAll(sender)
			if err != nil {
				logx.Errorf("[feed] polling failed: %v", err)
			} else if result.Updated > 0 || result.Delivered > 0 || result.Failed > 0 {
				logx.Infof("[feed] checked=%d updated=%d items=%d delivered=%d queued=%d failed=%d",
					result.Checked, result.Updated, result.Items, result.Delivered, result.Queued, result.Failed)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

type fetchedFeed struct {
	feed *Feed
	err  error
}

func (m *Manager) pollRows(sender Sender, rows []db.FeedSubscription, deliveryGroups []int64) CheckResult {
	result := CheckResult{Checked: len(rows)}
	if sender == nil {
		return result
	}
	now := m.now()
	checkedAt := now.Unix()

	urls := make([]string, 0, len(rows))
	seenURLs := map[string]bool{}
	for _, row := range rows {
		if !seenURLs[row.URL] {
			seenURLs[row.URL] = true
			urls = append(urls, row.URL)
		}
	}
	sort.Strings(urls)

	fetched := make(map[string]fetchedFeed, len(urls))
	var fetchedMu sync.Mutex
	semaphore := make(chan struct{}, maxConcurrentFetches)
	var wait sync.WaitGroup
	for _, rawURL := range urls {
		rawURL := rawURL
		wait.Add(1)
		go func() {
			defer wait.Done()
			semaphore <- struct{}{}
			parsed, err := m.fetch(rawURL)
			<-semaphore
			fetchedMu.Lock()
			fetched[rawURL] = fetchedFeed{feed: parsed, err: err}
			fetchedMu.Unlock()
		}()
	}
	wait.Wait()

	disabledGroups := make(map[int64]bool)
	for _, row := range rows {
		loaded := fetched[row.URL]
		if loaded.err != nil || loaded.feed == nil {
			result.Failed++
			message := cleanFeedText(fmt.Sprint(loaded.err), 200)
			if loaded.feed == nil && loaded.err == nil {
				message = "订阅源返回空响应"
			}
			failures := row.ConsecutiveFailures + 1
			nextCheckAt := now.Add(feedFailureBackoff(failures)).Unix()
			if err := db.RecordFeedFetchFailure(row.ID, failures, message, checkedAt, nextCheckAt); err != nil {
				logx.Warnf("[feed] record failure subscription=%d: %v", row.ID, err)
			}
			logx.Warnf("[feed] fetch subscription=%d url=%s failed: %s", row.ID, row.URL, message)
			continue
		}

		disabled, known := disabledGroups[row.GroupID]
		if !known {
			var err error
			disabled, err = db.IsGroupPluginDisabled(row.GroupID, catalog.PluginFeedSubscription)
			if err != nil {
				result.Failed++
				continue
			}
			disabledGroups[row.GroupID] = disabled
		}
		newestID := row.LastItemID
		if len(loaded.feed.Items) > 0 {
			newestID = loaded.feed.Items[0].Key
		}
		nextCheckAt := now.Add(pollInterval).Unix()
		if disabled {
			if _, err := db.RecordFeedFetchSuccess(row.ID, newestID, checkedAt, nextCheckAt, nil); err != nil {
				result.Failed++
				logx.Warnf("[feed] update disabled subscription=%d: %v", row.ID, err)
			}
			continue
		}

		items := itemsSince(loaded.feed.Items, row.LastItemID)
		pending := make([]db.FeedPendingItem, 0, len(items))
		for index := len(items) - 1; index >= 0; index-- {
			item := items[index]
			publishedAt := int64(0)
			if !item.Published.IsZero() {
				publishedAt = item.Published.Unix()
			}
			pending = append(pending, db.FeedPendingItem{
				SubscriptionID: row.ID,
				GroupID:        row.GroupID,
				FeedName:       row.Name,
				ItemKey:        item.Key,
				Title:          item.Title,
				Link:           item.Link,
				PublishedAt:    publishedAt,
				QueuedAt:       checkedAt,
			})
		}
		inserted, err := db.RecordFeedFetchSuccess(row.ID, newestID, checkedAt, nextCheckAt, pending)
		if err != nil {
			result.Failed++
			logx.Warnf("[feed] store outbox subscription=%d failed: %v", row.ID, err)
			continue
		}
		if inserted > 0 {
			result.Updated++
			result.Items += int(inserted)
		}
	}

	for _, groupID := range uniqueGroupIDs(deliveryGroups) {
		disabled, known := disabledGroups[groupID]
		if !known {
			var err error
			disabled, err = db.IsGroupPluginDisabled(groupID, catalog.PluginFeedSubscription)
			if err != nil {
				result.Failed++
				continue
			}
		}
		if disabled {
			if err := db.DeleteFeedPendingItemsForGroup(groupID); err != nil {
				result.Failed++
			}
			continue
		}
		setting, err := db.GetFeedGroupSetting(groupID)
		if err != nil {
			result.Failed++
			continue
		}
		if isQuietTime(setting, now) {
			continue
		}
		pending, err := db.ListFeedPendingItems(groupID, maxPendingPerDelivery)
		if err != nil {
			result.Failed++
			continue
		}
		if len(pending) == 0 {
			continue
		}
		if err := sender.SendGroupText(groupID, formatPendingNotification(pending, setting.ItemMaxChars)); err != nil {
			result.Failed++
			logx.Warnf("[feed] deliver group=%d failed: %v", groupID, err)
			continue
		}
		ids := make([]uint, 0, len(pending))
		for _, item := range pending {
			ids = append(ids, item.ID)
		}
		if err := db.DeleteFeedPendingItems(groupID, ids); err != nil {
			result.Failed++
			logx.Warnf("[feed] clear outbox group=%d failed: %v", groupID, err)
			continue
		}
		result.Delivered += len(pending)
	}
	for _, groupID := range uniqueGroupIDs(deliveryGroups) {
		count, err := db.CountFeedPendingItems(groupID)
		if err != nil {
			result.Failed++
			continue
		}
		result.Queued += int(count)
	}
	return result
}

func itemsSince(items []Item, lastItemID string) []Item {
	if len(items) == 0 {
		return nil
	}
	limit := maxItemsPerNotification
	if lastItemID == "" {
		return append([]Item(nil), items[:min(len(items), limit)]...)
	}
	for index, item := range items {
		if item.Key == lastItemID {
			return append([]Item(nil), items[:min(index, limit)]...)
		}
	}
	// The previous cursor fell out of the feed window. Avoid flooding the group;
	// announce only the newest entry and continue from there.
	return append([]Item(nil), items[:1]...)
}

func formatPendingNotification(items []db.FeedPendingItem, itemMaxChars int) string {
	var builder strings.Builder
	builder.WriteString("📡 订阅更新")
	lastFeed := ""
	for _, item := range items {
		if item.FeedName != lastFeed {
			builder.WriteString("\n\n【")
			builder.WriteString(item.FeedName)
			builder.WriteString("】")
			lastFeed = item.FeedName
		}
		builder.WriteString("\n\n• ")
		title := item.Title
		if itemMaxChars > 0 {
			title = cleanFeedText(title, itemMaxChars)
		}
		builder.WriteString(title)
		if item.Link != "" {
			builder.WriteByte('\n')
			builder.WriteString(item.Link)
		}
	}
	return builder.String()
}

func feedFailureBackoff(failures int) time.Duration {
	if failures < 1 {
		failures = 1
	}
	delay := pollInterval
	for attempt := 1; attempt < failures && delay < maxFailureBackoff; attempt++ {
		delay *= 2
	}
	if delay > maxFailureBackoff {
		return maxFailureBackoff
	}
	return delay
}

func uniqueGroupIDs(values []int64) []int64 {
	seen := make(map[int64]bool, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func ParseQuietHours(start, end string) (string, string, error) {
	start, err := normalizeClock(start)
	if err != nil {
		return "", "", fmt.Errorf("静默开始时间格式错误，应为 HH:MM")
	}
	end, err = normalizeClock(end)
	if err != nil {
		return "", "", fmt.Errorf("静默结束时间格式错误，应为 HH:MM")
	}
	if start == end {
		return "", "", fmt.Errorf("静默开始和结束时间不能相同")
	}
	return start, end, nil
}

func normalizeClock(value string) (string, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return "", errors.New("invalid clock")
	}
	hour, hourErr := strconv.Atoi(parts[0])
	minute, minuteErr := strconv.Atoi(parts[1])
	if hourErr != nil || minuteErr != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return "", errors.New("invalid clock")
	}
	return fmt.Sprintf("%02d:%02d", hour, minute), nil
}

func (m *Manager) SetQuietHours(groupID int64, enabled bool, start, end string) (db.FeedGroupSetting, error) {
	m.runMu.Lock()
	defer m.runMu.Unlock()

	setting, err := db.GetFeedGroupSetting(groupID)
	if err != nil {
		return db.FeedGroupSetting{}, err
	}
	return setDeliverySettings(groupID, enabled, start, end, setting.ItemMaxChars)
}

func (m *Manager) SetDeliverySettings(groupID int64, enabled bool, start, end string, itemMaxChars int) (db.FeedGroupSetting, error) {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	return setDeliverySettings(groupID, enabled, start, end, itemMaxChars)
}

func setDeliverySettings(groupID int64, enabled bool, start, end string, itemMaxChars int) (db.FeedGroupSetting, error) {
	if err := validateItemMaxChars(itemMaxChars); err != nil {
		return db.FeedGroupSetting{}, err
	}
	if !enabled && (strings.TrimSpace(start) == "" || strings.TrimSpace(end) == "") {
		setting, err := db.GetFeedGroupSetting(groupID)
		if err != nil {
			return db.FeedGroupSetting{}, err
		}
		start, end = setting.QuietStart, setting.QuietEnd
	}
	start, end, err := ParseQuietHours(start, end)
	if err != nil {
		return db.FeedGroupSetting{}, err
	}
	return db.SetFeedGroupSetting(groupID, enabled, start, end, itemMaxChars)
}

func validateItemMaxChars(value int) error {
	if value != 0 && (value < MinItemMaxChars || value > MaxItemMaxChars) {
		return fmt.Errorf("订阅内容长度应为 0（完整）或 %d–%d 字", MinItemMaxChars, MaxItemMaxChars)
	}
	return nil
}

func (m *Manager) DeliveryStatus(groupID int64) (db.FeedGroupSetting, int64, error) {
	setting, err := db.GetFeedGroupSetting(groupID)
	if err != nil {
		return db.FeedGroupSetting{}, 0, err
	}
	pending, err := db.CountFeedPendingItems(groupID)
	return setting, pending, err
}

func isQuietTime(setting db.FeedGroupSetting, now time.Time) bool {
	if !setting.QuietEnabled {
		return false
	}
	start, startErr := clockMinutes(setting.QuietStart)
	end, endErr := clockMinutes(setting.QuietEnd)
	if startErr != nil || endErr != nil || start == end {
		return false
	}
	locationName := strings.TrimSpace(config.C.Bot.Timezone)
	if locationName == "" {
		locationName = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(locationName)
	if err != nil {
		location, _ = time.LoadLocation("Asia/Shanghai")
	}
	now = now.In(location)
	current := now.Hour()*60 + now.Minute()
	if start < end {
		return current >= start && current < end
	}
	return current >= start || current < end
}

func clockMinutes(value string) (int, error) {
	normalized, err := normalizeClock(value)
	if err != nil {
		return 0, err
	}
	parts := strings.Split(normalized, ":")
	hour, _ := strconv.Atoi(parts[0])
	minute, _ := strconv.Atoi(parts[1])
	return hour*60 + minute, nil
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
