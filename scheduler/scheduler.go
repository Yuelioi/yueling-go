package scheduler

import (
	"fmt"
	"sync"
	"time"

	cronlib "github.com/robfig/cron/v3"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/services/logx"
)

var (
	mu   sync.Mutex
	cr   *cronlib.Cron
	jobs = map[uint]cronlib.EntryID{}
	tz   *time.Location
)

func loadTZ() *time.Location {
	name := config.C.Bot.Timezone
	if name == "" {
		name = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		logx.Warnf("[scheduler] invalid timezone %q, falling back to Asia/Shanghai: %v", name, err)
		loc, _ = time.LoadLocation("Asia/Shanghai")
	}
	return loc
}

// Init (re)starts the scheduler with the given API. Safe to call on reconnect.
func Init(api *bot.BotAPI) {
	mu.Lock()
	defer mu.Unlock()

	tz = loadTZ()
	if cr != nil {
		cr.Stop()
	}
	cr = cronlib.New(cronlib.WithLocation(tz))
	jobs = map[uint]cronlib.EntryID{}

	reminders, err := db.GetActiveReminders()
	if err != nil {
		logx.Errorf("[scheduler] failed to load reminders: %v", err)
	}
	for _, r := range reminders {
		addJob(api, r)
	}
	cr.Start()
	logx.Infof("[scheduler] started, %d reminder(s) loaded", len(reminders))
}

func addJob(api *bot.BotAPI, r db.Reminder) {
	rid := r.ID
	groupID := r.GroupID
	userID := r.UserID
	message := r.Message

	entryID, err := cr.AddFunc(r.CronExpr, func() {
		msg := bot.Msg().At(userID).Text(" " + message).Build()
		if _, err := api.SendGroupMsg(groupID, msg); err != nil {
			logx.Errorf("[scheduler] send reminder %d failed: %v", rid, err)
		}
	})
	if err != nil {
		logx.Warnf("[scheduler] invalid cron expr for reminder %d: %v", rid, err)
		return
	}
	jobs[rid] = entryID
}

// Add schedules a new reminder and persists it.
func Add(api *bot.BotAPI, userID, groupID int64, cronExpr, message string) (*db.Reminder, error) {
	mu.Lock()
	defer mu.Unlock()

	r, err := db.AddReminder(userID, groupID, cronExpr, message)
	if err != nil {
		return nil, err
	}
	if cr != nil {
		addJob(api, *r)
	}
	return r, nil
}

// Remove cancels and deletes a reminder.
func Remove(reminderID uint, userID int64) error {
	mu.Lock()
	defer mu.Unlock()

	if err := db.DeleteReminder(reminderID, userID); err != nil {
		return err
	}
	if cr != nil {
		if entryID, ok := jobs[reminderID]; ok {
			cr.Remove(entryID)
			delete(jobs, reminderID)
		}
	}
	return nil
}

// ParseTime converts "HH:MM" to a daily cron expression (CST).
func ParseTime(hhmm string) (string, error) {
	t, err := time.ParseInLocation("15:04", hhmm, tz)
	if err != nil {
		return "", fmt.Errorf("时间格式错误，请使用 HH:MM（如 09:30）")
	}
	return fmt.Sprintf("%d %d * * *", t.Minute(), t.Hour()), nil
}

// AfterMinutes returns a one-shot cron expression that fires once, n minutes from now.
// The returned cron expression uses a specific date so it only fires once.
func AfterMinutes(n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("时间必须大于0")
	}
	t := time.Now().In(tz).Add(time.Duration(n) * time.Minute)
	// "分 时 日 月 *" — specific date fires once then never again (cron won't match next year)
	return fmt.Sprintf("%d %d %d %d *", t.Minute(), t.Hour(), t.Day(), int(t.Month())), nil
}
