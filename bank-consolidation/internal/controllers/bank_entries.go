package controllers

import (
	"bank-consolidation/models"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type BankEntryController struct{ DB *sql.DB }

func genID(prefix string) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s-%d-%x", prefix, time.Now().UnixNano(), b)
}

func computeFingerprint(dt, desc, branch string, amount float64, amtType, bankCode string) string {
	base := strings.ToLower(strings.TrimSpace(dt)) + "|" + strings.ToLower(strings.TrimSpace(desc)) + "|" + strings.TrimSpace(branch) + "|" + fmt.Sprintf("%.2f", amount) + "|" + strings.TrimSpace(amtType) + "|" + strings.TrimSpace(bankCode)
	h := sha256.Sum256([]byte(base))
	return hex.EncodeToString(h[:])
}

func parseDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("transactionDate is required")
	}
	if strings.Contains(s, "/") {
		t, err := time.Parse("02/01/2006", s)
		if err != nil {
			return "", err
		}
		return t.Format("2006-01-02 15:04:05"), nil
	}
	// try RFC3339 or date-only
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Format("2006-01-02 15:04:05"), nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Format("2006-01-02 15:04:05"), nil
	}
	return "", errors.New("unsupported date format")
}

func (c BankEntryController) CreateOrList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var body models.BankEntry
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body.Description == "" || body.Branch == "" || strings.TrimSpace(body.BankCode) == "" {
			http.Error(w, "description, branch, bankCode are required", http.StatusBadRequest)
			return
		}
		if body.AmountType != "CR" && body.AmountType != "DB" {
			http.Error(w, "amountType must be CR or DB", http.StatusBadRequest)
			return
		}
		dt, err := parseDate(body.TransactionDate)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.ID) == "" {
			body.ID = genID("BE")
		}
		fp := computeFingerprint(dt, body.Description, body.Branch, body.Amount, body.AmountType, body.BankCode)
		_, err = c.DB.Exec(`INSERT IGNORE INTO bank_entries (id, transaction_date, description, branch, amount, amount_type, balance, bank_code, fingerprint) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			body.ID, dt, body.Description, body.Branch, body.Amount, body.AmountType, body.Balance, body.BankCode, fp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": body.ID})
	case http.MethodGet:
		q := r.URL.Query()
		var where []string
		var args []any
		if v := q.Get("bankCode"); strings.TrimSpace(v) == "" {
			http.Error(w, "bankCode is required", http.StatusBadRequest)
			return
		} else {
			where = append(where, "bank_code = ?")
			args = append(args, v)
		}
		if v := q.Get("branch"); v != "" {
			where = append(where, "branch = ?")
			args = append(args, v)
		}
		if v := q.Get("amountType"); v != "" {
			where = append(where, "amount_type = ?")
			args = append(args, v)
		}
		if v := q.Get("desc"); v != "" {
			where = append(where, "description LIKE ?")
			args = append(args, "%"+v+"%")
		}
		if v := q.Get("startDate"); v != "" {
			where = append(where, "transaction_date >= ?")
			if dt, err := parseDate(v); err == nil {
				args = append(args, dt)
			} else {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if v := q.Get("endDate"); v != "" {
			where = append(where, "transaction_date <= ?")
			if dt, err := parseDate(v); err == nil {
				args = append(args, dt)
			} else {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if v := q.Get("month"); v != "" {
			mt, err := time.Parse("2006-01", v)
			if err != nil {
				http.Error(w, "invalid month, expected YYYY-MM", http.StatusBadRequest)
				return
			}
			start := mt.Format("2006-01-02 15:04:05")
			next := mt.AddDate(0, 1, 0).Format("2006-01-02 15:04:05")
			where = append(where, "transaction_date >= ?")
			args = append(args, start)
			where = append(where, "transaction_date < ?")
			args = append(args, next)
		}
		base := "SELECT be.id, DATE_FORMAT(be.transaction_date,'%Y-%m-%dT%H:%i:%sZ') AS transaction_date, be.description, be.branch, be.amount, be.amount_type, be.balance, be.bank_code, COALESCE(st.attached_count,0) AS attached_count, COALESCE(st.matched_total,0) AS matched_total FROM bank_entries be LEFT JOIN (SELECT bank_entry_id, COUNT(1) AS attached_count, COALESCE(SUM(matched_amount),0) AS matched_total FROM bank_entry_invoices GROUP BY bank_entry_id) st ON st.bank_entry_id = be.id WHERE be.deleted_at IS NULL"
		countBase := "SELECT COUNT(1) FROM bank_entries be WHERE be.deleted_at IS NULL"
		if len(where) > 0 {
			wc := " AND " + strings.Join(where, " AND ")
			base += wc
			countBase += wc
		}
		base += " ORDER BY transaction_date DESC"
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
		items := make([]models.BankEntry, 0)
		for rows.Next() {
			var m models.BankEntry
			if err := rows.Scan(&m.ID, &m.TransactionDate, &m.Description, &m.Branch, &m.Amount, &m.AmountType, &m.Balance, &m.BankCode, &m.AttachedCount, &m.MatchedTotal); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			items = append(items, m)
		}
		w.Header().Set("Content-Type", "application/json")
		flat := q.Get("flat")
		if flat == "1" || strings.EqualFold(flat, "true") {
			_ = json.NewEncoder(w).Encode(items)
			return
		}
		hasNext := off+lim < total
		nextOffset := off + lim
		if !hasNext {
			nextOffset = off
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": items,
			"pagination": map[string]any{
				"total": total, "limit": lim, "offset": off, "hasNext": hasNext, "nextOffset": nextOffset,
			},
		})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c BankEntryController) GetByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/bank-entries/")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var m models.BankEntry
	err := c.DB.QueryRow(`SELECT be.id, DATE_FORMAT(be.transaction_date,'%Y-%m-%dT%H:%i:%sZ'), be.description, be.branch, be.amount, be.amount_type, be.balance, be.bank_code, COALESCE(st.attached_count,0), COALESCE(st.matched_total,0)
		FROM bank_entries be
		LEFT JOIN (SELECT bank_entry_id, COUNT(1) AS attached_count, COALESCE(SUM(matched_amount),0) AS matched_total FROM bank_entry_invoices GROUP BY bank_entry_id) st
		ON st.bank_entry_id = be.id
		WHERE be.id = ? AND be.deleted_at IS NULL`, id).
		Scan(&m.ID, &m.TransactionDate, &m.Description, &m.Branch, &m.Amount, &m.AmountType, &m.Balance, &m.BankCode, &m.AttachedCount, &m.MatchedTotal)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(m)
}

func (c BankEntryController) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/bank-entries/")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var body models.BankEntry
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.AmountType != "CR" && body.AmountType != "DB" {
		http.Error(w, "amountType must be CR or DB", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.BankCode) == "" {
		http.Error(w, "bankCode is required", http.StatusBadRequest)
		return
	}
	dt, err := parseDate(body.TransactionDate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fp := computeFingerprint(dt, body.Description, body.Branch, body.Amount, body.AmountType, body.BankCode)
	_, err = c.DB.Exec(`UPDATE bank_entries SET transaction_date = ?, description = ?, branch = ?, amount = ?, amount_type = ?, balance = ?, bank_code = ?, fingerprint = ? WHERE id = ? AND deleted_at IS NULL`,
		dt, body.Description, body.Branch, body.Amount, body.AmountType, body.Balance, body.BankCode, fp, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": id})
}

func (c BankEntryController) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/bank-entries/")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	_, err := c.DB.Exec(`UPDATE bank_entries SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": id})
}

func (c BankEntryController) BulkCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var list []models.BankEntry
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&list); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(list) == 0 {
		http.Error(w, "payload must be a non-empty array", http.StatusBadRequest)
		return
	}
	tx, err := c.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() {
		_ = tx.Rollback()
	}()
	const batchSize = 200
	inserted := 0
	skipped := 0
	for i := 0; i < len(list); i += batchSize {
		end := i + batchSize
		if end > len(list) {
			end = len(list)
		}
		var sb strings.Builder
		args := make([]any, 0, (end-i)*9)
		sb.WriteString("INSERT IGNORE INTO bank_entries (id, transaction_date, description, branch, amount, amount_type, balance, bank_code, fingerprint) VALUES ")
		first := true
		for _, body := range list[i:end] {
			if strings.TrimSpace(body.Description) == "" || strings.TrimSpace(body.Branch) == "" || strings.TrimSpace(body.BankCode) == "" {
				skipped++
				continue
			}
			if body.AmountType != "CR" && body.AmountType != "DB" {
				skipped++
				continue
			}
			dt, err := parseDate(body.TransactionDate)
			if err != nil {
				skipped++
				continue
			}
			if strings.TrimSpace(body.ID) == "" {
				body.ID = genID("BE")
			}
			fp := computeFingerprint(dt, body.Description, body.Branch, body.Amount, body.AmountType, body.BankCode)
			if !first {
				sb.WriteString(",")
			}
			first = false
			sb.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?)")
			args = append(args, body.ID, dt, body.Description, body.Branch, body.Amount, body.AmountType, body.Balance, body.BankCode, fp)
		}
		if len(args) == 0 {
			continue
		}
		res, err := tx.Exec(sb.String(), args...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		aff, _ := res.RowsAffected()
		inserted += int(aff)
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"inserted": inserted, "skipped": skipped, "total": len(list)})
}

type reconcilePayload struct {
	Invoices []struct {
		ID     string  `json:"id"`
		Amount float64 `json:"amount"`
	} `json:"invoices"`
	Note string `json:"note"`
	Mode string `json:"mode"`
}

func (c BankEntryController) Reconcile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/bank-entries/")
	id = strings.TrimSuffix(id, "/reconcile")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var p reconcilePayload
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(p.Invoices) == 0 {
		http.Error(w, "invoices must be non-empty", http.StatusBadRequest)
		return
	}
	var exists int
	if err := c.DB.QueryRow(`SELECT COUNT(1) FROM bank_entries WHERE id = ? AND deleted_at IS NULL`, id).Scan(&exists); err != nil || exists == 0 {
		http.Error(w, "bank entry not found", http.StatusNotFound)
		return
	}
	tx, err := c.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback() }()
	if strings.EqualFold(p.Mode, "replace") || p.Mode == "" {
		if _, err := tx.Exec(`DELETE FROM bank_entry_invoices WHERE bank_entry_id = ?`, id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	const batchSize = 100
	inserted := 0
	for i := 0; i < len(p.Invoices); i += batchSize {
		end := i + batchSize
		if end > len(p.Invoices) {
			end = len(p.Invoices)
		}
		var sb strings.Builder
		args := make([]any, 0, (end-i)*4)
		sb.WriteString("INSERT IGNORE INTO bank_entry_invoices (bank_entry_id, invoice_header_id, matched_amount, note) VALUES ")
		first := true
		for _, inv := range p.Invoices[i:end] {
			if strings.TrimSpace(inv.ID) == "" {
				continue
			}
			if !first {
				sb.WriteString(",")
			}
			first = false
			sb.WriteString("(?, ?, ?, ?)")
			args = append(args, id, inv.ID, inv.Amount, p.Note)
		}
		if len(args) == 0 {
			continue
		}
		res, err := tx.Exec(sb.String(), args...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		aff, _ := res.RowsAffected()
		inserted += int(aff)
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"inserted": inserted})
}

func (c BankEntryController) ListAttachedInvoices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/bank-entries/")
	id = strings.TrimSuffix(id, "/invoices")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	rows, err := c.DB.Query(`
		SELECT ih.id, ih.invoice_no, DATE_FORMAT(ih.invoice_date,'%Y-%m-%dT%H:%i:%sZ') AS invoice_date,
		       ih.customer_id, ih.customer_name, ih.status, ih.total_amount, ih.total_tax, ih.company_code,
		       bei.matched_amount
		FROM bank_entry_invoices bei
		JOIN invoice_headers ih ON ih.id = bei.invoice_header_id
		WHERE bei.bank_entry_id = ?
		ORDER BY ih.invoice_date DESC
	`, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	type item struct {
		ID            string  `json:"id"`
		InvoiceNo     string  `json:"invoiceNo"`
		InvoiceDate   string  `json:"invoiceDate"`
		CustomerID    string  `json:"customerId"`
		CustomerName  string  `json:"customerName"`
		Status        string  `json:"status"`
		TotalAmount   float64 `json:"totalAmount"`
		TotalTax      float64 `json:"totalTax"`
		CompanyCode   string  `json:"companyCode"`
		MatchedAmount float64 `json:"matchedAmount"`
	}
	items := make([]item, 0)
	for rows.Next() {
		var m item
		if err := rows.Scan(&m.ID, &m.InvoiceNo, &m.InvoiceDate, &m.CustomerID, &m.CustomerName, &m.Status, &m.TotalAmount, &m.TotalTax, &m.CompanyCode, &m.MatchedAmount); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		items = append(items, m)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}
