package db

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProfileStore struct{ database *gorm.DB }

func NewProfileStore(database *gorm.DB) *ProfileStore { return &ProfileStore{database: database} }

type UserProfile struct {
	ID     uint   `gorm:"primarykey;autoIncrement"`
	UserID int64  `gorm:"uniqueIndex:idx_user_profile"`
	Key    string `gorm:"size:32;uniqueIndex:idx_user_profile"`
	Value  string `gorm:"size:256"`
}

func (s *ProfileStore) SetUserProfile(userID int64, key, value string) error {
	return s.database.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"value"})}).Create(&UserProfile{UserID: userID, Key: key, Value: value}).Error
}

func (s *ProfileStore) GetUserProfile(userID int64, key string) (string, bool) {
	var p UserProfile
	err := s.database.Where("user_id = ? AND key = ?", userID, key).First(&p).Error
	if err != nil {
		return "", false
	}
	return p.Value, true
}

func (s *ProfileStore) GetAllUserProfile(userID int64) (map[string]string, error) {
	var rows []UserProfile
	if err := s.database.Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.Key] = r.Value
	}
	return m, nil
}

func (s *ProfileStore) DeleteUserProfile(userID int64, key string) error {
	return s.database.Where("user_id = ? AND key = ?", userID, key).Delete(&UserProfile{}).Error
}

func (s *ProfileStore) ClearUserProfiles(userID int64) (int64, error) {
	result := s.database.Where("user_id = ?", userID).Delete(&UserProfile{})
	return result.RowsAffected, result.Error
}
