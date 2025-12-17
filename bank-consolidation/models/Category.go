package models

import "gorm.io/gorm"

type Category struct {
	ID             string         `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Type           string         `json:"type" gorm:"type:varchar(32);not null"`
	Name           string         `json:"name" gorm:"type:varchar(255);not null"`
	DefaultAccount string         `json:"defaultAccount" gorm:"type:varchar(255)"`
	BusinessRules  string         `json:"businessRules" gorm:"type:text"`
	TaxRules       string         `json:"taxRules" gorm:"type:text"`
	BudgetRef      string         `json:"budgetRef" gorm:"type:varchar(255)"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}
