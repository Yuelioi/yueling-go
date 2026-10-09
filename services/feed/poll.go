package feed

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services/logx"
)

func (m *Manager) CheckGroup(sender Sender, groupID int64) (CheckResult, error) {
	if !config.PluginEnabled(catalog.PluginFeedSubscription) {
		return CheckResult{}, fmt.Errorf("订阅插件已全局关闭")
	}
	m.runMu.Lock()
	defer m.runMu.Unlock()
	rows, err := m.store.ListActiveFeedSubscriptions(groupID)
	if err != nil {
		return CheckResult{}, err
	}
	return m.pollRows(sender, rows, []int64{groupID}), nil
}

func (m *Manager) CheckAll(sender Sender) (CheckResult, error) {
	if !config.PluginEnabled(catalog.PluginFeedSubscription) {
		return CheckResult{}, fmt.Errorf("订阅插件已全局关闭")
	}
	m.runMu.Lock()
	defer m.runMu.Unlock()
	rows, err := m.store.ListAllFeedSubscriptions()
	if err != nil {
		return CheckResult{}, err
	}
	groups, err := m.store.ListFeedPendingGroupIDs()
	if err != nil {
		return CheckResult{}, err
	}
	active := make([]db.FeedSubscription, 0, len(rows))
	for _, row := range rows {
		if row.Enabled {
			active = append(active, row)
			groups = append(groups, row.GroupID)
		}
	}
	return m.pollRows(sender, active, uniqueGroupIDs(groups)), nil
}

func (m *Manager) PollAll(sender Sender) (CheckResult, error) {
	return m.pollAll(context.Background(), sender)
}

func (m *Manager) pollAll(ctx context.Context, sender Sender) (CheckResult, error) {
	if !config.PluginEnabled(catalog.PluginFeedSubscription) {
		return CheckResult{}, nil
	}
	m.runMu.Lock()
	defer m.runMu.Unlock()
	now := m.now().Unix()
	rows, err := m.store.ListDueFeedSubscriptions(now)
	if err != nil {
		return CheckResult{}, err
	}
	groupIDs, err := m.store.ListFeedPendingGroupIDs()
	if err != nil {
		return CheckResult{}, err
	}
	for _, row := range rows {
		groupIDs = append(groupIDs, row.GroupID)
	}
	return m.pollRowsContext(ctx, sender, rows, uniqueGroupIDs(groupIDs)), nil
}

type fetchedFeed struct {
	feed *Feed
	err  error
}

func (m *Manager) pollRows(sender Sender, rows []db.FeedSubscription, deliveryGroups []int64) CheckResult {
	return m.pollRowsContext(context.Background(), sender, rows, deliveryGroups)
}

func (m *Manager) pollRowsContext(ctx context.Context, sender Sender, rows []db.FeedSubscription, deliveryGroups []int64) CheckResult {
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
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}
			if ctx.Err() != nil {
				<-semaphore
				return
			}
			parsed, err := m.fetchContext(ctx, rawURL)
			<-semaphore
			fetchedMu.Lock()
			fetched[rawURL] = fetchedFeed{feed: parsed, err: err}
			fetchedMu.Unlock()
		}()
	}
	wait.Wait()

	disabledGroups := make(map[int64]bool)
	for _, row := range rows {
		if ctx.Err() != nil {
			return result
		}
		loaded := fetched[row.URL]
		if loaded.err != nil || loaded.feed == nil {
			result.Failed++
			message := cleanFeedText(fmt.Sprint(loaded.err), 200)
			if loaded.feed == nil && loaded.err == nil {
				message = "订阅源返回空响应"
			}
			failures := row.ConsecutiveFailures + 1
			nextCheckAt := now.Add(feedFailureBackoff(failures)).Unix()
			if err := m.store.RecordFeedFetchFailure(row.ID, failures, message, checkedAt, nextCheckAt); err != nil {
				logx.Warnf("[feed] record failure subscription=%d: %v", row.ID, err)
			}
			logx.Warnf("[feed] fetch subscription=%d url=%s failed: %s", row.ID, row.URL, message)
			continue
		}

		disabled, known := disabledGroups[row.GroupID]
		if !known {
			var err error
			disabled, err = m.store.IsGroupPluginDisabled(row.GroupID, catalog.PluginFeedSubscription)
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
		setting, err := m.store.GetFeedGroupSetting(row.GroupID)
		if err != nil {
			result.Failed++
			continue
		}
		nextCheckAt := now.Add(time.Duration(setting.PollIntervalMinutes) * time.Minute).Unix()
		if disabled {
			if _, err := m.store.RecordFeedFetchSuccess(row.ID, newestID, checkedAt, nextCheckAt, nil); err != nil {
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
				Title:          itemDeliveryText(item),
				Link:           item.Link,
				PublishedAt:    publishedAt,
				QueuedAt:       checkedAt,
			})
		}
		inserted, err := m.store.RecordFeedFetchSuccess(row.ID, newestID, checkedAt, nextCheckAt, pending)
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
		if ctx.Err() != nil {
			return result
		}
		disabled, known := disabledGroups[groupID]
		if !known {
			var err error
			disabled, err = m.store.IsGroupPluginDisabled(groupID, catalog.PluginFeedSubscription)
			if err != nil {
				result.Failed++
				continue
			}
		}
		if disabled {
			if err := m.store.DeleteFeedPendingItemsForGroup(groupID); err != nil {
				result.Failed++
			}
			continue
		}
		setting, err := m.store.GetFeedGroupSetting(groupID)
		if err != nil {
			result.Failed++
			continue
		}
		if isQuietTime(setting, now) {
			continue
		}
		pending, err := m.store.ListFeedPendingItems(groupID, maxPendingPerDelivery)
		if err != nil {
			result.Failed++
			continue
		}
		if len(pending) == 0 {
			continue
		}
		subscriptions, err := m.store.ListFeedSubscriptions(groupID)
		if err != nil {
			result.Failed++
			continue
		}
		translatedSubscriptions := make(map[uint]bool, len(subscriptions))
		for _, subscription := range subscriptions {
			if subscription.TranslateToChinese {
				translatedSubscriptions[subscription.ID] = true
			}
		}
		deliveryItems := pending
		hasTranslationTarget := false
		for _, item := range pending {
			if translatedSubscriptions[item.SubscriptionID] {
				hasTranslationTarget = true
				break
			}
		}
		if hasTranslationTarget {
			translateCtx, cancel := context.WithTimeout(ctx, translationTimeout)
			translated, translateErr := translatePendingItemsMatching(translateCtx, pending, func(item db.FeedPendingItem) bool {
				return translatedSubscriptions[item.SubscriptionID]
			}, m.translate)
			cancel()
			if translateErr != nil {
				result.Failed++
				logx.Warnf("[feed] translate sources group=%d failed: %v", groupID, translateErr)
				continue
			}
			deliveryItems = translated
		}
		if ctx.Err() != nil {
			return result
		}
		if err := sender.SendGroupText(groupID, formatPendingNotification(deliveryItems, setting.ItemMaxChars)); err != nil {
			result.Failed++
			logx.Warnf("[feed] deliver group=%d failed: %v", groupID, err)
			continue
		}
		ids := make([]uint, 0, len(pending))
		for _, item := range pending {
			ids = append(ids, item.ID)
		}
		if err := m.store.DeleteFeedPendingItems(groupID, ids); err != nil {
			result.Failed++
			logx.Warnf("[feed] clear outbox group=%d failed: %v", groupID, err)
			continue
		}
		result.Delivered += len(pending)
	}
	for _, groupID := range uniqueGroupIDs(deliveryGroups) {
		if ctx.Err() != nil {
			return result
		}
		count, err := m.store.CountFeedPendingItems(groupID)
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

func feedFailureBackoff(failures int) time.Duration {
	if failures < 1 {
		failures = 1
	}
	delay := 10 * time.Minute
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
