package models

import "gorm.io/gorm"

type BankType struct {
	ID          string         `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Name        string         `json:"name" gorm:"type:varchar(100);not null;uniqueIndex"`
	Code        string         `json:"code" gorm:"type:varchar(20);not null;uniqueIndex"`
	Description string         `json:"description" gorm:"type:text"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}
