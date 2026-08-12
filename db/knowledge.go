package db

import (
	"time"

	"gorm.io/gorm"
)

// GroupKnowledge is one group-scoped source used for grounded Q&A.
type GroupKnowledge struct {
	ID        uint   `gorm:"primarykey;autoIncrement" json:"id"`
	GroupID   int64  `gorm:"index" json:"group_id"`
	Title     string `gorm:"size:80" json:"title"`
	Content   string `gorm:"type:text" json:"content"`
	SourceURL string `gorm:"size:1024" json:"source_url"`
	CreatedBy int64  `json:"created_by"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func CreateGroupKnowledge(groupID, createdBy int64, title, content, sourceURL string) (*GroupKnowledge, error) {
	now := time.Now().Unix()
	row := &GroupKnowledge{
		GroupID: groupID, CreatedBy: createdBy, Title: title, Content: content,
		SourceURL: sourceURL, CreatedAt: now, UpdatedAt: now,
	}
	return row, DB.Create(row).Error
}

func CountGroupKnowledge(groupID int64) (int64, error) {
	var count int64
	err := DB.Model(&GroupKnowledge{}).Where("group_id = ?", groupID).Count(&count).Error
	return count, err
}

func ListGroupKnowledge(groupID int64) ([]GroupKnowledge, error) {
	var rows []GroupKnowledge
	err := DB.Where("group_id = ?", groupID).Order("updated_at desc, id desc").Find(&rows).Error
	return rows, err
}

// SearchGroupKnowledge uses zhparser for both document and query tokenisation.
// Query terms are ORed so natural questions can retrieve a document even when
// only their informative terms overlap; title lexemes carry the higher rank.
func SearchGroupKnowledge(groupID int64, question string, limit int) ([]GroupKnowledge, error) {
	if limit <= 0 || limit > 8 {
		limit = 5
	}
	var rows []GroupKnowledge
	err := DB.Raw(`
		WITH parsed AS (
			SELECT tsvector_to_array(to_tsvector('public.chinese_zhparser', ?)) AS terms
		), query AS (
			SELECT CASE
				WHEN cardinality(terms) = 0 THEN NULL
				ELSE to_tsquery('public.chinese_zhparser', array_to_string(terms, ' | '))
			END AS value
			FROM parsed
		)
		SELECT knowledge.id, knowledge.group_id, knowledge.title, knowledge.content,
		       knowledge.source_url, knowledge.created_by, knowledge.created_at, knowledge.updated_at
		FROM group_knowledges AS knowledge
		CROSS JOIN query
		WHERE knowledge.group_id = ? AND query.value IS NOT NULL
		  AND knowledge.search_vector @@ query.value
		ORDER BY ts_rank_cd(knowledge.search_vector, query.value) DESC,
		         knowledge.updated_at DESC, knowledge.id DESC
		LIMIT ?`, question, groupID, limit).Scan(&rows).Error
	return rows, err
}

func DeleteGroupKnowledge(id uint, groupID int64) error {
	result := DB.Where("id = ? AND group_id = ?", id, groupID).Delete(&GroupKnowledge{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
