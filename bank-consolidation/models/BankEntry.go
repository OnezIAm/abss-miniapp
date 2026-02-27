package models

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type BankEntry struct {
	ID              string         `json:"id" gorm:"primaryKey;type:varchar(64)"`
	TransactionDate time.Time      `json:"transactionDate" gorm:"type:datetime;not null"`
	Description     string         `json:"description" gorm:"type:text;not null"`
	Branch          string         `json:"branch" gorm:"type:varchar(32);not null"`
	Amount          float64        `json:"amount" gorm:"type:decimal(18,2);not null"`
	AmountType      string         `json:"amountType" gorm:"type:varchar(2);not null"`
	Balance         float64        `json:"balance" gorm:"type:decimal(18,2);not null"`
	BankCode        string         `json:"bankCode" gorm:"type:varchar(20);not null;default:'UNKNOWN'"`
	Fingerprint     string         `json:"fingerprint" gorm:"type:varchar(64);uniqueIndex"`
	AttachedCount   int            `json:"attachedCount" gorm:"-"`
	MatchedTotal    float64        `json:"matchedTotal" gorm:"-"`
	Delta           float64        `json:"delta" gorm:"-"`
	AttachedInvoices []BankEntryInvoiceSummary `json:"attachedInvoices,omitempty" gorm:"-"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

type BankEntryInvoiceSummary struct {
	ID            string    `json:"id"`
	InvoiceNo     string    `json:"invoiceNo"`
	InvoiceDate   time.Time `json:"invoiceDate"`
	CustomerName  string    `json:"customerName"`
	Status        string    `json:"status"`
	TotalAmount   float64   `json:"totalAmount"`
	MatchedAmount float64   `json:"matchedAmount"`
}

func (b *BankEntry) UnmarshalJSON(data []byte) error {
	type Alias BankEntry
	aux := &struct {
		TransactionDate string `json:"transactionDate"`
		*Alias
	}{
		Alias: (*Alias)(b),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.TransactionDate == "" {
		return nil
	}

	s := strings.TrimSpace(aux.TransactionDate)
	var t time.Time
	var err error

	if strings.Contains(s, "/") {
		t, err = time.Parse("02/01/2006", s)
	} else {
		// try RFC3339 or date-only
		t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			t, err = time.Parse("2006-01-02", s)
		}
	}

	if err != nil {
		return errors.New("unsupported date format")
	}
	b.TransactionDate = t
	return nil
}
