package models

type InvoiceHeader struct {
	InvoiceHeaderID   string  `json:"invoiceHeaderId"`
	InvoiceNo         string  `json:"invoiceNo"`
	InvoiceSequenceNo string  `json:"invoiceSequenceNo"`
	InvoiceDate       string  `json:"invoiceDate"`
	TradeDate         string  `json:"tradeDate"`
	SalesOrderID      string  `json:"salesOrderId"`
	SalesOrderNo      string  `json:"salesOrderNo"`
	DeliveryOrderID   string  `json:"deliveryOrderId"`
	DeliveryOrderNo   string  `json:"deliveryOrderNo"`
	PurchaseOrderNo   string  `json:"purchaseOrderNo"`
	SalesID           string  `json:"salesId"`
	SalesName         string  `json:"salesName"`
	CustomerID        string  `json:"customerId"`
	CustomerName      string  `json:"customerName"`
	DeliverTo         string  `json:"deliverTo"`
	TotalAmount       float64 `json:"totalAmount"`
	TotalProduct      int     `json:"totalProduct"`
	DueDate           string  `json:"dueDate"`
	Termin            string  `json:"termin"`
	Notes             string  `json:"notes"`
	CompanyCode       string  `json:"companyCode"`
	OtherExpense      float64 `json:"otherExpense"`
}
