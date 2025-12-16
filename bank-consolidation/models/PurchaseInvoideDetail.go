package models

type PurchaseInvoiceDetail struct {
	PurchaseInvoiceDetailID int64                 `json:"purchaseInvoiceDetailId"`
	PurchaseInvoiceHeader   PurchaseInvoiceHeader `json:"purchaseInvoiceHeader"`
	ProductID               string                `json:"productId"`
	ProductCode             string                `json:"productCode"`
	ProductName             string                `json:"productName"`
	UomPackingID            string                `json:"uomPackingId"`
	UomPackingName          string                `json:"uomPackingName"`
	Qty                     int64                 `json:"qty"`
	ExpDate                 string                `json:"expDate"`
	BatchCode               string                `json:"batchCode"`
	ProductUnitPrice        float64               `json:"productUnitPrice"`
	Discount                float64               `json:"discount"`
	DiscProduct             string                `json:"discProduct"`
	Amount                  float64               `json:"amount"`
	Ppn                     float64               `json:"ppn"`
	PpnInPercent            float64               `json:"ppnInPercent"`
	Notes                   string                `json:"notes"`
}
