package db

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestOneShotReminderPersistenceAndCompletion(t *testing.T) {
	initPostgresForTest(t)

	runAt := time.Now().Add(30 * time.Minute).Truncate(time.Second)
	row, err := AddOneShotReminder(1, 100, runAt, "起来走动")
	if err != nil {
		t.Fatal(err)
	}
	if row.Recurring || row.RunAt != runAt.Unix() || row.CronExpr != "" {
		t.Fatalf("one-shot reminder = %+v", row)
	}
	active, err := GetActiveReminders()
	if err != nil || len(active) != 1 || active[0].ID != row.ID {
		t.Fatalf("active reminders = %+v, err=%v", active, err)
	}
	if err := CompleteReminder(row.ID); err != nil {
		t.Fatal(err)
	}
	if active, _ := GetActiveReminders(); len(active) != 0 {
		t.Fatalf("active after completion = %+v", active)
	}
}

func TestRecurringReminderKeepsRecurringFlag(t *testing.T) {
	initPostgresForTest(t)

	row, err := AddReminder(1, 100, "30 9 * * *", "喝水")
	if err != nil {
		t.Fatal(err)
	}
	if !row.Recurring || row.RunAt != 0 {
		t.Fatalf("recurring reminder = %+v", row)
	}
}

func TestDeleteReminderRequiresMatchingUserAndGroup(t *testing.T) {
	initPostgresForTest(t)

	row, err := AddReminder(1, 100, "30 9 * * *", "喝水")
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteReminder(row.ID, 1, 200); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("wrong-group delete error = %v", err)
	}
	if rows, _ := GetUserReminders(1, 100); len(rows) != 1 {
		t.Fatalf("reminder deleted from wrong group: %+v", rows)
	}
	if err := DeleteReminder(row.ID, 1, 100); err != nil {
		t.Fatal(err)
	}
}
