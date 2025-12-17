package controllers

import (
	"bank-consolidation/models"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TransactionController struct{ DB *gorm.DB }

func (c TransactionController) CreateOrList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var body models.Transaction
		if err := json.Unmarshal(b, &body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body.ID == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}

		t := models.Transaction{
			ID:               body.ID,
			RawCSV:           string(b),
			ImportSource:     body.ImportSource,
			ValidationStatus: "pending",
		}

		if err := c.DB.Create(&t).Error; err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": body.ID})
	case http.MethodGet:
		var list []models.Transaction
		if err := c.DB.Select("id", "import_source", "validation_status", "import_timestamp").
			Order("import_timestamp DESC").
			Limit(100).
			Find(&list).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c TransactionController) MapCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Path
	if !strings.HasSuffix(path, "/categories") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	id := strings.TrimSuffix(path[len("/transactions/"):], "/categories")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var body struct {
		CategoryIDs []string `json:"categoryIds"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := c.DB.Transaction(func(tx *gorm.DB) error {
		for _, cid := range body.CategoryIDs {
			tc := models.TransactionCategory{TransactionID: id, CategoryID: cid}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&tc).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "transactionId": id, "count": len(body.CategoryIDs)})
}
