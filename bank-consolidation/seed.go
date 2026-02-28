package main

import (
	"bank-consolidation/models"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func seedDefaultBanks(db *gorm.DB) error {
	banks := []models.BankType{
		{
			ID:          fmt.Sprintf("BT-%d-BCA", time.Now().UnixNano()),
			Name:        "BCA",
			Code:        "BCA",
			Description: "Bank Central Asia",
		},
		{
			ID:          fmt.Sprintf("BT-%d-DANAMON", time.Now().UnixNano()),
			Name:        "Danamon",
			Code:        "DANAMON",
			Description: "Bank Danamon",
		},
	}

	for _, bank := range banks {
		// Use Upsert on Code
		err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "description"}),
		}).Create(&bank).Error
		if err != nil {
			return err
		}
	}
	return nil
}
