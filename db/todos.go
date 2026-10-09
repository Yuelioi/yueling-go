package db

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TodoItem struct {
	ID        uint  `gorm:"primarykey;autoIncrement"`
	UserID    int64 `gorm:"index"`
	GroupID   int64
	Content   string `gorm:"size:256"`
	Done      bool   `gorm:"default:false"`
	CreatedAt float64
}

var ErrTodoLimit = errors.New("待办太多了，先完成一些吧（上限20条）")

type TodoStore struct{ database *gorm.DB }

func NewTodoStore(database *gorm.DB) *TodoStore { return &TodoStore{database: database} }

func (s *TodoStore) Add(userID, groupID int64, content string) error {
	return s.database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", fmt.Sprintf("todos:%d", userID)).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&TodoItem{}).Where("user_id = ? AND done = ?", userID, false).Count(&count).Error; err != nil {
			return err
		}
		if count >= 20 {
			return ErrTodoLimit
		}
		return tx.Create(&TodoItem{UserID: userID, GroupID: groupID, Content: content, CreatedAt: float64(time.Now().Unix())}).Error
	})
}

func (s *TodoStore) List(userID int64) ([]TodoItem, error) {
	var rows []TodoItem
	err := s.database.Where("user_id = ? AND done = ?", userID, false).Order("created_at asc, id asc").Find(&rows).Error
	return rows, err
}

func (s *TodoStore) Complete(userID int64, id uint) (*TodoItem, error) {
	return s.change(userID, id, false)
}
func (s *TodoStore) Delete(userID int64, id uint) (*TodoItem, error) {
	return s.change(userID, id, true)
}

func (s *TodoStore) change(userID int64, id uint, remove bool) (*TodoItem, error) {
	var row TodoItem
	err := s.database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ? AND done = ?", userID, id, false).First(&row).Error; err != nil {
			return err
		}
		if remove {
			return tx.Delete(&row).Error
		}
		return tx.Model(&row).Update("done", true).Error
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}
