package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type BankEntry struct {
	ID               string                    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	TransactionDate  time.Time                 `json:"transactionDate" gorm:"type:datetime;not null"`
	Description      string                    `json:"description" gorm:"type:text;not null"`
	Branch           string                    `json:"branch" gorm:"type:varchar(32);not null"`
	Amount           float64                   `json:"amount" gorm:"type:decimal(18,2);not null"`
	AmountType       string                    `json:"amountType" gorm:"type:varchar(2);not null"`
	Balance          float64                   `json:"balance" gorm:"type:decimal(18,2);not null"`
	BankCode         string                    `json:"bankCode" gorm:"type:varchar(20);not null;default:'UNKNOWN'"`
	IsFinalized      bool                      `json:"isFinalized" gorm:"type:boolean;default:false"`
	Fingerprint      string                    `json:"fingerprint" gorm:"type:varchar(64);uniqueIndex"`
	AttachedCount    int                       `json:"attachedCount" gorm:"-"`
	MatchedTotal     float64                   `json:"matchedTotal" gorm:"-"`
	Delta            float64                   `json:"delta" gorm:"-"`
	AttachedInvoices []BankEntryInvoiceSummary `json:"attachedInvoices,omitempty" gorm:"-"`
	DeletedAt        gorm.DeletedAt            `json:"-" gorm:"index"`
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

	t, err := ParseDate(aux.TransactionDate)
	if err != nil {
		return err
	}
	b.TransactionDate = t
	return nil
}

func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("date string is empty")
	}

	layouts := []string{
		"2006-01-02",
		"02/01/2006",
		time.RFC3339,
		"2006/01/02",
		"02-01-2006",
		"02 Jan 2006",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported date format: %s", s)
}
