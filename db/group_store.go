package db

import "gorm.io/gorm"

type GroupStore struct{ database *gorm.DB }

func NewGroupStore(database *gorm.DB) *GroupStore { return &GroupStore{database: database} }
func (s *GroupStore) Available() bool             { return s != nil && s.database != nil }

type OverviewStore struct{ database *gorm.DB }

func NewOverviewStore(database *gorm.DB) *OverviewStore { return &OverviewStore{database: database} }
