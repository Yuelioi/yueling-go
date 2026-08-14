package db

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommandUsageAggregatesByGroupDayAndUser(t *testing.T) {
	oldDB := DB
	if err := initSQLiteForTest(filepath.Join(t.TempDir(), "command-usage.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { DB = oldDB })

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	day13 := time.Date(2026, 8, 13, 9, 0, 0, 0, loc)
	day14 := time.Date(2026, 8, 14, 20, 0, 0, 0, loc)
	records := []struct {
		groupID  int64
		userID   int64
		pluginID int
		command  string
		at       time.Time
	}{
		{100, 1, 42, "ping", day13},
		{100, 1, 42, "ping", day13.Add(time.Minute)},
		{100, 1, 9, "签到", day14},
		{100, 2, 42, "ping", day14.Add(time.Minute)},
		{200, 3, 42, "ping", day14},
	}
	for _, record := range records {
		if err := RecordCommandUsageAt(record.groupID, record.userID, record.pluginID, record.command, record.at); err != nil {
			t.Fatal(err)
		}
	}

	stats, err := GetGroupCommandUsageStats(100, 2, day14)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalCalls != 4 || stats.UniqueUsers != 2 || stats.ActiveCommands != 2 {
		t.Fatalf("stats=%+v", stats)
	}
	if len(stats.TopCommands) != 2 || stats.TopCommands[0].Command != "ping" ||
		stats.TopCommands[0].Calls != 3 || stats.TopCommands[0].UniqueUsers != 2 {
		t.Fatalf("top commands=%+v", stats.TopCommands)
	}
	if len(stats.Daily) != 2 || stats.Daily[0].Date != "2026-08-13" ||
		stats.Daily[0].Calls != 2 || stats.Daily[1].Calls != 2 || stats.Daily[1].UniqueUsers != 2 {
		t.Fatalf("daily=%+v", stats.Daily)
	}

	allStats, err := GetGroupCommandUsageStats(0, 2, day14)
	if err != nil {
		t.Fatal(err)
	}
	if allStats.GroupID != 0 || allStats.TotalCalls != 5 || allStats.UniqueUsers != 3 ||
		allStats.ActiveCommands != 2 || len(allStats.TopCommands) != 2 ||
		allStats.TopCommands[0].Command != "ping" || allStats.TopCommands[0].Calls != 4 ||
		allStats.Daily[1].Calls != 3 {
		t.Fatalf("all-group stats=%+v", allStats)
	}
}

func TestCommandUsageNormalizesAndValidatesInput(t *testing.T) {
	oldDB := DB
	if err := initSQLiteForTest(filepath.Join(t.TempDir(), "command-usage-validation.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { DB = oldDB })

	longName := strings.Repeat("月", MaxCommandUsageNameRunes+20)
	if err := RecordCommandUsageAt(100, 1, 0, "  "+longName+"  ", time.Now()); err != nil {
		t.Fatal(err)
	}
	var row GroupCommandUsage
	if err := DB.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if len([]rune(row.Command)) != MaxCommandUsageNameRunes {
		t.Fatalf("command runes=%d", len([]rune(row.Command)))
	}

	for _, args := range []struct {
		groupID  int64
		userID   int64
		pluginID int
		command  string
	}{
		{0, 1, 1, "ping"},
		{100, 0, 1, "ping"},
		{100, 1, -1, "ping"},
		{100, 1, 1, "   "},
	} {
		if err := RecordCommandUsageAt(args.groupID, args.userID, args.pluginID, args.command, time.Now()); err == nil {
			t.Fatalf("expected invalid input error: %+v", args)
		}
	}
	if _, err := GetGroupCommandUsageStats(100, MaxCommandUsageDays+1, time.Now()); err == nil {
		t.Fatal("expected invalid day range")
	}
}
