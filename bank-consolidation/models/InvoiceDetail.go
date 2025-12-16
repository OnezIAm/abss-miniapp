package models

type InvoiceDetail struct {
	InvoiceDetailID     string  `json:"invoiceDetailId"`
	InvoiceHeaderID     string  `json:"invoiceHeaderId"`
	ProductID           string  `json:"productId"`
	ProductName         string  `json:"productName"`
	UomID               string  `json:"uomId"`
	PackingName         string  `json:"packingName"`
	Qty                 float64 `json:"qty"`
	Disc                float64 `json:"disc"`
	UnitPrice           float64 `json:"unitPrice"`
	Amount              float64 `json:"amount"`
	Ppn                 float64 `json:"ppn"`
	PpnPercent          float64 `json:"ppnPercent"`
	ClaimAbleDisc       float64 `json:"claimAbleDisc"`
	ClaimAbleDiscAmount float64 `json:"claimAbleDiscAmount"`
	ManualDiscount      float64 `json:"manualDiscount"`
}
