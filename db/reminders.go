package db

import (
	"time"

	"gorm.io/gorm"
)

// Reminder is a persistent scheduled reminder.
type Reminder struct {
	ID        uint  `gorm:"primarykey;autoIncrement"`
	UserID    int64 `gorm:"index"`
	GroupID   int64
	CronExpr  string `gorm:"size:32"` // recurring reminders only
	Message   string `gorm:"size:256"`
	RunAt     int64  `gorm:"default:0"` // Unix timestamp; >0 means one-shot
	Recurring bool
	Active    bool `gorm:"default:true"`
}

func (s *ScheduleStore) AddReminder(userID, groupID int64, cronExpr, message string) (*Reminder, error) {
	r := &Reminder{UserID: userID, GroupID: groupID, CronExpr: cronExpr, Message: message, Recurring: true, Active: true}
	err := s.database.Create(r).Error
	return r, err
}

func (s *ScheduleStore) AddOneShotReminder(userID, groupID int64, runAt time.Time, message string) (*Reminder, error) {
	r := &Reminder{UserID: userID, GroupID: groupID, Message: message, RunAt: runAt.Unix(), Recurring: false, Active: true}
	err := s.database.Create(r).Error
	return r, err
}

func (s *ScheduleStore) DeleteReminder(id uint, userID, groupID int64) error {
	result := s.database.Where("id = ? AND user_id = ? AND group_id = ?", id, userID, groupID).Delete(&Reminder{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *ScheduleStore) GetUserReminders(userID, groupID int64) ([]Reminder, error) {
	var rows []Reminder
	err := s.database.Where("user_id = ? AND group_id = ? AND active = ?", userID, groupID, true).Find(&rows).Error
	return rows, err
}

func (s *ScheduleStore) GetActiveReminders() ([]Reminder, error) {
	var rows []Reminder
	err := s.database.Where("active = ?", true).Find(&rows).Error
	return rows, err
}

func (s *ScheduleStore) CountUserReminders(userID, groupID int64) (int64, error) {
	var count int64
	err := s.database.Model(&Reminder{}).Where("user_id = ? AND group_id = ? AND active = ?", userID, groupID, true).Count(&count).Error
	return count, err
}

func (s *ScheduleStore) CompleteReminder(id uint) error {
	return s.database.Model(&Reminder{}).Where("id = ?", id).Update("active", false).Error
}

func (s *ScheduleStore) GetReminder(id uint, userID, groupID int64) (*Reminder, error) {
	var row Reminder
	err := s.database.Where("id = ? AND user_id = ? AND group_id = ? AND active = ?", id, userID, groupID, true).First(&row).Error
	return &row, err
}

func (s *ScheduleStore) UpdateReminder(id uint, userID, groupID int64, cronExpr, message string, runAt int64, recurring bool) (*Reminder, error) {
	result := s.database.Model(&Reminder{}).
		Where("id = ? AND user_id = ? AND group_id = ? AND active = ?", id, userID, groupID, true).
		Updates(map[string]any{
			"cron_expr": cronExpr,
			"message":   message,
			"run_at":    runAt,
			"recurring": recurring,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return s.GetReminder(id, userID, groupID)
}
