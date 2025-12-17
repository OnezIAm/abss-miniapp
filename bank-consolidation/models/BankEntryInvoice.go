package models

import (
	"time"
)

type BankEntryInvoice struct {
	BankEntryID     string    `json:"bankEntryId" gorm:"primaryKey;type:varchar(64)"`
	InvoiceHeaderID string    `json:"invoiceHeaderId" gorm:"primaryKey;type:varchar(64);index"`
	MatchedAmount   float64   `json:"matchedAmount" gorm:"type:decimal(18,2)"`
	Note            string    `json:"note" gorm:"type:text"`
	CreatedAt       time.Time `json:"createdAt" gorm:"type:datetime;not null;default:NOW()"`
}
