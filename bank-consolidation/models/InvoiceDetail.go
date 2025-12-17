package models

import "gorm.io/gorm"

type InvoiceDetail struct {
	InvoiceDetailID     string         `json:"invoiceDetailId" gorm:"column:id;primaryKey;type:varchar(64)"`
	InvoiceHeaderID     string         `json:"invoiceHeaderId" gorm:"column:header_id;type:varchar(64);not null;index"`
	ProductID           string         `json:"productId" gorm:"column:product_id;type:varchar(64);not null"`
	ProductName         string         `json:"productName" gorm:"column:description;type:varchar(255);not null"`
	UomID               string         `json:"uomId" gorm:"-"`
	PackingName         string         `json:"packingName" gorm:"-"`
	Qty                 float64        `json:"qty" gorm:"column:quantity;type:decimal(15,4);not null"`
	Disc                float64        `json:"disc" gorm:"-"`
	UnitPrice           float64        `json:"unitPrice" gorm:"column:unit_price;type:decimal(15,4);not null"`
	Amount              float64        `json:"amount" gorm:"type:decimal(15,4);not null"`
	Ppn                 float64        `json:"ppn" gorm:"column:tax_amount;type:decimal(15,4);not null"`
	PpnPercent          float64        `json:"ppnPercent" gorm:"column:tax_rate;type:decimal(5,2);not null"`
	ClaimAbleDisc       float64        `json:"claimAbleDisc" gorm:"-"`
	ClaimAbleDiscAmount float64        `json:"claimAbleDiscAmount" gorm:"-"`
	ManualDiscount      float64        `json:"manualDiscount" gorm:"-"`
	DeletedAt           gorm.DeletedAt `json:"-" gorm:"index"`
}
