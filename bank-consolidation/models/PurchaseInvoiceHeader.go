package models

type PurchaseInvoiceStatusType string

const (
	PurchaseInvoiceStatusPending     PurchaseInvoiceStatusType = "PENDING"
	PurchaseInvoiceStatusVerified    PurchaseInvoiceStatusType = "VERIFIED"
	PurchaseInvoiceStatusPaid        PurchaseInvoiceStatusType = "PAID"
	PurchaseInvoiceStatusVoid        PurchaseInvoiceStatusType = "VOID"
	RecreatedPurchaseInvoiceStatus   PurchaseInvoiceStatusType = "RECREATED"
	CollectiblePurchaseInvoiceStatus PurchaseInvoiceStatusType = "COLLECTIBLE"
)

type PaymentStatusType string

const (
	PaymentStatusCash   PaymentStatusType = "CASH"
	PaymentStatusCredit PaymentStatusType = "CREDIT"
)

type PurchaseInvoiceHeader struct {
	PurchaseInvoiceHeaderID  int64                     `json:"purchaseInvoiceHeaderId"`
	PurchaseInvoiceNo        string                    `json:"purchaseInvoiceNo"`
	PurchaseOrderGroupNo     string                    `json:"purchaseOrderGroupNo"`
	PurchaseOrderNo          string                    `json:"purchaseOrderNo"`
	PurchaseInvoiceDate      string                    `json:"purchaseInvoiceDate"`
	SupplierId               string                    `json:"supplierId"`
	SupplierName             string                    `json:"supplierName"`
	SupplierAddress          string                    `json:"supplierAddress"`
	PurchaseInvoiceStatus    PurchaseInvoiceStatusType `json:"purchaseInvoiceStatus"`
	DueDate                  string                    `json:"dueDate"`
	ReceiveDate              string                    `json:"receiveDate"`
	TotalAmount              float64                   `json:"totalAmount"`
	RoundValue               float64                   `json:"roundValue"`
	TotalQty                 float64                   `json:"totalQty"`
	TotalProduct             float64                   `json:"totalProduct"`
	Ppn                      float64                   `json:"ppn"`
	CreatedBy                string                    `json:"createdBy"`
	CreatedDate              string                    `json:"createdDate"`
	UpdatedBy                string                    `json:"updatedBy"`
	UpdatedAt                string                    `json:"updatedAt"`
	Details                  []PurchaseInvoiceDetail   `json:"details"`
	Notes                    string                    `json:"notes"`
	PurchaseInvoiceDisc      string                    `json:"purchaseInvoiceDisc"`
	AdditionalCost           float64                   `json:"additionalCost"`
	PaymentStatus            PaymentStatusType         `json:"paymentStatus"`
	PurchaseInvoiceToID      int64                     `json:"purchaseInvoiceToId"`
	PurchaseInvoiceToName    string                    `json:"purchaseInvoiceToName"`
	PurchaseInvoiceToAddress string                    `json:"purchaseInvoiceToAddress"`
	FromPurchaseOrder        bool                      `json:"fromPurchaseOrder"`
}
