package controllers

import (
	"encoding/json"
	"net/http"

	"gorm.io/gorm"
)

type ReportsController struct {
	DB *gorm.DB
}

func (c ReportsController) GetInvoices(w http.ResponseWriter, r *http.Request) {
	var list []map[string]any
	if err := c.DB.Table("v_invoice_summary").Order("invoice_date DESC").Find(&list).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var responseList []map[string]any
	for _, item := range list {
		responseList = append(responseList, map[string]any{
			"headerId":     item["header_id"],
			"invoiceNo":    item["invoice_no"],
			"invoiceDate":  item["invoice_date"],
			"customerId":   item["customer_id"],
			"customerName": item["customer_name"],
			"status":       item["status"],
			"totalAmount":  item["total_amount"],
			"totalTax":     item["total_tax"],
			"companyCode":  item["company_code"],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(responseList)
}

func (c ReportsController) GetTransactionCategories(w http.ResponseWriter, r *http.Request) {
	var list []map[string]any
	if err := c.DB.Table("v_transaction_category_summary").Order("transaction_id").Find(&list).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var responseList []map[string]any
	for _, item := range list {
		responseList = append(responseList, map[string]any{
			"transactionId":    item["transaction_id"],
			"importSource":     item["import_source"],
			"validationStatus": item["validation_status"],
			"categoryId":       item["category_id"],
			"categoryType":     item["category_type"],
			"categoryName":     item["category_name"],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(responseList)
}
