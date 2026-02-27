package controllers

import (
	"bank-consolidation/models"
	"encoding/json"
	"net/http"

	"gorm.io/gorm"
)

type BankTypeController struct{ DB *gorm.DB }

func (c BankTypeController) CreateOrList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var body struct {
			Name        string `json:"name"`
			Code        string `json:"code"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body.Name == "" || body.Code == "" {
			http.Error(w, "name and code are required", http.StatusBadRequest)
			return
		}

		// Check if exists
		var count int64
		c.DB.Model(&models.BankType{}).Where("code = ?", body.Code).Count(&count)
		if count > 0 {
			http.Error(w, "bank type with this code already exists", http.StatusConflict)
			return
		}

		bankType := models.BankType{
			ID:          genID("BT"),
			Name:        body.Name,
			Code:        body.Code,
			Description: body.Description,
		}

		if err := c.DB.Create(&bankType).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(bankType)

	case http.MethodGet:
		var bankTypes []models.BankType
		if err := c.DB.Find(&bankTypes).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bankTypes)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
