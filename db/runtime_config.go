package db

import "fmt"

func LoadRuntimeConfig() ([]byte, int64, error) {
	if DB == nil {
		return nil, 0, fmt.Errorf("数据库未初始化")
	}
	var row struct {
		Payload  string
		Revision int64
	}
	result := DB.Table("runtime_configuration").Where("id = 1").Find(&row)
	return []byte(row.Payload), row.Revision, result.Error
}

func SeedRuntimeConfig(payload []byte) error {
	return DB.Exec(`INSERT INTO runtime_configuration (id,payload) VALUES (1,?::jsonb) ON CONFLICT (id) DO NOTHING`, string(payload)).Error
}

func SaveRuntimeConfig(payload []byte, revision int64) error {
	r := DB.Exec(`UPDATE runtime_configuration SET payload=?::jsonb, revision=revision+1, updated_at=now() WHERE id=1 AND revision=?`, string(payload), revision)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return fmt.Errorf("配置已被其他操作修改，请刷新后重试")
	}
	return nil
}
