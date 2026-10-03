package db

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

func LoadBilibiliCheckpoint(accountID int64) ([]byte, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var row struct{ Payload string }
	err := DB.Table("bilibili_checkpoints").Where("account_id = ?", accountID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return []byte(row.Payload), err
}

func SaveBilibiliCheckpoint(accountID int64, payload []byte) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return DB.Exec(`INSERT INTO bilibili_checkpoints (account_id, payload) VALUES (?, ?::jsonb)
		ON CONFLICT (account_id) DO UPDATE SET payload = EXCLUDED.payload, updated_at = now()`, accountID, string(payload)).Error
}
