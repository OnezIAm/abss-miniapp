package controllers

import (
	"bank-consolidation/models"
	"encoding/json"
	"net/http"
	"strings"

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
			Format      string `json:"format"`
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
			Format:      body.Format,
		}
		if bankType.Format == "" {
			bankType.Format = "GENERIC"
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

func (c BankTypeController) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/bank-types/")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var body struct {
		Name        string `json:"name"`
		Code        string `json:"code"`
		Description string `json:"description"`
		Format      string `json:"format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if body.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	// Check for unique constraints on name and code (excluding current record)
	var count int64
	if err := c.DB.Model(&models.BankType{}).
		Where("(name = ? OR code = ?) AND id != ?", body.Name, body.Code, id).
		Count(&count).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if count > 0 {
		http.Error(w, "Bank type with this name or code already exists", http.StatusConflict)
		return
	}

	updates := map[string]interface{}{
		"name":        body.Name,
		"code":        body.Code,
		"description": body.Description,
		"format":      body.Format,
	}

	if err := c.DB.Model(&models.BankType{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": id})
}
