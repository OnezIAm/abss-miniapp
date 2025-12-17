package controllers

import (
	"bank-consolidation/models"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type InvoiceController struct{ DB *gorm.DB }

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

	err := c.DB.Transaction(func(tx *gorm.DB) error {
		// Prepare header
		header := payload.Header
		header.Status = "pending"
		header.TotalTax = 0 // Will be calculated or assumed 0 as per original code? Original code set it to 0 in INSERT but loop might calculate?
		// Actually original code: INSERT ... VALUES (..., 0, ?) -> TotalTax = 0.
		// But wait, the loop calculates tax? No, the loop inserts into invoice_details with calculated tax, but doesn't update header.
		// Let's stick to original behavior: Header TotalTax = 0 in INSERT.

		if err := tx.Create(&header).Error; err != nil {
			return err
		}

		for _, d := range payload.Details {
			detail := d
			detail.InvoiceHeaderID = header.InvoiceHeaderID
			if err := tx.Create(&detail).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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

	var header models.InvoiceHeader
	if err := c.DB.Where("id = ?", id).First(&header).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var details []models.InvoiceDetail
	if err := c.DB.Where("header_id = ?", id).Find(&details).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Transform to response format if needed, or just return models
	// Original code returned a specific structure. Let's match it.
	h := struct {
		ID           string  `json:"id"`
		InvoiceNo    string  `json:"invoiceNo"`
		InvoiceDate  string  `json:"invoiceDate"`
		CustomerID   string  `json:"customerId"`
		CustomerName string  `json:"customerName"`
		Status       string  `json:"status"`
		TotalAmount  float64 `json:"totalAmount"`
		TotalTax     float64 `json:"totalTax"`
		CompanyCode  string  `json:"companyCode"`
	}{
		ID:           header.InvoiceHeaderID,
		InvoiceNo:    header.InvoiceNo,
		InvoiceDate:  header.InvoiceDate.Format("2006-01-02"), // Assuming simple date format
		CustomerID:   header.CustomerID,
		CustomerName: header.CustomerName,
		Status:       header.Status,
		TotalAmount:  header.TotalAmount,
		TotalTax:     header.TotalTax,
		CompanyCode:  header.CompanyCode,
	}

	// Details structure in original code
	type DetailResponse struct {
		ID          string  `json:"invoiceDetailId"`
		ProductID   string  `json:"productId"`
		ProductName string  `json:"productName"`
		Qty         float64 `json:"qty"`
		UnitPrice   float64 `json:"unitPrice"`
		Amount      float64 `json:"amount"`
		PpnPercent  float64 `json:"ppnPercent"`
		Ppn         float64 `json:"ppn"`
	}
	var detailsResp []DetailResponse
	for _, d := range details {
		detailsResp = append(detailsResp, DetailResponse{
			ID:          d.InvoiceDetailID,
			ProductID:   d.ProductID,
			ProductName: d.ProductName,
			Qty:         d.Qty,
			UnitPrice:   d.UnitPrice,
			Amount:      d.Amount,
			PpnPercent:  d.PpnPercent,
			Ppn:         d.Ppn,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"header":  h,
		"details": detailsResp,
	})
}

func (c InvoiceController) CreateOrList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		c.Create(w, r)
	case http.MethodGet:
		q := r.URL.Query()
		db := c.DB.Model(&models.InvoiceHeader{})

		if v := q.Get("status"); v != "" {
			db = db.Where("status = ?", v)
		}
		if v := q.Get("customerId"); v != "" {
			db = db.Where("customer_id = ?", v)
		}
		if v := q.Get("invoiceNo"); v != "" {
			db = db.Where("invoice_no LIKE ?", "%"+v+"%")
		}
		if v := q.Get("companyCode"); v != "" {
			db = db.Where("company_code = ?", v)
		}
		if v := q.Get("startDate"); v != "" {
			db = db.Where("invoice_date >= ?", v)
		}
		if v := q.Get("endDate"); v != "" {
			db = db.Where("invoice_date <= ?", v)
		}

		// Subquery for paid amount
		paidSubquery := "(SELECT COALESCE(SUM(matched_amount), 0) FROM bank_entry_invoices WHERE invoice_header_id = invoice_headers.id)"

		// Option to exclude fully paid
		if v := q.Get("excludeFullyPaid"); v == "1" || strings.EqualFold(v, "true") {
			condition := fmt.Sprintf("total_amount > %s", paidSubquery)

			if inc := q.Get("includeIds"); inc != "" {
				idsRaw := strings.Split(inc, ",")
				var ids []string
				for _, id := range idsRaw {
					if trimmed := strings.TrimSpace(id); trimmed != "" {
						ids = append(ids, trimmed)
					}
				}

				if len(ids) > 0 {
					db = db.Where(fmt.Sprintf("(%s OR invoice_headers.id IN ?)", condition), ids)

					// Prioritize included IDs so they appear on the first page
					quotedIds := make([]string, len(ids))
					for i, id := range ids {
						quotedIds[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(id, "'", "''"))
					}
					db = db.Order(gorm.Expr(fmt.Sprintf("CASE WHEN invoice_headers.id IN (%s) THEN 1 ELSE 0 END DESC", strings.Join(quotedIds, ","))))
				} else {
					db = db.Where(condition)
				}
			} else {
				db = db.Where(condition)
			}
		}

		// Count total before pagination
		var total int64
		if err := db.Count(&total).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Pagination
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

		db = db.Order("invoice_date DESC").Limit(lim).Offset(off)

		// Select fields including paid_amount
		type Result struct {
			models.InvoiceHeader
			PaidAmount float64 `json:"paidAmount" gorm:"column:paid_amount"`
		}
		var results []Result

		// We need to select specific fields to populate the struct correctly, especially the computed column
		// GORM can scan into struct.
		if err := db.Select("invoice_headers.*, " + paidSubquery + " as paid_amount").Scan(&results).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var list []map[string]any
		for _, m := range results {
			list = append(list, map[string]any{
				"id":           m.InvoiceHeaderID,
				"invoiceNo":    m.InvoiceNo,
				"invoiceDate":  m.InvoiceDate.Format("2006-01-02"), // simplified date
				"customerId":   m.CustomerID,
				"customerName": m.CustomerName,
				"status":       m.Status,
				"totalAmount":  m.TotalAmount,
				"totalTax":     m.TotalTax,
				"companyCode":  m.CompanyCode,
				"paidAmount":   m.PaidAmount,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		hasNext := int64(off+lim) < total
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

func (c InvoiceController) GenerateSample(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	err := c.DB.Transaction(func(tx *gorm.DB) error {
		for i := 0; i < 5; i++ {
			headerID := fmt.Sprintf("INV-H-%d", time.Now().UnixNano()+int64(i))
			invoiceNo := fmt.Sprintf("INV-%05d", rand.Intn(10000))
			customerID := fmt.Sprintf("CUST-%03d", rand.Intn(100))

			totalAmount := 0.0
			totalTax := 0.0

			numDetails := rand.Intn(3) + 1
			var details []models.InvoiceDetail

			for j := 0; j < numDetails; j++ {
				detailID := fmt.Sprintf("INV-D-%d-%d", time.Now().UnixNano()+int64(i), j)
				qty := float64(rand.Intn(10) + 1)
				price := float64(rand.Intn(1000)) / 10.0
				amount := qty * price
				tax := amount * 0.1

				details = append(details, models.InvoiceDetail{
					InvoiceDetailID: detailID,
					InvoiceHeaderID: headerID,
					ProductID:       fmt.Sprintf("PROD-%03d", rand.Intn(50)),
					ProductName:     fmt.Sprintf("Product Description %d", j),
					Qty:             qty,
					UnitPrice:       price,
					Amount:          amount,
					PpnPercent:      10.0,
					Ppn:             tax,
				})

				totalAmount += amount + tax
				totalTax += tax
			}

			header := models.InvoiceHeader{
				InvoiceHeaderID: headerID,
				InvoiceNo:       invoiceNo,
				InvoiceDate:     time.Now(),
				CustomerID:      customerID,
				CustomerName:    fmt.Sprintf("Customer %s", customerID),
				Status:          "pending",
				TotalAmount:     totalAmount,
				TotalTax:        totalTax,
				CompanyCode:     "CMP-001",
			}

			if err := tx.Create(&header).Error; err != nil {
				return err
			}
			if err := tx.Create(&details).Error; err != nil {
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
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "generated 5 sample invoices",
	})
}
