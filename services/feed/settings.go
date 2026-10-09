package feed

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
)

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

	setting, err := m.store.GetFeedGroupSetting(groupID)
	if err != nil {
		return db.FeedGroupSetting{}, err
	}
	return m.setDeliverySettings(groupID, enabled, start, end, setting.ItemMaxChars)
}

func (m *Manager) SetDeliverySettings(groupID int64, enabled bool, start, end string, itemMaxChars int, pollMinutes ...int) (db.FeedGroupSetting, error) {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	return m.setDeliverySettings(groupID, enabled, start, end, itemMaxChars, pollMinutes...)
}

func (m *Manager) setDeliverySettings(groupID int64, enabled bool, start, end string, itemMaxChars int, pollMinutes ...int) (db.FeedGroupSetting, error) {
	if err := validateItemMaxChars(itemMaxChars); err != nil {
		return db.FeedGroupSetting{}, err
	}
	if !enabled && (strings.TrimSpace(start) == "" || strings.TrimSpace(end) == "") {
		setting, err := m.store.GetFeedGroupSetting(groupID)
		if err != nil {
			return db.FeedGroupSetting{}, err
		}
		start, end = setting.QuietStart, setting.QuietEnd
	}
	start, end, err := ParseQuietHours(start, end)
	if err != nil {
		return db.FeedGroupSetting{}, err
	}
	return m.store.SetFeedGroupSetting(groupID, enabled, start, end, itemMaxChars, pollMinutes...)
}

func validateItemMaxChars(value int) error {
	if value != 0 && (value < MinItemMaxChars || value > MaxItemMaxChars) {
		return fmt.Errorf("订阅内容长度应为 0（完整）或 %d–%d 字", MinItemMaxChars, MaxItemMaxChars)
	}
	return nil
}

func (m *Manager) DeliveryStatus(groupID int64) (db.FeedGroupSetting, int64, error) {
	setting, err := m.store.GetFeedGroupSetting(groupID)
	if err != nil {
		return db.FeedGroupSetting{}, 0, err
	}
	pending, err := m.store.CountFeedPendingItems(groupID)
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
	locationName := strings.TrimSpace(config.Current().Bot.Timezone)
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

func (m *Manager) PendingCount(groupID int64) (int64, error) {
	return m.store.CountFeedPendingItems(groupID)
}
