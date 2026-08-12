package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	cronlib "github.com/robfig/cron/v3"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/services/logx"
)

var (
	mu         sync.Mutex
	cr         *cronlib.Cron
	jobs       = map[uint]cronlib.EntryID{}
	oneShots   = map[uint]*time.Timer{}
	oneShotSeq = map[uint]uint64{}
	nextSeq    uint64
	digestJobs = map[int64]cronlib.EntryID{}
	tz         *time.Location
)

const (
	maxOneShotDelay   = 365 * 24 * time.Hour
	oneShotRetryDelay = time.Minute
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
	for _, timer := range oneShots {
		timer.Stop()
	}
	oneShots = map[uint]*time.Timer{}
	oneShotSeq = map[uint]uint64{}
	digestJobs = map[int64]cronlib.EntryID{}

	reminders, err := db.GetActiveReminders()
	if err != nil {
		logx.Errorf("[scheduler] failed to load reminders: %v", err)
	}
	for _, r := range reminders {
		addJob(api, r)
	}
	digests, err := db.GetActiveDailyDigests()
	if err != nil {
		logx.Errorf("[scheduler] failed to load daily digests: %v", err)
	}
	for _, digest := range digests {
		addDigestJob(api, digest)
	}
	cr.Start()
	logx.Infof("[scheduler] started, %d reminder(s), %d daily digest(s) loaded", len(reminders), len(digests))
}

func addDigestJob(api *bot.BotAPI, digest db.DailyDigest) {
	groupID := digest.GroupID
	entryID, err := cr.AddFunc(digest.CronExpr, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		summary, err := ai.GenerateAndSendGroupDigest(ctx, api, groupID, digest.MessageCount)
		if err != nil {
			logx.Errorf("[scheduler] generate daily digest group=%d failed: %v", groupID, err)
			return
		}
		if summary == "" {
			return
		}
	})
	if err != nil {
		logx.Warnf("[scheduler] invalid daily digest group=%d: %v", groupID, err)
		return
	}
	digestJobs[groupID] = entryID
}

func addJob(api *bot.BotAPI, r db.Reminder) {
	if !r.Recurring && r.RunAt > 0 {
		addOneShotJob(api, r)
		return
	}
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

func addOneShotJob(api *bot.BotAPI, reminder db.Reminder) {
	runAt := time.Unix(reminder.RunAt, 0)
	delay := time.Until(runAt)
	if delay < 0 {
		delay = 0
	}
	scheduleOneShotAttempt(api, reminder, delay)
}

// scheduleOneShotAttempt must be called with mu held. The sequence token keeps
// a callback from an old Bot connection from replacing a timer created by Init.
func scheduleOneShotAttempt(api *bot.BotAPI, reminder db.Reminder, delay time.Duration) {
	nextSeq++
	seq := nextSeq
	oneShotSeq[reminder.ID] = seq
	oneShots[reminder.ID] = time.AfterFunc(delay, func() {
		msg := bot.Msg().At(reminder.UserID).Text(" " + reminder.Message).Build()
		if _, err := api.SendGroupMsg(reminder.GroupID, msg); err != nil {
			logx.Errorf("[scheduler] send one-shot reminder %d failed: %v", reminder.ID, err)
			mu.Lock()
			if oneShotSeq[reminder.ID] == seq {
				scheduleOneShotAttempt(api, reminder, oneShotRetryDelay)
			}
			mu.Unlock()
			return
		}
		markOneShotComplete(reminder.ID, seq)
	})
}

func markOneShotComplete(reminderID uint, seq uint64) {
	if err := db.CompleteReminder(reminderID); err != nil {
		logx.Errorf("[scheduler] complete one-shot reminder %d failed: %v", reminderID, err)
		mu.Lock()
		if oneShotSeq[reminderID] == seq {
			oneShots[reminderID] = time.AfterFunc(oneShotRetryDelay, func() {
				markOneShotComplete(reminderID, seq)
			})
		}
		mu.Unlock()
		return
	}

	mu.Lock()
	if timer, ok := oneShots[reminderID]; ok {
		timer.Stop()
	}
	delete(oneShots, reminderID)
	delete(oneShotSeq, reminderID)
	mu.Unlock()
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

func AddAfter(api *bot.BotAPI, userID, groupID int64, delay time.Duration, message string) (*db.Reminder, error) {
	if delay <= 0 {
		return nil, fmt.Errorf("时间必须大于0")
	}
	if delay > maxOneShotDelay {
		return nil, fmt.Errorf("一次性提醒最长可设置一年后")
	}
	mu.Lock()
	defer mu.Unlock()
	r, err := db.AddOneShotReminder(userID, groupID, time.Now().Add(delay), message)
	if err != nil {
		return nil, err
	}
	addOneShotJob(api, *r)
	return r, nil
}

// Remove cancels and deletes a reminder.
func Remove(reminderID uint, userID, groupID int64) error {
	mu.Lock()
	defer mu.Unlock()

	if err := db.DeleteReminder(reminderID, userID, groupID); err != nil {
		return err
	}
	if cr != nil {
		if entryID, ok := jobs[reminderID]; ok {
			cr.Remove(entryID)
			delete(jobs, reminderID)
		}
	}
	if timer, ok := oneShots[reminderID]; ok {
		timer.Stop()
		delete(oneShots, reminderID)
		delete(oneShotSeq, reminderID)
	}
	return nil
}

// DescribeReminder returns a concise schedule label for user-facing lists.
func DescribeReminder(reminder db.Reminder) string {
	if !reminder.Recurring && reminder.RunAt > 0 {
		return time.Unix(reminder.RunAt, 0).In(loadTZ()).Format("2006-01-02 15:04")
	}
	fields := strings.Fields(reminder.CronExpr)
	if len(fields) == 5 && fields[2] == "*" && fields[3] == "*" && fields[4] == "*" {
		minute, minuteErr := strconv.Atoi(fields[0])
		hour, hourErr := strconv.Atoi(fields[1])
		if minuteErr == nil && hourErr == nil && minute >= 0 && minute < 60 && hour >= 0 && hour < 24 {
			return fmt.Sprintf("每天 %02d:%02d", hour, minute)
		}
	}
	return "定时提醒"
}

func parseTimeAt(hhmm string, location *time.Location) (string, error) {
	t, err := time.ParseInLocation("15:04", hhmm, location)
	if err != nil {
		return "", fmt.Errorf("时间格式错误，请使用 HH:MM（如 09:30）")
	}
	return fmt.Sprintf("%d %d * * *", t.Minute(), t.Hour()), nil
}

// ParseTime converts "HH:MM" to a daily cron expression in the configured timezone.
func ParseTime(hhmm string) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	if tz == nil {
		tz = loadTZ()
	}
	return parseTimeAt(hhmm, tz)
}

func SetDailyDigest(api *bot.BotAPI, groupID, createdBy int64, hhmm string, messageCount int) (*db.DailyDigest, error) {
	mu.Lock()
	defer mu.Unlock()
	if tz == nil {
		tz = loadTZ()
	}
	cronExpr, err := parseTimeAt(hhmm, tz)
	if err != nil {
		return nil, err
	}
	if messageCount < 10 || messageCount > 100 {
		messageCount = 80
	}
	row, err := db.UpsertDailyDigest(groupID, createdBy, hhmm, cronExpr, messageCount)
	if err != nil {
		return nil, err
	}
	if cr != nil {
		if entryID, ok := digestJobs[groupID]; ok {
			cr.Remove(entryID)
		}
		addDigestJob(api, *row)
	}
	return row, nil
}

func RemoveDailyDigest(groupID int64) error {
	mu.Lock()
	defer mu.Unlock()
	if err := db.DeleteDailyDigest(groupID); err != nil {
		return err
	}
	if cr != nil {
		if entryID, ok := digestJobs[groupID]; ok {
			cr.Remove(entryID)
			delete(digestJobs, groupID)
		}
	}
	return nil
}
