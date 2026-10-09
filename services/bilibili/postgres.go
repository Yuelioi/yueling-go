package bilibili

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type PostgresStore struct{ database *gorm.DB }

func NewPostgresStore(database *gorm.DB) PostgresStore { return PostgresStore{database: database} }

func (s PostgresStore) Load(accountID int64) ([]byte, error) {
	if s.database == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var row struct{ Payload string }
	err := s.database.Table("bilibili_checkpoints").Where("account_id = ?", accountID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return []byte(row.Payload), err
}

func (s PostgresStore) Save(accountID int64, payload []byte) error {
	if s.database == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return s.database.Exec(`INSERT INTO bilibili_checkpoints (account_id, payload) VALUES (?, ?::jsonb)
		ON CONFLICT (account_id) DO UPDATE SET payload = EXCLUDED.payload, updated_at = now()`, accountID, string(payload)).Error
}
