package models

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type InvoiceHeader struct {
	InvoiceHeaderID   interface{}     `json:"invoiceHeaderId" gorm:"column:id;primaryKey;type:varchar(64)"`
	InvoiceNo         string          `json:"invoiceNo" gorm:"type:varchar(64);not null"`
	InvoiceSequenceNo string          `json:"invoiceSequenceNo" gorm:"-"`
	InvoiceDate       time.Time       `json:"invoiceDate" gorm:"type:datetime;not null"`
	TradeDate         string          `json:"tradeDate" gorm:"-"`
	SalesOrderID      interface{}     `json:"salesOrderId" gorm:"-"`
	SalesOrderNo      string          `json:"salesOrderNo" gorm:"-"`
	DeliveryOrderID   interface{}     `json:"deliveryOrderId" gorm:"-"`
	DeliveryOrderNo   string          `json:"deliveryOrderNo" gorm:"-"`
	PurchaseOrderNo   interface{}     `json:"purchaseOrderNo" gorm:"-"`
	SalesID           interface{}     `json:"salesId" gorm:"-"`
	SalesName         string          `json:"salesName" gorm:"-"`
	CustomerID        interface{}     `json:"customerId" gorm:"column:customer_id;type:varchar(64);not null"`
	CustomerName      string          `json:"customerName" gorm:"column:customer_name;type:varchar(255);not null"`
	DeliverTo         string          `json:"deliverTo" gorm:"-"`
	Status            string          `json:"status" gorm:"type:varchar(32);not null;default:'pending'"`
	TotalAmount       float64         `json:"totalAmount" gorm:"type:decimal(15,2);not null"`
	TotalTax          float64         `json:"totalTax" gorm:"type:decimal(15,2);not null"`
	TotalProduct      int             `json:"totalProduct" gorm:"-"`
	DueDate           string          `json:"dueDate" gorm:"-"`
	Termin            interface{}     `json:"termin" gorm:"-"`
	Notes             string          `json:"notes" gorm:"-"`
	CompanyCode       string          `json:"companyCode" gorm:"column:company_code;type:varchar(64);not null"`
	OtherExpense      interface{}     `json:"otherExpense" gorm:"-"`
	Details           []InvoiceDetail `json:"detail" gorm:"-"`
	DeletedAt         gorm.DeletedAt  `json:"-" gorm:"index"`
}

func (h *InvoiceHeader) UnmarshalJSON(data []byte) error {
	type Alias InvoiceHeader
	aux := &struct {
		InvoiceDate string `json:"invoiceDate"`
		*Alias
	}{
		Alias: (*Alias)(h),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.InvoiceDate == "" {
		return nil
	}

	s := strings.TrimSpace(aux.InvoiceDate)
	var t time.Time
	var err error

	// Try standard formats
	t, err = time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02", s)
	}
	if err != nil {
		// Try custom format like DD/MM/YYYY if needed, but keeping it simple for now
		return errors.New("unsupported invoice date format")
	}

	h.InvoiceDate = t

	// Fallback for plural 'details' or 'items' if 'detail' is empty
	if len(h.Details) == 0 {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err == nil {
			var targetKey string
			if _, ok := raw["details"]; ok {
				targetKey = "details"
			} else if _, ok := raw["items"]; ok {
				targetKey = "items"
			}

			if targetKey != "" {
				var details []InvoiceDetail
				if err := json.Unmarshal(raw[targetKey], &details); err == nil {
					h.Details = details
				}
			}
		}
	}

	return nil
}
