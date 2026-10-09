package runtimeconfig

import (
	"fmt"

	"gorm.io/gorm"
)

type PostgresStore struct{ database *gorm.DB }

func NewPostgresStore(database *gorm.DB) PostgresStore { return PostgresStore{database: database} }

func (s PostgresStore) Load() ([]byte, int64, error) {
	if s.database == nil {
		return nil, 0, fmt.Errorf("数据库未初始化")
	}
	var row struct {
		Payload  string
		Revision int64
	}
	result := s.database.Table("runtime_configuration").Where("id = 1").Find(&row)
	return []byte(row.Payload), row.Revision, result.Error
}

func (s PostgresStore) Seed(payload []byte) error {
	return s.database.Exec(`INSERT INTO runtime_configuration (id,payload) VALUES (1,?::jsonb) ON CONFLICT (id) DO NOTHING`, string(payload)).Error
}

func (s PostgresStore) Save(payload []byte, revision int64) error {
	r := s.database.Exec(`UPDATE runtime_configuration SET payload=?::jsonb, revision=revision+1, updated_at=now() WHERE id=1 AND revision=?`, string(payload), revision)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return ErrConflict
	}
	return nil
}
