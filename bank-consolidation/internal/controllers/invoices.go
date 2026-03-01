package controllers

import (
	"bank-consolidation/models"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InvoiceController struct{ DB *gorm.DB }

func validateInvoiceHeader(h models.InvoiceHeader) error {
	if h.InvoiceHeaderID == nil || fmt.Sprintf("%v", h.InvoiceHeaderID) == "" {
		return errors.New("invoiceHeaderId is required")
	}
	if h.InvoiceNo == "" {
		return errors.New("invoiceNo is required")
	}
	if len(h.Details) == 0 {
		return errors.New("details must not be empty")
	}
	for i, d := range h.Details {
		if d.InvoiceDetailID == nil || fmt.Sprintf("%v", d.InvoiceDetailID) == "" {
			return errors.New("details[" + strconv.Itoa(i) + "].invoiceDetailId is required")
		}
		if d.ProductID == nil || fmt.Sprintf("%v", d.ProductID) == "" {
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
	var header models.InvoiceHeader
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&header); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateInvoiceHeader(header); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := c.DB.Transaction(func(tx *gorm.DB) error {
		// Prepare header
		if header.Status == "" {
			header.Status = "pending"
		}

		if err := tx.Create(&header).Error; err != nil {
			return err
		}

		for _, d := range header.Details {
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
		"invoiceHeaderId": header.InvoiceHeaderID,
		"invoiceNo":       header.InvoiceNo,
		"totalDetails":    len(header.Details),
	})
}

func (c InvoiceController) BulkCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var headers []models.InvoiceHeader

	// Read body for debugging
	bodyBytes, _ := io.ReadAll(r.Body)
	// Restore body
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	fmt.Printf("DEBUG: Received Payload: %s\n", string(bodyBytes))

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&headers); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(headers) == 0 {
		http.Error(w, "empty list", http.StatusBadRequest)
		return
	}

	for i, h := range headers {
		if err := validateInvoiceHeader(h); err != nil {
			http.Error(w, fmt.Sprintf("index %d: %v", i, err), http.StatusBadRequest)
			return
		}
	}

	err := c.DB.Transaction(func(tx *gorm.DB) error {
		for _, header := range headers {
			if header.Status == "" {
				header.Status = "pending"
			}

			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&header).Error; err != nil {
				return err
			}

			for _, d := range header.Details {
				detail := d
				detail.InvoiceHeaderID = header.InvoiceHeaderID
				if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&detail).Error; err != nil {
					return err
				}
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
		"status": "ok",
		"count":  len(headers),
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
		ID           interface{} `json:"id"`
		InvoiceNo    string      `json:"invoiceNo"`
		InvoiceDate  string      `json:"invoiceDate"`
		CustomerID   interface{} `json:"customerId"`
		CustomerName string      `json:"customerName"`
		Status       string      `json:"status"`
		TotalAmount  float64     `json:"totalAmount"`
		TotalTax     float64     `json:"totalTax"`
		CompanyCode  string      `json:"companyCode"`
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
		ID          interface{} `json:"invoiceDetailId"`
		ProductID   interface{} `json:"productId"`
		ProductName string      `json:"productName"`
		Qty         float64     `json:"qty"`
		UnitPrice   float64     `json:"unitPrice"`
		Amount      float64     `json:"amount"`
		PpnPercent  float64     `json:"ppnPercent"`
		Ppn         float64     `json:"ppn"`
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
		includeIDs := make([]string, 0)
		if inc := strings.TrimSpace(q.Get("includeIds")); inc != "" {
			parts := strings.Split(inc, ",")
			seen := make(map[string]struct{}, len(parts))
			for _, p := range parts {
				id := strings.TrimSpace(p)
				if id == "" {
					continue
				}
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				includeIDs = append(includeIDs, id)
			}
		}

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

			if len(includeIDs) > 0 {
				db = db.Where(fmt.Sprintf("(%s OR id IN ?)", condition), includeIDs)
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
			PaidAmount float64 `json:"paidAmount"`
		}
		var results []Result

		// We need to select specific fields to populate the struct correctly, especially the computed column
		// GORM can scan into struct.
		if err := db.Select("*, " + paidSubquery + " as paid_amount").Scan(&results).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if len(includeIDs) > 0 {
			existing := make(map[string]struct{}, len(results))
			for _, r := range results {
				existing[fmt.Sprint(r.InvoiceHeaderID)] = struct{}{}
			}

			var extras []Result
			if err := c.DB.Model(&models.InvoiceHeader{}).
				Where("id IN ?", includeIDs).
				Select("*, " + paidSubquery + " as paid_amount").
				Scan(&extras).Error; err == nil {
				for _, ex := range extras {
					id := fmt.Sprint(ex.InvoiceHeaderID)
					if _, ok := existing[id]; ok {
						continue
					}
					existing[id] = struct{}{}
					results = append(results, ex)
				}
			}
		}

		list := make([]map[string]any, 0)
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
