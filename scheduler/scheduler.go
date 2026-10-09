package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services/logx"
	cronlib "github.com/robfig/cron/v3"
)

type Manager struct {
	store      *db.ScheduleStore
	initMu     sync.Mutex
	restarting bool
	mu         sync.Mutex
	stopped    bool
	callbacks  sync.WaitGroup
	retired    []context.Context
	cr         *cronlib.Cron
	jobs       map[uint]cronlib.EntryID
	oneShots   map[uint]*time.Timer
	oneShotSeq map[uint]uint64
	nextSeq    uint64
	digestJobs map[int64]cronlib.EntryID
	tz         *time.Location
}

func New(store *db.ScheduleStore) *Manager {
	return &Manager{store: store, jobs: map[uint]cronlib.EntryID{}, oneShots: map[uint]*time.Timer{}, oneShotSeq: map[uint]uint64{}, digestJobs: map[int64]cronlib.EntryID{}}
}

const (
	maxOneShotDelay   = 365 * 24 * time.Hour
	oneShotRetryDelay = time.Minute
)

func loadTZ() *time.Location {
	name := config.Current().Bot.Timezone
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
func (m *Manager) Init(api *bot.BotAPI) {
	m.initMu.Lock()
	defer m.initMu.Unlock()
	m.mu.Lock()
	if m.stopped || api.Context().Err() != nil {
		m.mu.Unlock()
		return
	}
	m.restarting = true
	if m.cr != nil {
		m.retired = append(m.retired, m.cr.Stop())
		m.cr = nil
	}
	for _, timer := range m.oneShots {
		timer.Stop()
	}
	m.oneShots = map[uint]*time.Timer{}
	m.oneShotSeq = map[uint]uint64{}
	pending := append([]context.Context(nil), m.retired...)
	m.mu.Unlock()
	// A delivered reminder must finish persisting before a reconnect reloads it.
	for _, done := range pending {
		<-done.Done()
	}
	m.callbacks.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.restarting = false
	if m.stopped || api.Context().Err() != nil {
		return
	}
	m.retired = nil
	m.tz = loadTZ()
	m.cr = cronlib.New(cronlib.WithLocation(m.tz))
	m.jobs = map[uint]cronlib.EntryID{}
	m.digestJobs = map[int64]cronlib.EntryID{}

	reminders, err := m.store.GetActiveReminders()
	if err != nil {
		logx.Errorf("[scheduler] failed to load reminders: %v", err)
	}
	for _, r := range reminders {
		m.addJob(api, r)
	}
	digests, err := m.store.GetActiveDailyDigests()
	if err != nil {
		logx.Errorf("[scheduler] failed to load daily digests: %v", err)
	}
	for _, digest := range digests {
		m.addDigestJob(api, digest)
	}
	m.cr.Start()
	logx.Infof("[scheduler] started, %d reminder(s), %d daily digest(s) loaded", len(reminders), len(digests))
}

func (m *Manager) addDigestJob(api *bot.BotAPI, digest db.DailyDigest) {
	groupID := digest.GroupID
	entryID, err := m.cr.AddFunc(digest.CronExpr, func() {
		ctx, cancel := context.WithTimeout(api.Context(), 45*time.Second)
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
	m.digestJobs[groupID] = entryID
}

func (m *Manager) addJob(api *bot.BotAPI, r db.Reminder) {
	if !r.Recurring && r.RunAt > 0 {
		m.addOneShotJob(api, r)
		return
	}
	rid := r.ID
	groupID := r.GroupID
	userID := r.UserID
	message := r.Message

	entryID, err := m.cr.AddFunc(r.CronExpr, func() {
		if !config.PluginEnabled(catalog.PluginReminder) {
			return
		}
		msg := bot.Msg().At(userID).Text(" " + message).Build()
		if _, err := api.SendGroupMsg(groupID, msg); err != nil {
			logx.Errorf("[scheduler] send reminder %d failed: %v", rid, err)
		}
	})
	if err != nil {
		logx.Warnf("[scheduler] invalid cron expr for reminder %d: %v", rid, err)
		return
	}
	m.jobs[rid] = entryID
}

func (m *Manager) addOneShotJob(api *bot.BotAPI, reminder db.Reminder) {
	runAt := time.Unix(reminder.RunAt, 0)
	delay := time.Until(runAt)
	if delay < 0 {
		delay = 0
	}
	m.scheduleOneShotAttempt(api, reminder, delay)
}

// scheduleOneShotAttempt must be called with m.mu held. The sequence token keeps
// a callback from an old Bot connection from replacing a timer created by Init.
func (m *Manager) scheduleOneShotAttempt(api *bot.BotAPI, reminder db.Reminder, delay time.Duration) {
	if m.stopped || m.restarting {
		return
	}
	m.nextSeq++
	seq := m.nextSeq
	m.oneShotSeq[reminder.ID] = seq
	m.oneShots[reminder.ID] = time.AfterFunc(delay, func() {
		if !m.beginCallback(reminder.ID, seq) {
			return
		}
		defer m.callbacks.Done()
		if !config.PluginEnabled(catalog.PluginReminder) {
			m.mu.Lock()
			if m.oneShotSeq[reminder.ID] == seq {
				m.scheduleOneShotAttempt(api, reminder, oneShotRetryDelay)
			}
			m.mu.Unlock()
			return
		}
		msg := bot.Msg().At(reminder.UserID).Text(" " + reminder.Message).Build()
		if _, err := api.SendGroupMsg(reminder.GroupID, msg); err != nil {
			logx.Errorf("[scheduler] send one-shot reminder %d failed: %v", reminder.ID, err)
			m.mu.Lock()
			if m.oneShotSeq[reminder.ID] == seq {
				m.scheduleOneShotAttempt(api, reminder, oneShotRetryDelay)
			}
			m.mu.Unlock()
			return
		}
		m.markOneShotComplete(reminder.ID, seq)
	})
}

func (m *Manager) markOneShotComplete(reminderID uint, seq uint64) {
	if err := m.store.CompleteReminder(reminderID); err != nil {
		logx.Errorf("[scheduler] complete one-shot reminder %d failed: %v", reminderID, err)
		m.mu.Lock()
		if m.oneShotSeq[reminderID] == seq {
			m.oneShots[reminderID] = time.AfterFunc(oneShotRetryDelay, func() {
				if !m.beginCallback(reminderID, seq) {
					return
				}
				defer m.callbacks.Done()
				m.markOneShotComplete(reminderID, seq)
			})
		}
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	if m.oneShotSeq[reminderID] != seq {
		m.mu.Unlock()
		return
	}
	if timer, ok := m.oneShots[reminderID]; ok {
		timer.Stop()
	}
	delete(m.oneShots, reminderID)
	delete(m.oneShotSeq, reminderID)
	m.mu.Unlock()
}

// Add schedules a new reminder and persists it.
func (m *Manager) Add(api *bot.BotAPI, userID, groupID int64, cronExpr, message string) (*db.Reminder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, err := m.store.AddReminder(userID, groupID, cronExpr, message)
	if err != nil {
		return nil, err
	}
	if m.cr != nil && api != nil {
		m.addJob(api, *r)
	}
	return r, nil
}

func (m *Manager) AddAfter(api *bot.BotAPI, userID, groupID int64, delay time.Duration, message string) (*db.Reminder, error) {
	if delay <= 0 {
		return nil, fmt.Errorf("时间必须大于0")
	}
	if delay > maxOneShotDelay {
		return nil, fmt.Errorf("一次性提醒最长可设置一年后")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.addAtLocked(api, userID, groupID, time.Now().Add(delay), message)
}

// AddAt schedules an absolute one-shot reminder. The caller should pass a time
// with an explicit timezone (for example an RFC3339 timestamp).
func (m *Manager) AddAt(api *bot.BotAPI, userID, groupID int64, runAt time.Time, message string) (*db.Reminder, error) {
	delay := time.Until(runAt)
	if delay <= 0 {
		return nil, fmt.Errorf("提醒时间必须晚于现在")
	}
	if delay > maxOneShotDelay {
		return nil, fmt.Errorf("一次性提醒最长可设置一年后")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.addAtLocked(api, userID, groupID, runAt, message)
}

func (m *Manager) addAtLocked(api *bot.BotAPI, userID, groupID int64, runAt time.Time, message string) (*db.Reminder, error) {
	r, err := m.store.AddOneShotReminder(userID, groupID, runAt, message)
	if err != nil {
		return nil, err
	}
	if api != nil {
		m.addOneShotJob(api, *r)
	}
	return r, nil
}

// Update replaces a user's reminder schedule and/or content, preserving its ID.
func (m *Manager) Update(api *bot.BotAPI, reminderID uint, userID, groupID int64, cronExpr, message string, runAt time.Time, recurring bool) (*db.Reminder, error) {
	if recurring {
		if _, err := cronlib.ParseStandard(cronExpr); err != nil {
			return nil, fmt.Errorf("无效的重复时间: %w", err)
		}
	} else {
		delay := time.Until(runAt)
		if delay <= 0 || delay > maxOneShotDelay {
			return nil, fmt.Errorf("提醒时间必须在未来一年内")
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	runAtUnix := int64(0)
	if !recurring {
		runAtUnix = runAt.Unix()
	}
	row, err := m.store.UpdateReminder(reminderID, userID, groupID, cronExpr, message, runAtUnix, recurring)
	if err != nil {
		return nil, err
	}
	m.removeJobLocked(reminderID)
	if api != nil {
		if row.Recurring {
			if m.cr != nil {
				m.addJob(api, *row)
			}
		} else {
			m.addJob(api, *row)
		}
	}
	return row, nil
}

// Remove cancels and deletes a reminder.
func (m *Manager) Remove(reminderID uint, userID, groupID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.store.DeleteReminder(reminderID, userID, groupID); err != nil {
		return err
	}
	m.removeJobLocked(reminderID)
	return nil
}

func (m *Manager) removeJobLocked(reminderID uint) {
	if m.cr != nil {
		if entryID, ok := m.jobs[reminderID]; ok {
			m.cr.Remove(entryID)
			delete(m.jobs, reminderID)
		}
	}
	if timer, ok := m.oneShots[reminderID]; ok {
		timer.Stop()
		delete(m.oneShots, reminderID)
		delete(m.oneShotSeq, reminderID)
	}
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
	if len(fields) == 5 && fields[2] == "*" && fields[3] == "*" {
		minute, minuteErr := strconv.Atoi(fields[0])
		hour, hourErr := strconv.Atoi(fields[1])
		if minuteErr == nil && hourErr == nil && minute >= 0 && minute < 60 && hour >= 0 && hour < 24 {
			switch fields[4] {
			case "1-5":
				return fmt.Sprintf("工作日 %02d:%02d", hour, minute)
			default:
				if days, ok := describeWeekdays(fields[4]); ok {
					return fmt.Sprintf("每周%s %02d:%02d", days, hour, minute)
				}
			}
		}
	}
	return "定时提醒"
}

func describeWeekdays(field string) (string, bool) {
	names := map[string]string{"0": "日", "1": "一", "2": "二", "3": "三", "4": "四", "5": "五", "6": "六"}
	parts := strings.Split(field, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		name, ok := names[part]
		if !ok {
			return "", false
		}
		out = append(out, name)
	}
	return strings.Join(out, "、"), len(out) > 0
}

func parseTimeAt(hhmm string, location *time.Location) (string, error) {
	t, err := time.ParseInLocation("15:04", hhmm, location)
	if err != nil {
		return "", fmt.Errorf("时间格式错误，请使用 HH:MM（如 09:30）")
	}
	return fmt.Sprintf("%d %d * * *", t.Minute(), t.Hour()), nil
}

// ParseTime converts "HH:MM" to a daily cron expression in the configured timezone.
func (m *Manager) ParseTime(hhmm string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tz == nil {
		m.tz = loadTZ()
	}
	return parseTimeAt(hhmm, m.tz)
}

// ParseRecurring converts a human recurrence into the standard five-field cron
// syntax used by the persisted scheduler. Weekday values use 0=Sunday..6=Saturday.
func (m *Manager) ParseRecurring(hhmm, repeat string, weekdays []int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tz == nil {
		m.tz = loadTZ()
	}
	base, err := parseTimeAt(hhmm, m.tz)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(base)
	switch repeat {
	case "daily":
		return base, nil
	case "workdays":
		fields[4] = "1-5"
	case "weekly":
		if len(weekdays) == 0 {
			return "", fmt.Errorf("每周提醒需要指定星期")
		}
		seen := map[int64]bool{}
		parts := make([]string, 0, len(weekdays))
		for _, day := range weekdays {
			if day < 0 || day > 6 {
				return "", fmt.Errorf("星期必须是0到6（0代表星期日）")
			}
			if !seen[day] {
				seen[day] = true
				parts = append(parts, strconv.FormatInt(day, 10))
			}
		}
		fields[4] = strings.Join(parts, ",")
	default:
		return "", fmt.Errorf("重复方式只支持 daily、workdays 或 weekly")
	}
	return strings.Join(fields, " "), nil
}

func (m *Manager) SetDailyDigest(api *bot.BotAPI, groupID, createdBy int64, hhmm string, messageCount int) (*db.DailyDigest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tz == nil {
		m.tz = loadTZ()
	}
	cronExpr, err := parseTimeAt(hhmm, m.tz)
	if err != nil {
		return nil, err
	}
	if messageCount < 10 || messageCount > 100 {
		messageCount = 80
	}
	row, err := m.store.UpsertDailyDigest(groupID, createdBy, hhmm, cronExpr, messageCount)
	if err != nil {
		return nil, err
	}
	if m.cr != nil {
		if entryID, ok := m.digestJobs[groupID]; ok {
			m.cr.Remove(entryID)
		}
		m.addDigestJob(api, *row)
	}
	return row, nil
}

func (m *Manager) RemoveDailyDigest(groupID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.store.DeleteDailyDigest(groupID); err != nil {
		return err
	}
	if m.cr != nil {
		if entryID, ok := m.digestJobs[groupID]; ok {
			m.cr.Remove(entryID)
			delete(m.digestJobs, groupID)
		}
	}
	return nil
}

func (m *Manager) beginCallback(id uint, seq uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped || m.restarting || m.oneShotSeq[id] != seq {
		return false
	}
	m.callbacks.Add(1)
	return true
}

// Cancel closes timer admission before Stop waits for already-running m.callbacks.
func (m *Manager) Cancel() {
	m.mu.Lock()
	m.stopped = true
	if m.cr != nil {
		m.retired = append(m.retired, m.cr.Stop())
		m.cr = nil
	}
	for _, timer := range m.oneShots {
		timer.Stop()
	}
	m.oneShots = map[uint]*time.Timer{}
	m.oneShotSeq = map[uint]uint64{}
	m.mu.Unlock()
}

func (m *Manager) Stop() {
	m.Cancel()
	m.mu.Lock()
	pending := append([]context.Context(nil), m.retired...)
	m.mu.Unlock()
	for _, done := range pending {
		<-done.Done()
	}
	m.callbacks.Wait()
}

func (m *Manager) List(userID, groupID int64) ([]db.Reminder, error) {
	return m.store.GetUserReminders(userID, groupID)
}
func (m *Manager) Count(userID, groupID int64) (int64, error) {
	return m.store.CountUserReminders(userID, groupID)
}
func (m *Manager) Get(id uint, userID, groupID int64) (*db.Reminder, error) {
	return m.store.GetReminder(id, userID, groupID)
}
func (m *Manager) Digest(groupID int64) (*db.DailyDigest, error) {
	return m.store.GetDailyDigest(groupID)
}
func (m *Manager) Digests() ([]db.DailyDigest, error) { return m.store.GetActiveDailyDigests() }
