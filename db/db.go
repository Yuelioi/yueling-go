package db

import (
	"fmt"
	"time"

	"github.com/Yuelioi/yueling-go/util"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

type AutoReply struct {
	ID      uint   `gorm:"primarykey;autoIncrement"`
	QQ      int64  `gorm:"not null"`
	Keyword string `gorm:"size:128"`
	Reply   string `gorm:"size:1024"`
	Group   string `gorm:"size:128"`
}

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

// Reminder is a persistent scheduled reminder.
type Reminder struct {
	ID       uint  `gorm:"primarykey;autoIncrement"`
	UserID   int64 `gorm:"index"`
	GroupID  int64
	CronExpr string `gorm:"size:32"` // "30 14 * * *"
	Message  string `gorm:"size:256"`
	Active   bool   `gorm:"default:true"`
}

func AddReminder(userID, groupID int64, cronExpr, message string) (*Reminder, error) {
	r := &Reminder{UserID: userID, GroupID: groupID, CronExpr: cronExpr, Message: message, Active: true}
	err := DB.Create(r).Error
	return r, err
}

func DeleteReminder(id uint, userID int64) error {
	return DB.Where("id = ? AND user_id = ?", id, userID).Delete(&Reminder{}).Error
}

func GetUserReminders(userID, groupID int64) ([]Reminder, error) {
	var rows []Reminder
	err := DB.Where("user_id = ? AND group_id = ? AND active = ?", userID, groupID, true).Find(&rows).Error
	return rows, err
}

func GetActiveReminders() ([]Reminder, error) {
	var rows []Reminder
	err := DB.Where("active = ?", true).Find(&rows).Error
	return rows, err
}

func CountUserReminders(userID, groupID int64) (int64, error) {
	var count int64
	err := DB.Model(&Reminder{}).Where("user_id = ? AND group_id = ? AND active = ?", userID, groupID, true).Count(&count).Error
	return count, err
}

// ── User tag ─────────────────────────────────────────────────────────────────

type UserTag struct {
	ID     uint   `gorm:"primarykey;autoIncrement"`
	UserID int64  `gorm:"uniqueIndex:idx_user_tag"`
	Tag    string `gorm:"size:64;uniqueIndex:idx_user_tag"`
}

func AddUserTag(userID int64, tag string) error {
	return DB.FirstOrCreate(&UserTag{}, UserTag{UserID: userID, Tag: tag}).Error
}

func DeleteUserTag(userID int64, tag string) error {
	return DB.Where("user_id = ? AND tag = ?", userID, tag).Delete(&UserTag{}).Error
}

func GetUserTags(userID int64) ([]UserTag, error) {
	var rows []UserTag
	err := DB.Where("user_id = ?", userID).Find(&rows).Error
	return rows, err
}

// ── User profile (key-value) ──────────────────────────────────────────────────

type UserProfile struct {
	ID     uint   `gorm:"primarykey;autoIncrement"`
	UserID int64  `gorm:"uniqueIndex:idx_user_profile"`
	Key    string `gorm:"size:32;uniqueIndex:idx_user_profile"`
	Value  string `gorm:"size:256"`
}

func SetUserProfile(userID int64, key, value string) error {
	return DB.Where(UserProfile{UserID: userID, Key: key}).
		Assign(UserProfile{Value: value}).
		FirstOrCreate(&UserProfile{}).Error
}

func GetUserProfile(userID int64, key string) (string, bool) {
	var p UserProfile
	err := DB.Where("user_id = ? AND key = ?", userID, key).First(&p).Error
	if err != nil {
		return "", false
	}
	return p.Value, true
}

func GetAllUserProfile(userID int64) (map[string]string, error) {
	var rows []UserProfile
	if err := DB.Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.Key] = r.Value
	}
	return m, nil
}

func DeleteUserProfile(userID int64, key string) error {
	return DB.Where("user_id = ? AND key = ?", userID, key).Delete(&UserProfile{}).Error
}

// ── Todo ──────────────────────────────────────────────────────────────────────

type TodoItem struct {
	ID        uint  `gorm:"primarykey;autoIncrement"`
	UserID    int64 `gorm:"index"`
	GroupID   int64
	Content   string `gorm:"size:256"`
	Done      bool   `gorm:"default:false"`
	CreatedAt float64
}

func AddTodo(userID, groupID int64, content string) error {
	var count int64
	DB.Model(&TodoItem{}).Where("user_id = ? AND done = ?", userID, false).Count(&count)
	if count >= 20 {
		return fmt.Errorf("待办太多了，先完成一些吧（上限20条）")
	}
	return DB.Create(&TodoItem{UserID: userID, GroupID: groupID, Content: content, CreatedAt: float64(time.Now().Unix())}).Error
}

func GetTodos(userID int64) ([]TodoItem, error) {
	var rows []TodoItem
	err := DB.Where("user_id = ? AND done = ?", userID, false).Order("created_at asc").Find(&rows).Error
	return rows, err
}

func DoneTodo(id uint) error {
	return DB.Model(&TodoItem{}).Where("id = ?", id).Update("done", true).Error
}

func DeleteTodo(id uint) error {
	return DB.Delete(&TodoItem{}, id).Error
}

// ── Memory models ────────────────────────────────────────────────────────────

type SemanticMemory struct {
	ID           uint    `gorm:"primarykey;autoIncrement"`
	UserID       int64   `gorm:"index"`
	Content      string  `gorm:"type:text"`
	Category     string  `gorm:"size:32"`
	Score        float64 `gorm:"default:1.0"`
	CreatedAt    float64
	LastAccessed float64
}

type EpisodicMemory struct {
	ID            uint  `gorm:"primarykey;autoIncrement"`
	UserID        int64 `gorm:"index"`
	GroupID       int64
	InputText     string `gorm:"type:text"`
	ToolName      string `gorm:"size:64"`
	ResultSummary string `gorm:"type:text"`
	Steps         int
	CreatedAt     float64
}

type ProceduralMemory struct {
	ID        uint   `gorm:"primarykey;autoIncrement"`
	GroupID   int64  `gorm:"index"`
	Rule      string `gorm:"type:text"`
	Priority  int    `gorm:"default:0"`
	CreatedBy int64
	CreatedAt float64
}

var allModels = []any{
	&AutoReply{}, &UserGameRecord{}, &Reminder{},
	&SemanticMemory{}, &EpisodicMemory{}, &ProceduralMemory{},
	&UserTag{}, &TodoItem{}, &UserProfile{},
}

func Init(path string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}
	// Only create tables that don't exist yet — never alter existing ones.
	// glebarez/sqlite's AutoMigrate has a DDL-parsing bug when recreating tables
	// with NOT NULL / UNIQUE constraints, which corrupts the migration.
	m := DB.Migrator()
	for _, model := range allModels {
		if !m.HasTable(model) {
			if err := m.CreateTable(model); err != nil {
				return fmt.Errorf("create table %T: %w", model, err)
			}
		}
	}
	// ADD COLUMN is safe in SQLite and avoids the AutoMigrate DDL bug.
	addColumnIfMissing("user_game_records", "check_in_month", "VARCHAR(7) DEFAULT ''")
	addColumnIfMissing("user_game_records", "monthly_check_in", "INTEGER DEFAULT 0")
	return nil
}

func addColumnIfMissing(table, column, definition string) {
	var count int64
	DB.Raw("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", table, column).Scan(&count)
	if count == 0 {
		DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	}
}

func getOrCreateInTx(tx *gorm.DB, userID, groupID int64, nickname string) (*UserGameRecord, error) {
	var r UserGameRecord
	if err := tx.Where(UserGameRecord{UserID: userID, GroupID: groupID}).FirstOrCreate(&r).Error; err != nil {
		return nil, err
	}
	if nickname != "" && r.Nickname != nickname {
		r.Nickname = nickname
	}
	return &r, nil
}

// GetOrCreateGameRecord fetches or initialises a user's game record.
func GetOrCreateGameRecord(userID, groupID int64, nickname string) (*UserGameRecord, error) {
	return getOrCreateInTx(DB, userID, groupID, nickname)
}

// CheckIn processes today's check-in inside a transaction to prevent double check-ins.
// Returns (points gained, current streak, monthly count, already done today, error).
func CheckIn(userID, groupID int64, nickname string) (int64, int, int, bool, error) {
	now := util.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	thisMonth := now.Format("2006-01")

	var gained int64
	var streak, monthly int
	alreadyDone := false

	err := DB.Transaction(func(tx *gorm.DB) error {
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

// UpdatePKResult records a PK outcome inside a transaction: winner gains 5, loser loses 2 (floor 0).
// Returns (winner score, loser score, error).
func UpdatePKResult(winnerID, loserID, groupID int64, winnerNick, loserNick string) (int64, int64, error) {
	var winnerScore, loserScore int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		winner, err := getOrCreateInTx(tx, winnerID, groupID, winnerNick)
		if err != nil {
			return err
		}
		loser, err := getOrCreateInTx(tx, loserID, groupID, loserNick)
		if err != nil {
			return err
		}
		winner.Score += 5
		winner.WinCount++
		if loser.Score >= 2 {
			loser.Score -= 2
		} else {
			loser.Score = 0
		}
		loser.LoseCount++
		if err := tx.Save(winner).Error; err != nil {
			return err
		}
		if err := tx.Save(loser).Error; err != nil {
			return err
		}
		winnerScore = winner.Score
		loserScore = loser.Score
		return nil
	})
	return winnerScore, loserScore, err
}

// GetTopScores returns the top n records for a group, sorted by score descending.
func GetTopScores(groupID int64, n int) ([]UserGameRecord, error) {
	var rows []UserGameRecord
	err := DB.Where("group_id = ?", groupID).
		Order("score desc").
		Limit(n).
		Find(&rows).Error
	return rows, err
}

func AddReply(qq int64, keyword, reply, group string) error {
	return DB.Create(&AutoReply{QQ: qq, Keyword: keyword, Reply: reply, Group: group}).Error
}

func DeleteReply(id uint) error {
	return DB.Delete(&AutoReply{}, id).Error
}

func GetAllReplies() ([]AutoReply, error) {
	var rows []AutoReply
	err := DB.Find(&rows).Error
	return rows, err
}
