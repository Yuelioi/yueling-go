package scheduler

import (
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
)

func TestDescribeReminder(t *testing.T) {
	oldTimezone := config.C.Bot.Timezone
	config.C.Bot.Timezone = "Asia/Shanghai"
	t.Cleanup(func() { config.C.Bot.Timezone = oldTimezone })

	tests := []struct {
		name     string
		reminder db.Reminder
		want     string
	}{
		{
			name:     "daily",
			reminder: db.Reminder{Recurring: true, CronExpr: "5 9 * * *"},
			want:     "每天 09:05",
		},
		{
			name: "one shot",
			reminder: db.Reminder{
				RunAt: time.Date(2026, time.August, 13, 9, 5, 0, 0, time.FixedZone("CST", 8*60*60)).Unix(),
			},
			want: "2026-08-13 09:05",
		},
		{
			name:     "unknown cron",
			reminder: db.Reminder{Recurring: true, CronExpr: "*/5 * * * *"},
			want:     "定时提醒",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DescribeReminder(test.reminder); got != test.want {
				t.Fatalf("DescribeReminder() = %q, want %q", got, test.want)
			}
		})
	}
}
