package controllers

import (
	"bank-consolidation/models"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type TransactionController struct{ DB *sql.DB }

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
		_, err = c.DB.Exec(`INSERT INTO transactions (id, raw_csv, import_source, validation_status) VALUES (?, ?, ?, 'pending')`, body.ID, string(b), body.ImportSource)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": body.ID})
	case http.MethodGet:
		rows, err := c.DB.Query(`SELECT id, import_source, validation_status, import_timestamp FROM transactions WHERE deleted_at IS NULL ORDER BY import_timestamp DESC LIMIT 100`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var list []models.Transaction
		for rows.Next() {
			var id, vs, ts string
			var src sql.NullString
			if err := rows.Scan(&id, &src, &vs, &ts); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			t := models.Transaction{ID: id, ValidationStatus: vs, ImportTimestamp: ts}
			if src.Valid {
				t.ImportSource = src.String
			}
			list = append(list, t)
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
	tx, err := c.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, cid := range body.CategoryIDs {
		if _, err := tx.Exec(`INSERT IGNORE INTO transaction_categories (transaction_id, category_id) VALUES (?, ?)`, id, cid); err != nil {
			_ = tx.Rollback()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "transactionId": id, "count": len(body.CategoryIDs)})
}
