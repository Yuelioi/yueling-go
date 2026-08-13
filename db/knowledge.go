package db

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SharedKnowledgeGroupID is the database scope used by knowledge available to
// every group. Positive IDs remain private to that group.
const SharedKnowledgeGroupID int64 = 0

// GroupKnowledge is one group- or shared-scope source used for grounded Q&A.
type GroupKnowledge struct {
	ID        uint                     `gorm:"primarykey;autoIncrement" json:"id"`
	GroupID   int64                    `gorm:"index" json:"group_id"`
	Title     string                   `gorm:"size:80" json:"title"`
	Content   string                   `gorm:"type:text" json:"content"`
	SourceURL string                   `gorm:"size:1024" json:"source_url"`
	CreatedBy int64                    `json:"created_by"`
	CreatedAt int64                    `json:"created_at"`
	UpdatedAt int64                    `json:"updated_at"`
	Shortcuts []GroupKnowledgeShortcut `gorm:"foreignKey:KnowledgeID;constraint:OnDelete:CASCADE" json:"shortcuts"`
}

// GroupKnowledgeShortcut turns a group knowledge entry into an exact, zero-AI reply.
type GroupKnowledgeShortcut struct {
	ID          uint   `gorm:"primarykey;autoIncrement" json:"id"`
	KnowledgeID uint   `gorm:"index;not null" json:"knowledge_id"`
	GroupID     int64  `gorm:"uniqueIndex:idx_group_knowledge_shortcut;not null" json:"group_id"`
	Trigger     string `gorm:"size:128;uniqueIndex:idx_group_knowledge_shortcut;not null" json:"trigger"`
	CreatedAt   int64  `json:"created_at"`
}

var ErrKnowledgeShortcutConflict = errors.New("快捷触发词已被其他知识使用")

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
	err := DB.Preload("Shortcuts").Where("group_id = ?", groupID).Order("updated_at desc, id desc").Find(&rows).Error
	return rows, err
}

// ListAvailableGroupKnowledge returns group-private entries followed by shared
// entries. WebUI scope management uses ListGroupKnowledge instead so shared and
// private content remain visibly separate.
func ListAvailableGroupKnowledge(groupID int64) ([]GroupKnowledge, error) {
	if groupID == SharedKnowledgeGroupID {
		return ListGroupKnowledge(SharedKnowledgeGroupID)
	}
	var rows []GroupKnowledge
	err := DB.Preload("Shortcuts").
		Where("group_id IN ?", []int64{groupID, SharedKnowledgeGroupID}).
		Order("group_id DESC, updated_at DESC, id DESC").
		Find(&rows).Error
	return rows, err
}

// SetGroupKnowledgeShortcuts atomically replaces all exact triggers belonging
// to an entry. The group condition prevents cross-group mutation.
func SetGroupKnowledgeShortcuts(knowledgeID uint, groupID int64, triggers []string) ([]GroupKnowledgeShortcut, error) {
	var shortcuts []GroupKnowledgeShortcut
	err := DB.Transaction(func(tx *gorm.DB) error {
		var knowledge GroupKnowledge
		if err := tx.Where("id = ? AND group_id = ?", knowledgeID, groupID).First(&knowledge).Error; err != nil {
			return err
		}
		if err := tx.Where("knowledge_id = ?", knowledgeID).Delete(&GroupKnowledgeShortcut{}).Error; err != nil {
			return err
		}
		now := time.Now().Unix()
		for _, trigger := range triggers {
			shortcut := GroupKnowledgeShortcut{
				KnowledgeID: knowledgeID,
				GroupID:     groupID,
				Trigger:     strings.ToLower(strings.TrimSpace(trigger)),
				CreatedAt:   now,
			}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&shortcut)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrKnowledgeShortcutConflict
			}
			shortcuts = append(shortcuts, shortcut)
		}
		return nil
	})
	return shortcuts, err
}

// FindGroupKnowledgeShortcut performs an exact, case-insensitive lookup. A
// group-private shortcut overrides a shared shortcut with the same trigger.
func FindGroupKnowledgeShortcut(groupID int64, trigger string) (*GroupKnowledge, error) {
	var row GroupKnowledge
	query := DB.Model(&GroupKnowledge{}).
		Joins("JOIN group_knowledge_shortcuts AS shortcut ON shortcut.knowledge_id = group_knowledges.id").
		Where("shortcut.trigger = ?", strings.ToLower(strings.TrimSpace(trigger)))
	if groupID == SharedKnowledgeGroupID {
		query = query.Where("shortcut.group_id = ?", SharedKnowledgeGroupID)
	} else {
		query = query.Where("shortcut.group_id IN ?", []int64{groupID, SharedKnowledgeGroupID}).
			Order("shortcut.group_id DESC")
	}
	err := query.Preload("Shortcuts").First(&row).Error
	return &row, err
}

// SearchGroupKnowledge uses zhparser for both document and query tokenisation
// and searches both the current group's private scope and the shared scope.
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
		WHERE knowledge.group_id IN (?, ?) AND query.value IS NOT NULL
		  AND knowledge.search_vector @@ query.value
		ORDER BY ts_rank_cd(knowledge.search_vector, query.value) DESC,
		         (knowledge.group_id = ?) DESC,
		         knowledge.updated_at DESC, knowledge.id DESC
		LIMIT ?`, question, groupID, SharedKnowledgeGroupID, groupID, limit).Scan(&rows).Error
	return rows, err
}

func DeleteGroupKnowledge(id uint, groupID int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var row GroupKnowledge
		if err := tx.Where("id = ? AND group_id = ?", id, groupID).First(&row).Error; err != nil {
			return err
		}
		if err := tx.Where("knowledge_id = ?", id).Delete(&GroupKnowledgeShortcut{}).Error; err != nil {
			return err
		}
		return tx.Delete(&row).Error
	})
}
