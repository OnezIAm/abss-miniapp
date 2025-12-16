package controllers

import (
	"bank-consolidation/models"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

type InvoiceController struct{ DB *sql.DB }

type InvoicePayload struct {
	Header  models.InvoiceHeader   `json:"header"`
	Details []models.InvoiceDetail `json:"details"`
}

func validateInvoicePayload(p InvoicePayload) error {
	if p.Header.InvoiceHeaderID == "" {
		return errors.New("invoiceHeaderId is required")
	}
	if p.Header.InvoiceNo == "" {
		return errors.New("invoiceNo is required")
	}
	if len(p.Details) == 0 {
		return errors.New("details must not be empty")
	}
	for i, d := range p.Details {
		if d.InvoiceDetailID == "" {
			return errors.New("details[" + strconv.Itoa(i) + "].invoiceDetailId is required")
		}
		if d.ProductID == "" {
			return errors.New("details[" + strconv.Itoa(i) + "].productId is required")
		}
	}
	return nil
}

func (c InvoiceController) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var payload InvoicePayload
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateInvoicePayload(payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err := c.DB.Exec(
		`INSERT INTO invoice_headers (id, invoice_no, invoice_date, customer_id, customer_name, status, total_amount, total_tax, company_code)
         VALUES (?, ?, ?, ?, ?, 'pending', ?, 0, ?)`,
		payload.Header.InvoiceHeaderID,
		payload.Header.InvoiceNo,
		payload.Header.InvoiceDate,
		payload.Header.CustomerID,
		payload.Header.CustomerName,
		payload.Header.TotalAmount,
		payload.Header.CompanyCode,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, d := range payload.Details {
		_, err := c.DB.Exec(
			`INSERT INTO invoice_details (id, header_id, product_id, description, quantity, unit_price, amount, tax_rate, tax_amount)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			d.InvoiceDetailID,
			payload.Header.InvoiceHeaderID,
			d.ProductID,
			d.ProductName,
			d.Qty,
			d.UnitPrice,
			d.Amount,
			d.PpnPercent,
			d.Ppn,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":          "ok",
		"invoiceHeaderId": payload.Header.InvoiceHeaderID,
		"invoiceNo":       payload.Header.InvoiceNo,
		"totalDetails":    len(payload.Details),
	})
}

func (c InvoiceController) GetByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Path[len("/invoices/"):]
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var h struct {
		ID           string  `json:"id"`
		InvoiceNo    string  `json:"invoiceNo"`
		InvoiceDate  string  `json:"invoiceDate"`
		CustomerID   string  `json:"customerId"`
		CustomerName string  `json:"customerName"`
		Status       string  `json:"status"`
		TotalAmount  float64 `json:"totalAmount"`
		TotalTax     float64 `json:"totalTax"`
		CompanyCode  string  `json:"companyCode"`
	}
	err := c.DB.QueryRow(`SELECT id, invoice_no, invoice_date, customer_id, customer_name, status, total_amount, total_tax, company_code FROM invoice_headers WHERE id = ? AND deleted_at IS NULL`, id).
		Scan(&h.ID, &h.InvoiceNo, &h.InvoiceDate, &h.CustomerID, &h.CustomerName, &h.Status, &h.TotalAmount, &h.TotalTax, &h.CompanyCode)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rows, err := c.DB.Query(`SELECT id, product_id, description, quantity, unit_price, amount, tax_rate, tax_amount FROM invoice_details WHERE header_id = ? AND deleted_at IS NULL`, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	type Detail struct {
		ID          string  `json:"invoiceDetailId"`
		ProductID   string  `json:"productId"`
		ProductName string  `json:"productName"`
		Qty         float64 `json:"qty"`
		UnitPrice   float64 `json:"unitPrice"`
		Amount      float64 `json:"amount"`
		PpnPercent  float64 `json:"ppnPercent"`
		Ppn         float64 `json:"ppn"`
	}
	var details []Detail
	for rows.Next() {
		var d Detail
		if err := rows.Scan(&d.ID, &d.ProductID, &d.ProductName, &d.Qty, &d.UnitPrice, &d.Amount, &d.PpnPercent, &d.Ppn); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		details = append(details, d)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"header":  h,
		"details": details,
	})
}

func (c InvoiceController) CreateOrList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		c.Create(w, r)
	case http.MethodGet:
		q := r.URL.Query()
		var where []string
		var args []any
		if v := q.Get("status"); v != "" {
			where = append(where, "status = ?")
			args = append(args, v)
		}
		if v := q.Get("customerId"); v != "" {
			where = append(where, "customer_id = ?")
			args = append(args, v)
		}
		if v := q.Get("invoiceNo"); v != "" {
			where = append(where, "invoice_no LIKE ?")
			args = append(args, "%"+v+"%")
		}
		if v := q.Get("companyCode"); v != "" {
			where = append(where, "company_code = ?")
			args = append(args, v)
		}
		if v := q.Get("startDate"); v != "" {
			where = append(where, "invoice_date >= ?")
			args = append(args, v)
		}
		if v := q.Get("endDate"); v != "" {
			where = append(where, "invoice_date <= ?")
			args = append(args, v)
		}
		base := "SELECT id, invoice_no, invoice_date, customer_id, customer_name, status, total_amount, total_tax, company_code FROM invoice_headers WHERE deleted_at IS NULL"
		countBase := "SELECT COUNT(1) FROM invoice_headers WHERE deleted_at IS NULL"
		if len(where) > 0 {
			w := " AND " + strings.Join(where, " AND ")
			base += w
			countBase += w
		}
		base += " ORDER BY invoice_date DESC"
		lim := 50
		off := 0
		if v := q.Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
				lim = n
			}
		}
		if v := q.Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				off = n
			}
		}
		var total int
		if err := c.DB.QueryRow(countBase, args...).Scan(&total); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		base += " LIMIT ? OFFSET ?"
		args = append(args, lim, off)
		rows, err := c.DB.Query(base, args...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var list []map[string]any
		for rows.Next() {
			var m struct {
				ID, InvoiceNo, InvoiceDate, CustomerID, CustomerName, Status, CompanyCode string
				TotalAmount, TotalTax                                                     float64
			}
			if err := rows.Scan(&m.ID, &m.InvoiceNo, &m.InvoiceDate, &m.CustomerID, &m.CustomerName, &m.Status, &m.TotalAmount, &m.TotalTax, &m.CompanyCode); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			list = append(list, map[string]any{
				"id":           m.ID,
				"invoiceNo":    m.InvoiceNo,
				"invoiceDate":  m.InvoiceDate,
				"customerId":   m.CustomerID,
				"customerName": m.CustomerName,
				"status":       m.Status,
				"totalAmount":  m.TotalAmount,
				"totalTax":     m.TotalTax,
				"companyCode":  m.CompanyCode,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		hasNext := off+lim < total
		nextOffset := off + lim
		if !hasNext {
			nextOffset = off
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": list,
			"pagination": map[string]any{
				"total":      total,
				"limit":      lim,
				"offset":     off,
				"hasNext":    hasNext,
				"nextOffset": nextOffset,
			},
		})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
