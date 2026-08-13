package db

import (
	"errors"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/util"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MaxCommandUsageNameRunes = 128
	MaxCommandUsageDays      = 90
)

// GroupCommandUsage stores one aggregate row per day, group, user, plugin and
// command. Keeping counters instead of raw invocation logs makes long-running
// installations cheap while retaining group, user and trend statistics.
type GroupCommandUsage struct {
	ID         uint   `gorm:"primarykey;autoIncrement" json:"id"`
	UsageDate  string `gorm:"size:10;uniqueIndex:idx_group_command_usage_daily" json:"usage_date"`
	GroupID    int64  `gorm:"uniqueIndex:idx_group_command_usage_daily;index:idx_group_command_usage_group_date" json:"group_id"`
	UserID     int64  `gorm:"uniqueIndex:idx_group_command_usage_daily" json:"user_id"`
	PluginID   int    `gorm:"uniqueIndex:idx_group_command_usage_daily" json:"plugin_id"`
	Command    string `gorm:"size:128;uniqueIndex:idx_group_command_usage_daily" json:"command"`
	Count      int64  `gorm:"default:1" json:"count"`
	LastUsedAt int64  `gorm:"index" json:"last_used_at"`
}

type CommandUsageRank struct {
	Command     string `json:"command"`
	PluginID    int    `json:"plugin_id"`
	Calls       int64  `json:"calls"`
	UniqueUsers int64  `json:"unique_users"`
	LastUsedAt  int64  `json:"last_used_at"`
}

type CommandUsageDay struct {
	Date        string `json:"date"`
	Calls       int64  `json:"calls"`
	UniqueUsers int64  `json:"unique_users"`
}

type GroupCommandUsageStats struct {
	GroupID        int64              `json:"group_id"`
	Days           int                `json:"days"`
	TotalCalls     int64              `json:"total_calls"`
	UniqueUsers    int64              `json:"unique_users"`
	ActiveCommands int64              `json:"active_commands"`
	TopCommands    []CommandUsageRank `json:"top_commands"`
	Daily          []CommandUsageDay  `json:"daily"`
}

func normalizeCommandUsageName(command string) string {
	command = strings.TrimSpace(command)
	runes := []rune(command)
	if len(runes) > MaxCommandUsageNameRunes {
		command = string(runes[:MaxCommandUsageNameRunes])
	}
	return command
}

// RecordCommandUsage records an invocation using the bot's configured local
// date (Asia/Shanghai by default).
func RecordCommandUsage(groupID, userID int64, pluginID int, command string) error {
	return RecordCommandUsageAt(groupID, userID, pluginID, command, util.Now())
}

// RecordCommandUsageAt is exported so importers and tests can preserve the
// invocation time when replaying usage data.
func RecordCommandUsageAt(groupID, userID int64, pluginID int, command string, at time.Time) error {
	command = normalizeCommandUsageName(command)
	if groupID <= 0 || userID <= 0 || pluginID < 0 || command == "" {
		return errors.New("invalid command usage")
	}
	local := at.In(util.Now().Location())
	row := GroupCommandUsage{
		UsageDate:  local.Format("2006-01-02"),
		GroupID:    groupID,
		UserID:     userID,
		PluginID:   pluginID,
		Command:    command,
		Count:      1,
		LastUsedAt: at.Unix(),
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "usage_date"},
			{Name: "group_id"},
			{Name: "user_id"},
			{Name: "plugin_id"},
			{Name: "command"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"count":        gorm.Expr("group_command_usages.count + 1"),
			"last_used_at": row.LastUsedAt,
		}),
	}).Create(&row).Error
}

func GetGroupCommandUsageStats(groupID int64, days int, now time.Time) (*GroupCommandUsageStats, error) {
	if groupID <= 0 || days <= 0 || days > MaxCommandUsageDays {
		return nil, errors.New("invalid command usage query")
	}
	localNow := now.In(util.Now().Location())
	start := localNow.AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	end := localNow.Format("2006-01-02")
	usageQuery := func() *gorm.DB {
		return DB.Model(&GroupCommandUsage{}).
			Where("group_id = ? AND usage_date BETWEEN ? AND ?", groupID, start, end)
	}

	var totals struct {
		TotalCalls     int64
		UniqueUsers    int64
		ActiveCommands int64
	}
	if err := usageQuery().Select(
		"COALESCE(SUM(count), 0) AS total_calls, COUNT(DISTINCT user_id) AS unique_users, COUNT(DISTINCT command) AS active_commands",
	).Scan(&totals).Error; err != nil {
		return nil, err
	}

	var top []CommandUsageRank
	if err := usageQuery().Select(
		"command, plugin_id, SUM(count) AS calls, COUNT(DISTINCT user_id) AS unique_users, MAX(last_used_at) AS last_used_at",
	).Group("command, plugin_id").
		Order("calls DESC, last_used_at DESC, command ASC").
		Limit(20).
		Scan(&top).Error; err != nil {
		return nil, err
	}

	var recordedDays []CommandUsageDay
	if err := usageQuery().Select(
		"usage_date AS date, SUM(count) AS calls, COUNT(DISTINCT user_id) AS unique_users",
	).Group("usage_date").Order("usage_date ASC").Scan(&recordedDays).Error; err != nil {
		return nil, err
	}
	byDate := make(map[string]CommandUsageDay, len(recordedDays))
	for _, day := range recordedDays {
		byDate[day.Date] = day
	}
	daily := make([]CommandUsageDay, 0, days)
	for offset := days - 1; offset >= 0; offset-- {
		date := localNow.AddDate(0, 0, -offset).Format("2006-01-02")
		day := byDate[date]
		day.Date = date
		daily = append(daily, day)
	}

	if top == nil {
		top = []CommandUsageRank{}
	}
	return &GroupCommandUsageStats{
		GroupID:        groupID,
		Days:           days,
		TotalCalls:     totals.TotalCalls,
		UniqueUsers:    totals.UniqueUsers,
		ActiveCommands: totals.ActiveCommands,
		TopCommands:    top,
		Daily:          daily,
	}, nil
}
