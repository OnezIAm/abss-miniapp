package models

import "time"

type PurchaseInvoiceTax struct {
	PurchaseInvoiceTaxID  int64                 `json:"purchaseInvoiceTaxId"`
	PurchaseInvoiceHeader PurchaseInvoiceHeader `json:"purchaseInvoiceHeader"`
	InvoiceNoFromSupplier string                `json:"invoiceNoFromSupplier"`
	TaxInvoiceNo          string                `json:"taxInvoiceNo"`
	Dpp                   float64               `json:"dpp"`
	TotalInvoice          float64               `json:"totalInvoice"`
	TaxInvoiceDate        time.Time             `json:"taxInvoiceDate"`
	PaymentDate           time.Time             `json:"paymentDate"`
}
