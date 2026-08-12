package db

import "gorm.io/gorm/clause"

type DailyDigest struct {
	ID           uint  `gorm:"primarykey;autoIncrement"`
	GroupID      int64 `gorm:"uniqueIndex"`
	CreatedBy    int64
	SendTime     string `gorm:"size:5"`
	CronExpr     string `gorm:"size:32"`
	MessageCount int
	Enabled      bool `gorm:"default:true"`
}

func UpsertDailyDigest(groupID, createdBy int64, sendTime, cronExpr string, messageCount int) (*DailyDigest, error) {
	row := DailyDigest{
		GroupID: groupID, CreatedBy: createdBy, SendTime: sendTime,
		CronExpr: cronExpr, MessageCount: messageCount, Enabled: true,
	}
	err := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "group_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"created_by", "send_time", "cron_expr", "message_count", "enabled",
		}),
	}).Create(&row).Error
	if err != nil {
		return nil, err
	}
	if err := DB.Where("group_id = ?", groupID).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func GetDailyDigest(groupID int64) (*DailyDigest, error) {
	var row DailyDigest
	err := DB.Where("group_id = ? AND enabled = ?", groupID, true).First(&row).Error
	return &row, err
}

func GetActiveDailyDigests() ([]DailyDigest, error) {
	var rows []DailyDigest
	err := DB.Where("enabled = ?", true).Find(&rows).Error
	return rows, err
}

func DeleteDailyDigest(groupID int64) error {
	return DB.Where("group_id = ?", groupID).Delete(&DailyDigest{}).Error
}
