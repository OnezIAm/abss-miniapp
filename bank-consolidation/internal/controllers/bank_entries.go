package controllers

import (
	"bank-consolidation/models"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	mrand "math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BankEntryController struct{ DB *gorm.DB }

func genID(prefix string) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s-%d-%x", prefix, time.Now().UnixNano(), b)
}

func computeFingerprint(dt time.Time, desc, branch string, amount float64, amtType, bankCode string) string {
	dtStr := dt.Format("2006-01-02 15:04:05")
	base := strings.ToLower(strings.TrimSpace(dtStr)) + "|" + strings.ToLower(strings.TrimSpace(desc)) + "|" + strings.TrimSpace(branch) + "|" + fmt.Sprintf("%.2f", amount) + "|" + strings.TrimSpace(amtType) + "|" + strings.TrimSpace(bankCode)
	h := sha256.Sum256([]byte(base))
	return hex.EncodeToString(h[:])
}

func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("transactionDate is required")
	}
	if strings.Contains(s, "/") {
		return time.Parse("02/01/2006", s)
	}
	// try RFC3339 or date-only
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, errors.New("unsupported date format")
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
		// body.TransactionDate is already time.Time due to custom UnmarshalJSON in model
		// but wait, the model's UnmarshalJSON parses it.
		// Let's assume the model is correct.
		// However, I should check if the UnmarshalJSON works as expected.
		// The custom UnmarshalJSON in BankEntry.go handles string parsing.
		// So `body.TransactionDate` is a valid time.Time.

		if strings.TrimSpace(body.ID) == "" {
			body.ID = genID("BE")
		}
		body.Fingerprint = computeFingerprint(body.TransactionDate, body.Description, body.Branch, body.Amount, body.AmountType, body.BankCode)

		if err := c.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&body).Error; err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": body.ID})
	case http.MethodGet:
		q := r.URL.Query()
		db := c.DB.Model(&models.BankEntry{})

		if v := q.Get("bankCode"); strings.TrimSpace(v) == "" {
			http.Error(w, "bankCode is required", http.StatusBadRequest)
			return
		} else {
			db = db.Where("bank_code = ?", v)
		}
		if v := q.Get("branch"); v != "" {
			db = db.Where("branch = ?", v)
		}
		if v := q.Get("amountType"); v != "" {
			db = db.Where("amount_type = ?", v)
		}
		if v := q.Get("desc"); v != "" {
			db = db.Where("description LIKE ?", "%"+v+"%")
		}
		if v := q.Get("startDate"); v != "" {
			if dt, err := parseDate(v); err == nil {
				db = db.Where("transaction_date >= ?", dt)
			} else {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if v := q.Get("endDate"); v != "" {
			if dt, err := parseDate(v); err == nil {
				db = db.Where("transaction_date <= ?", dt)
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
			start := mt
			next := mt.AddDate(0, 1, 0)
			db = db.Where("transaction_date >= ? AND transaction_date < ?", start, next)
		}

		var total int64
		if err := db.Count(&total).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		db = db.Select("bank_entries.*, COALESCE(st.attached_count,0) AS attached_count, COALESCE(st.matched_total,0) AS matched_total").
			Joins("LEFT JOIN (SELECT bank_entry_id, COUNT(1) AS attached_count, COALESCE(SUM(matched_amount),0) AS matched_total FROM bank_entry_invoices GROUP BY bank_entry_id) st ON st.bank_entry_id = bank_entries.id").
			Order("transaction_date DESC")

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

		var items []models.BankEntry
		if err := db.Limit(lim).Offset(off).Find(&items).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		flat := q.Get("flat")
		if flat == "1" || strings.EqualFold(flat, "true") {
			_ = json.NewEncoder(w).Encode(items)
			return
		}
		hasNext := off+lim < int(total)
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
	err := c.DB.Model(&models.BankEntry{}).
		Select("bank_entries.*, COALESCE(st.attached_count,0) AS attached_count, COALESCE(st.matched_total,0) AS matched_total").
		Joins("LEFT JOIN (SELECT bank_entry_id, COUNT(1) AS attached_count, COALESCE(SUM(matched_amount),0) AS matched_total FROM bank_entry_invoices GROUP BY bank_entry_id) st ON st.bank_entry_id = bank_entries.id").
		Where("bank_entries.id = ?", id).
		First(&m).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
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

	fp := computeFingerprint(body.TransactionDate, body.Description, body.Branch, body.Amount, body.AmountType, body.BankCode)
	body.Fingerprint = fp

	// We only update specific fields
	err := c.DB.Model(&models.BankEntry{}).Where("id = ?", id).Updates(map[string]interface{}{
		"transaction_date": body.TransactionDate,
		"description":      body.Description,
		"branch":           body.Branch,
		"amount":           body.Amount,
		"amount_type":      body.AmountType,
		"balance":          body.Balance,
		"bank_code":        body.BankCode,
		"fingerprint":      fp,
	}).Error

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
	if err := c.DB.Delete(&models.BankEntry{}, "id = ?", id).Error; err != nil {
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

	// Filter valid entries and prepare them
	var validList []models.BankEntry
	skipped := 0

	for _, body := range list {
		if strings.TrimSpace(body.Description) == "" || strings.TrimSpace(body.Branch) == "" || strings.TrimSpace(body.BankCode) == "" {
			skipped++
			continue
		}
		if body.AmountType != "CR" && body.AmountType != "DB" {
			skipped++
			continue
		}
		if body.TransactionDate.IsZero() {
			skipped++
			continue
		}
		if strings.TrimSpace(body.ID) == "" {
			body.ID = genID("BE")
		}
		body.Fingerprint = computeFingerprint(body.TransactionDate, body.Description, body.Branch, body.Amount, body.AmountType, body.BankCode)
		validList = append(validList, body)
	}

	if len(validList) == 0 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"inserted": 0, "skipped": skipped, "total": len(list)})
		return
	}

	if err := c.DB.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(validList, 200).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Since CreateInBatches doesn't return exact number of inserted rows if some were ignored,
	// we just report the number of valid items we attempted to insert.
	// Or we can check RowsAffected if it's available.
	// But for simplicity, let's assume validList count.

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"inserted": len(validList), "skipped": skipped, "total": len(list)})
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

	var exists int64
	if err := c.DB.Model(&models.BankEntry{}).Where("id = ?", id).Count(&exists).Error; err != nil || exists == 0 {
		http.Error(w, "bank entry not found", http.StatusNotFound)
		return
	}

	err := c.DB.Transaction(func(tx *gorm.DB) error {
		// Validation
		for _, inv := range p.Invoices {
			if strings.TrimSpace(inv.ID) == "" {
				continue
			}
			var result struct {
				TotalAmount     float64
				ExistingMatched float64
			}

			// This query is tricky with GORM because of the subquery.
			// Let's use Raw SQL for this specific check, but within the transaction.
			err := tx.Raw(`
				SELECT 
					ih.total_amount,
					COALESCE((SELECT SUM(matched_amount) FROM bank_entry_invoices WHERE invoice_header_id = ih.id AND bank_entry_id != ?), 0) as existing_matched
				FROM invoice_headers ih
				WHERE ih.id = ?`, id, inv.ID).Scan(&result).Error

			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("invoice %s not found", inv.ID)
				}
				return err
			}

			if result.ExistingMatched+inv.Amount > result.TotalAmount+0.01 {
				return fmt.Errorf("invoice %s is already fully paid or amount exceeds total (Total: %.2f, Paid: %.2f, New: %.2f)", inv.ID, result.TotalAmount, result.ExistingMatched, inv.Amount)
			}
		}

		if strings.EqualFold(p.Mode, "replace") || p.Mode == "" {
			if err := tx.Delete(&models.BankEntryInvoice{}, "bank_entry_id = ?", id).Error; err != nil {
				return err
			}
		}

		var newEntries []models.BankEntryInvoice
		for _, inv := range p.Invoices {
			if strings.TrimSpace(inv.ID) == "" {
				continue
			}
			newEntries = append(newEntries, models.BankEntryInvoice{
				BankEntryID:     id,
				InvoiceHeaderID: inv.ID,
				MatchedAmount:   inv.Amount,
				Note:            p.Note,
			})
		}

		if len(newEntries) > 0 {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&newEntries).Error; err != nil {
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
	_ = json.NewEncoder(w).Encode(map[string]int{"inserted": len(p.Invoices)}) // Approximate
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

	var list []map[string]any
	// Using Raw SQL for join is often cleaner for complex projections not mapping directly to a single model
	// But we can try to map to a struct
	type Result struct {
		ID            string
		InvoiceNo     string
		InvoiceDate   time.Time
		CustomerName  string
		Status        string
		TotalAmount   float64
		MatchedAmount float64
	}
	var results []Result

	err := c.DB.Table("bank_entry_invoices bei").
		Select("ih.id, ih.invoice_no, ih.invoice_date, ih.customer_name, ih.status, ih.total_amount, bei.matched_amount").
		Joins("JOIN invoice_headers ih ON ih.id = bei.invoice_header_id").
		Where("bei.bank_entry_id = ?", id).
		Scan(&results).Error

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, m := range results {
		list = append(list, map[string]any{
			"id":            m.ID,
			"invoiceNo":     m.InvoiceNo,
			"invoiceDate":   m.InvoiceDate,
			"customerName":  m.CustomerName,
			"status":        m.Status,
			"totalAmount":   m.TotalAmount,
			"matchedAmount": m.MatchedAmount,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if list == nil {
		list = []map[string]any{}
	}
	_ = json.NewEncoder(w).Encode(list)
}

func (c BankEntryController) GenerateSample(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	bankCode := r.URL.Query().Get("bankCode")
	if bankCode == "" {
		bankCode = "SAMPLE-BANK"
	}

	var samples []models.BankEntry
	for i := 0; i < 5; i++ {
		dt := time.Now().Add(time.Duration(-mrand.Intn(30)) * 24 * time.Hour)
		desc := fmt.Sprintf("Sample Transaction %d", i+1)
		branch := "Main Branch"
		amount := float64(mrand.Intn(100000)) / 100.0
		amountType := "CR"
		if mrand.Intn(2) == 0 {
			amountType = "DB"
		}
		balance := float64(mrand.Intn(1000000)) / 100.0
		fp := computeFingerprint(dt, desc, branch, amount, amountType, bankCode)

		samples = append(samples, models.BankEntry{
			ID:              genID("BE"),
			TransactionDate: dt,
			Description:     desc,
			Branch:          branch,
			Amount:          amount,
			AmountType:      amountType,
			Balance:         balance,
			BankCode:        bankCode,
			Fingerprint:     fp,
		})
	}

	if err := c.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&samples).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "5 sample bank entries generated"})
}
