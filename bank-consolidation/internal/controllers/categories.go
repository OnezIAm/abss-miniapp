package controllers

import (
	"bank-consolidation/models"
	"encoding/json"
	"net/http"

	"gorm.io/gorm"
)

type CategoryController struct{ DB *gorm.DB }

func (c CategoryController) CreateOrList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var body struct {
			ID             string `json:"id"`
			Type           string `json:"type"`
			Name           string `json:"name"`
			DefaultAccount string `json:"defaultAccount"`
			BusinessRules  any    `json:"businessRules"`
			TaxRules       any    `json:"taxRules"`
			BudgetRef      string `json:"budgetRef"`
		}
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body.ID == "" || body.Type == "" || body.Name == "" {
			http.Error(w, "id, type, name are required", http.StatusBadRequest)
			return
		}
		if body.Type != "money_in" && body.Type != "money_out" {
			http.Error(w, "type must be money_in or money_out", http.StatusBadRequest)
			return
		}

		br, _ := json.Marshal(body.BusinessRules)
		tr, _ := json.Marshal(body.TaxRules)

		category := models.Category{
			ID:             body.ID,
			Type:           body.Type,
			Name:           body.Name,
			DefaultAccount: body.DefaultAccount,
			BusinessRules:  string(br),
			TaxRules:       string(tr),
			BudgetRef:      body.BudgetRef,
		}

		if err := c.DB.Create(&category).Error; err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": body.ID})
	case http.MethodGet:
		var categories []models.Category
		if err := c.DB.Select("id", "type", "name", "default_account").Order("name").Find(&categories).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(categories)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
