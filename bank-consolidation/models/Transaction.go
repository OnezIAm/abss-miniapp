package models

import (
	"time"

	"gorm.io/gorm"
)

type Transaction struct {
	ID               string         `json:"id" gorm:"primaryKey;type:varchar(64)"`
	RawCSV           string         `json:"rawCsv" gorm:"column:raw_csv;type:longtext;not null"`
	ImportSource     string         `json:"importSource" gorm:"type:varchar(255)"`
	ValidationStatus string         `json:"validationStatus" gorm:"type:varchar(32);not null"`
	ImportTimestamp  time.Time      `json:"importTimestamp" gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}
