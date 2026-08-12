package db

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FeedSubscription stores one RSS/Atom source followed by a group.
type FeedSubscription struct {
	ID         uint   `gorm:"primarykey;autoIncrement" json:"id"`
	GroupID    int64  `gorm:"uniqueIndex:idx_feed_group_url" json:"group_id"`
	URL        string `gorm:"size:1024;uniqueIndex:idx_feed_group_url" json:"url"`
	Name       string `gorm:"size:64" json:"name"`
	LastItemID string `gorm:"size:64" json:"-"`
	CreatedBy  int64  `json:"created_by"`
	Enabled    bool   `gorm:"default:true" json:"enabled"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`

	ConsecutiveFailures int    `gorm:"not null;default:0" json:"consecutive_failures"`
	LastError           string `gorm:"size:512" json:"last_error"`
	LastCheckedAt       int64  `gorm:"not null;default:0" json:"last_checked_at"`
	LastSuccessAt       int64  `gorm:"not null;default:0" json:"last_success_at"`
	NextCheckAt         int64  `gorm:"not null;default:0;index:idx_feed_due" json:"next_check_at"`
}

// FeedGroupSetting controls delivery for one group. Fetching continues during
// quiet hours; only delivery is delayed, so cursors and source health stay fresh.
type FeedGroupSetting struct {
	GroupID      int64  `gorm:"primaryKey;autoIncrement:false" json:"group_id"`
	QuietEnabled bool   `gorm:"not null;default:false" json:"quiet_enabled"`
	QuietStart   string `gorm:"size:5;not null;default:'23:00'" json:"quiet_start"`
	QuietEnd     string `gorm:"size:5;not null;default:'08:00'" json:"quiet_end"`
	UpdatedAt    int64  `gorm:"not null;default:0" json:"updated_at"`
}

// FeedPendingItem is a durable outbox. A feed cursor only advances in the same
// transaction that stores these rows, preventing updates from being lost when
// QQ delivery fails or the bot restarts.
type FeedPendingItem struct {
	ID             uint   `gorm:"primarykey;autoIncrement" json:"id"`
	SubscriptionID uint   `gorm:"not null;uniqueIndex:idx_feed_pending_item" json:"subscription_id"`
	GroupID        int64  `gorm:"not null;index:idx_feed_pending_group" json:"group_id"`
	FeedName       string `gorm:"size:64;not null" json:"feed_name"`
	ItemKey        string `gorm:"size:64;not null;uniqueIndex:idx_feed_pending_item" json:"item_key"`
	Title          string `gorm:"size:160;not null" json:"title"`
	Link           string `gorm:"size:512" json:"link"`
	PublishedAt    int64  `gorm:"not null;default:0" json:"published_at"`
	QueuedAt       int64  `gorm:"not null;index:idx_feed_pending_group" json:"queued_at"`
}

func CreateFeedSubscription(groupID, createdBy int64, rawURL, name, lastItemID string) (*FeedSubscription, error) {
	now := time.Now().Unix()
	row := &FeedSubscription{
		GroupID: groupID, CreatedBy: createdBy, URL: rawURL, Name: name,
		LastItemID: lastItemID, Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	return row, DB.Create(row).Error
}

func CountFeedSubscriptions(groupID int64) (int64, error) {
	var count int64
	err := DB.Model(&FeedSubscription{}).Where("group_id = ?", groupID).Count(&count).Error
	return count, err
}

func ListFeedSubscriptions(groupID int64) ([]FeedSubscription, error) {
	var rows []FeedSubscription
	err := DB.Where("group_id = ?", groupID).Order("id asc").Find(&rows).Error
	return rows, err
}

func ListAllFeedSubscriptions() ([]FeedSubscription, error) {
	var rows []FeedSubscription
	err := DB.Order("id asc").Find(&rows).Error
	return rows, err
}

func ListActiveFeedSubscriptions(groupID int64) ([]FeedSubscription, error) {
	var rows []FeedSubscription
	err := DB.Where("group_id = ? AND enabled = ?", groupID, true).Order("id asc").Find(&rows).Error
	return rows, err
}

func ListDueFeedSubscriptions(now int64) ([]FeedSubscription, error) {
	var rows []FeedSubscription
	err := DB.Where("enabled = ? AND (next_check_at = 0 OR next_check_at <= ?)", true, now).
		Order("id asc").Find(&rows).Error
	return rows, err
}

func DeleteFeedSubscription(id uint, groupID int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var row FeedSubscription
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND group_id = ?", id, groupID).First(&row).Error; err != nil {
			return err
		}
		if err := tx.Where("subscription_id = ?", id).Delete(&FeedPendingItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&row).Error
	})
}

func SetFeedSubscriptionEnabled(id uint, groupID int64, enabled bool) (*FeedSubscription, error) {
	var row FeedSubscription
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND group_id = ?", id, groupID).First(&row).Error; err != nil {
			return err
		}
		now := time.Now().Unix()
		updates := map[string]any{"enabled": enabled, "updated_at": now}
		if enabled {
			// Make a resumed source eligible for the next automatic poll immediately.
			updates["next_check_at"] = 0
		} else if err := tx.Where("subscription_id = ?", id).Delete(&FeedPendingItem{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&row).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).First(&row).Error
	})
	return &row, err
}

func UpdateFeedSubscriptionCursor(id uint, lastItemID string) error {
	return DB.Model(&FeedSubscription{}).Where("id = ?", id).Updates(map[string]any{
		"last_item_id": lastItemID,
		"updated_at":   time.Now().Unix(),
	}).Error
}

// RecordFeedFetchSuccess atomically persists newly discovered items and moves
// the source cursor. Items should be ordered from oldest to newest.
func RecordFeedFetchSuccess(id uint, lastItemID string, checkedAt, nextCheckAt int64, items []FeedPendingItem) (int64, error) {
	var inserted int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		var row FeedSubscription
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "enabled").First(&row, id).Error; err != nil {
			return err
		}
		// A pause that wins the row lock makes an in-flight fetch a no-op. This
		// keeps the outbox empty once pause has returned, even across processes.
		if !row.Enabled {
			return nil
		}
		if len(items) > 0 {
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(items, 100)
			if result.Error != nil {
				return result.Error
			}
			inserted = result.RowsAffected
		}
		return tx.Model(&FeedSubscription{}).Where("id = ?", id).Updates(map[string]any{
			"last_item_id":         lastItemID,
			"consecutive_failures": 0,
			"last_error":           "",
			"last_checked_at":      checkedAt,
			"last_success_at":      checkedAt,
			"next_check_at":        nextCheckAt,
			"updated_at":           checkedAt,
		}).Error
	})
	return inserted, err
}

func RecordFeedFetchFailure(id uint, failures int, message string, checkedAt, nextCheckAt int64) error {
	return DB.Model(&FeedSubscription{}).Where("id = ? AND enabled = ?", id, true).Updates(map[string]any{
		"consecutive_failures": failures,
		"last_error":           message,
		"last_checked_at":      checkedAt,
		"next_check_at":        nextCheckAt,
		"updated_at":           checkedAt,
	}).Error
}

func GetFeedGroupSetting(groupID int64) (FeedGroupSetting, error) {
	var setting FeedGroupSetting
	err := DB.Where("group_id = ?", groupID).First(&setting).Error
	if err == nil {
		return setting, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return FeedGroupSetting{GroupID: groupID, QuietStart: "23:00", QuietEnd: "08:00"}, nil
	}
	return FeedGroupSetting{}, err
}

func SetFeedGroupQuietHours(groupID int64, enabled bool, start, end string) (FeedGroupSetting, error) {
	now := time.Now().Unix()
	setting := FeedGroupSetting{
		GroupID: groupID, QuietEnabled: enabled, QuietStart: start, QuietEnd: end, UpdatedAt: now,
	}
	err := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "group_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"quiet_enabled", "quiet_start", "quiet_end", "updated_at",
		}),
	}).Create(&setting).Error
	return setting, err
}

func ListFeedPendingItems(groupID int64, limit int) ([]FeedPendingItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var rows []FeedPendingItem
	err := DB.Where("group_id = ?", groupID).Order("queued_at asc, id asc").Limit(limit).Find(&rows).Error
	return rows, err
}

func CountFeedPendingItems(groupID int64) (int64, error) {
	var count int64
	err := DB.Model(&FeedPendingItem{}).Where("group_id = ?", groupID).Count(&count).Error
	return count, err
}

func ListFeedPendingGroupIDs() ([]int64, error) {
	var groupIDs []int64
	err := DB.Model(&FeedPendingItem{}).Distinct().Order("group_id asc").Pluck("group_id", &groupIDs).Error
	return groupIDs, err
}

func DeleteFeedPendingItems(groupID int64, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return DB.Where("group_id = ? AND id IN ?", groupID, ids).Delete(&FeedPendingItem{}).Error
}

func DeleteFeedPendingItemsForGroup(groupID int64) error {
	return DB.Where("group_id = ?", groupID).Delete(&FeedPendingItem{}).Error
}
