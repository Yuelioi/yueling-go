package db

import (
	"github.com/Yuelioi/yueling-go/util"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserGameRecord stores per-user per-group score, PK stats, and check-in state.
type UserGameRecord struct {
	ID             uint   `gorm:"primarykey;autoIncrement"`
	UserID         int64  `gorm:"uniqueIndex:idx_ug"`
	GroupID        int64  `gorm:"uniqueIndex:idx_ug"`
	Nickname       string `gorm:"size:64"`
	Score          int64  `gorm:"default:0"`
	WinCount       int    `gorm:"default:0"`
	LoseCount      int    `gorm:"default:0"`
	CheckInDate    string `gorm:"size:10"` // last check-in YYYY-MM-DD (Asia/Shanghai)
	Streak         int    `gorm:"default:0"`
	CheckInMonth   string `gorm:"size:7"` // current month YYYY-MM
	MonthlyCheckIn int    `gorm:"default:0"`
}

func getOrCreateInTx(tx *gorm.DB, userID, groupID int64, nickname string) (*UserGameRecord, error) {
	row := UserGameRecord{UserID: userID, GroupID: groupID, Nickname: nickname}
	if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "group_id"}}, DoNothing: true}).Create(&row).Error; err != nil {
		return nil, err
	}
	var r UserGameRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND group_id = ?", userID, groupID).First(&r).Error; err != nil {
		return nil, err
	}
	if nickname != "" && r.Nickname != nickname {
		if err := tx.Model(&r).Update("nickname", nickname).Error; err != nil {
			return nil, err
		}
		r.Nickname = nickname
	}
	return &r, nil
}

// GetOrCreateGameRecord fetches or initialises a user's game record.
func (s *GameStore) GetOrCreateGameRecord(userID, groupID int64, nickname string) (*UserGameRecord, error) {
	var row *UserGameRecord
	err := s.database.Transaction(func(tx *gorm.DB) error {
		var err error
		row, err = getOrCreateInTx(tx, userID, groupID, nickname)
		return err
	})
	return row, err
}

// CheckIn processes today's check-in inside a transaction to prevent double check-ins.
// Returns (points gained, current streak, monthly count, already done today, error).
func (s *GameStore) CheckIn(userID, groupID int64, nickname string) (int64, int, int, bool, error) {
	now := util.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	thisMonth := now.Format("2006-01")

	var gained int64
	var streak, monthly int
	alreadyDone := false

	err := s.database.Transaction(func(tx *gorm.DB) error {
		r, err := getOrCreateInTx(tx, userID, groupID, nickname)
		if err != nil {
			return err
		}
		if r.CheckInDate == today {
			alreadyDone = true
			streak = r.Streak
			monthly = r.MonthlyCheckIn
			return nil
		}
		if r.CheckInDate == yesterday {
			r.Streak++
		} else {
			r.Streak = 1
		}
		bonus := min(int64(r.Streak-1), 7)
		gained = int64(10) + bonus
		r.Score += gained
		r.CheckInDate = today
		streak = r.Streak
		if r.CheckInMonth == thisMonth {
			r.MonthlyCheckIn++
		} else {
			r.CheckInMonth = thisMonth
			r.MonthlyCheckIn = 1
		}
		monthly = r.MonthlyCheckIn
		return tx.Save(r).Error
	})
	return gained, streak, monthly, alreadyDone, err
}

// GetTopScores returns the top n records for a group, sorted by score descending.
func (s *GameStore) GetTopScores(groupID int64, n int) ([]UserGameRecord, error) {
	var rows []UserGameRecord
	err := s.database.Where("group_id = ?", groupID).
		Order("score desc").
		Limit(n).
		Find(&rows).Error
	return rows, err
}

type GameStore struct{ database *gorm.DB }

func NewGameStore(database *gorm.DB) *GameStore { return &GameStore{database: database} }
