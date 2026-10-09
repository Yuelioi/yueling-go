package db

import "gorm.io/gorm"

type ScheduleStore struct{ database *gorm.DB }

func NewScheduleStore(database *gorm.DB) *ScheduleStore { return &ScheduleStore{database: database} }
