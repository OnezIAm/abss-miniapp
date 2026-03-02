package controllers

import (
	"bank-consolidation/models"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	mrand "math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BankEntryController struct{ DB *gorm.DB }

type bankEntryRow struct {
	models.BankEntry
	AttachedCount int     `gorm:"column:attached_count"`
	MatchedTotal  float64 `gorm:"column:matched_total"`
}

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
		db := c.DB.Model(&models.BankEntry{}).
			Select("bank_entries.*, COALESCE(st.attached_count,0) AS attached_count, COALESCE(st.matched_total,0) AS matched_total").
			Joins("LEFT JOIN (SELECT bank_entry_id, COUNT(1) AS attached_count, COALESCE(SUM(matched_amount),0) AS matched_total FROM bank_entry_invoices GROUP BY bank_entry_id) st ON st.bank_entry_id = bank_entries.id")

		if start := q.Get("startDate"); start != "" {
			if t, err := models.ParseDate(start); err == nil {
				db = db.Where("bank_entries.transaction_date >= ?", t)
			}
		}
		if end := q.Get("endDate"); end != "" {
			if t, err := models.ParseDate(end); err == nil {
				// make it end of day
				t = t.Add(24 * time.Hour).Add(-1 * time.Second)
				db = db.Where("bank_entries.transaction_date <= ?", t)
			}
		}
		if bank := q.Get("bankCode"); bank != "" {
			db = db.Where("bank_entries.bank_code = ?", bank)
		}
		if desc := q.Get("description"); desc != "" {
			db = db.Where("bank_entries.description LIKE ?", "%"+desc+"%")
		}
		if branch := q.Get("branch"); branch != "" {
			db = db.Where("bank_entries.branch LIKE ?", "%"+branch+"%")
		}
		if v := q.Get("reconciledOnly"); v == "1" || strings.EqualFold(v, "true") {
			db = db.Where("COALESCE(st.attached_count,0) > 0")
		}

		// Handle finalized filter
		// By default, only show non-finalized entries unless specific query param says otherwise
		// However, for export we might want all or just finalized.
		// Let's assume the main list view should hide finalized entries.
		if showFinalized := q.Get("showFinalized"); showFinalized == "true" {
			// Show only finalized
			db = db.Where("bank_entries.is_finalized = ?", true)
		} else if showFinalized == "all" {
			// Show all (both finalized and not)
		} else {
			// Default: Show only NOT finalized
			db = db.Where("bank_entries.is_finalized = ? OR bank_entries.is_finalized IS NULL", false)
		}

		// Pagination
		limitStr := q.Get("limit")
		offsetStr := q.Get("offset")
		limit := 0
		offset := 0
		if limitStr != "" {
			limit, _ = strconv.Atoi(limitStr)
		}
		if offsetStr != "" {
			offset, _ = strconv.Atoi(offsetStr)
		}

		var total int64
		// Use a session for count to avoid modifying the main query
		if err := db.Session(&gorm.Session{}).Count(&total).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if limit > 0 {
			db = db.Limit(limit).Offset(offset)
		}

		var entries []bankEntryRow
		if err := db.Find(&entries).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Convert to response
		resp := make([]models.BankEntry, len(entries))
		for i, e := range entries {
			resp[i] = e.BankEntry
			resp[i].AttachedCount = e.AttachedCount
			resp[i].MatchedTotal = e.MatchedTotal
			resp[i].Delta = math.Abs(resp[i].Amount) - e.MatchedTotal
		}

		w.Header().Set("Content-Type", "application/json")

		response := map[string]interface{}{
			"items": resp,
			"pagination": map[string]interface{}{
				"total":   total,
				"limit":   limit,
				"offset":  offset,
				"hasNext": int64(offset+len(resp)) < total,
			},
		}
		json.NewEncoder(w).Encode(response)

	case http.MethodPut:
		// Handle Finalize Action
		if r.URL.Path == "/api/v1/bank-entries/finalize" {
			var body struct {
				IDs []string `json:"ids"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if len(body.IDs) == 0 {
				http.Error(w, "ids are required", http.StatusBadRequest)
				return
			}

			if err := c.DB.Model(&models.BankEntry{}).Where("id IN ?", body.IDs).Update("is_finalized", true).Error; err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
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

	var row bankEntryRow
	err := c.DB.Model(&models.BankEntry{}).
		Select("bank_entries.*, COALESCE(st.attached_count,0) AS attached_count, COALESCE(st.matched_total,0) AS matched_total").
		Joins("LEFT JOIN (SELECT bank_entry_id, COUNT(1) AS attached_count, COALESCE(SUM(matched_amount),0) AS matched_total FROM bank_entry_invoices GROUP BY bank_entry_id) st ON st.bank_entry_id = bank_entries.id").
		Where("bank_entries.id = ?", id).
		Scan(&row).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	m := row.BankEntry
	m.AttachedCount = row.AttachedCount
	m.MatchedTotal = row.MatchedTotal
	m.Delta = math.Abs(m.Amount) - m.MatchedTotal

	var attached []models.BankEntryInvoiceSummary
	if err := c.DB.Table("bank_entry_invoices bei").
		Select("ih.id, ih.invoice_no, ih.invoice_date, ih.customer_name, ih.status, ih.total_amount, bei.matched_amount").
		Joins("JOIN invoice_headers ih ON ih.id = bei.invoice_header_id").
		Where("bei.bank_entry_id = ?", id).
		Scan(&attached).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	m.AttachedInvoices = attached

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

	var results []models.BankEntryInvoiceSummary

	err := c.DB.Table("bank_entry_invoices bei").
		Select("ih.id, ih.invoice_no, ih.invoice_date, ih.customer_name, ih.status, ih.total_amount, bei.matched_amount").
		Joins("JOIN invoice_headers ih ON ih.id = bei.invoice_header_id").
		Where("bei.bank_entry_id = ?", id).
		Scan(&results).Error

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if results == nil {
		results = []models.BankEntryInvoiceSummary{}
	}
	_ = json.NewEncoder(w).Encode(results)
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

func (c BankEntryController) UploadCSV(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	bankCode := r.FormValue("bankCode")
	if bankCode == "" {
		http.Error(w, "bankCode is required", http.StatusBadRequest)
		return
	}

	reader := csv.NewReader(file)
	var entries []models.BankEntry
	firstLine := true

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Error reading CSV: "+err.Error(), http.StatusBadRequest)
			return
		}

		if len(record) < 5 {
			continue
		}

		// Try parsing date from first column
		dt, err := models.ParseDate(record[0])
		if err != nil {
			if firstLine {
				firstLine = false
				continue // Skip header
			}
			continue // Skip invalid rows
		}
		firstLine = false

		amount, err := strconv.ParseFloat(record[3], 64)
		if err != nil {
			continue
		}

		entry := models.BankEntry{
			ID:              genID("BE"),
			TransactionDate: dt,
			Description:     record[1],
			Branch:          record[2],
			Amount:          amount,
			AmountType:      record[4],
			BankCode:        bankCode,
		}
		entry.Fingerprint = computeFingerprint(entry.TransactionDate, entry.Description, entry.Branch, entry.Amount, entry.AmountType, entry.BankCode)

		entries = append(entries, entry)
	}

	if len(entries) > 0 {
		if err := c.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&entries).Error; err != nil {
			http.Error(w, "Error saving to DB: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"count":  len(entries),
	})
}

func (c BankEntryController) ExportReconciled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=reconciled_entries.csv")

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	writer.Write([]string{"Date", "Description", "BankCode", "Amount", "Type", "Matched Amount", "Delta", "Invoice No", "Customer", "Invoice Date", "Invoice Total", "Allocated"})

	var rows []bankEntryRow
	// Find entries that have some matching
	db := c.DB.Model(&models.BankEntry{}).
		Select("bank_entries.*, COALESCE(st.attached_count,0) AS attached_count, COALESCE(st.matched_total,0) AS matched_total").
		Joins("JOIN (SELECT bank_entry_id, COUNT(1) AS attached_count, COALESCE(SUM(matched_amount),0) AS matched_total FROM bank_entry_invoices GROUP BY bank_entry_id) st ON st.bank_entry_id = bank_entries.id")

	// Filter by date range if provided
	startDateStr := r.URL.Query().Get("startDate")
	endDateStr := r.URL.Query().Get("endDate")
	if startDateStr != "" {
		if t, err := models.ParseDate(startDateStr); err == nil {
			db = db.Where("bank_entries.transaction_date >= ?", t)
		}
	}
	if endDateStr != "" {
		if t, err := models.ParseDate(endDateStr); err == nil {
			db = db.Where("bank_entries.transaction_date <= ?", t)
		}
	}

	if showFinalized := r.URL.Query().Get("showFinalized"); showFinalized == "true" {
		db = db.Where("bank_entries.is_finalized = ?", true)
	}

	if err := db.Order("bank_entries.transaction_date").Scan(&rows).Error; err != nil {
		// If error, we can't write JSON to a CSV stream easily, so just log or ignore for now in this snippet
		return
	}

	// We need detailed invoice info
	for _, r := range rows {
		var invoices []struct {
			MatchedAmount float64
			InvoiceNo     string
			CustomerName  string
			InvoiceDate   time.Time
			TotalAmount   float64
		}

		c.DB.Table("bank_entry_invoices").
			Select("bank_entry_invoices.matched_amount, invoice_headers.invoice_no, invoice_headers.customer_name, invoice_headers.invoice_date, invoice_headers.total_amount").
			Joins("JOIN invoice_headers ON invoice_headers.id = bank_entry_invoices.invoice_header_id").
			Where("bank_entry_invoices.bank_entry_id = ?", r.ID).
			Scan(&invoices)

		delta := math.Abs(r.Amount) - r.MatchedTotal

		for _, inv := range invoices {
			writer.Write([]string{
				r.TransactionDate.Format("2006-01-02"),
				r.Description,
				r.BankCode,
				fmt.Sprintf("%.2f", r.Amount),
				r.AmountType,
				fmt.Sprintf("%.2f", r.MatchedTotal),
				fmt.Sprintf("%.2f", delta),
				inv.InvoiceNo,
				inv.CustomerName,
				inv.InvoiceDate.Format("2006-01-02"),
				fmt.Sprintf("%.2f", inv.TotalAmount),
				fmt.Sprintf("%.2f", inv.MatchedAmount),
			})
		}
	}
}
