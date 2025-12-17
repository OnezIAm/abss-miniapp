package models

type TransactionCategory struct {
	TransactionID string `json:"transactionId" gorm:"primaryKey;type:varchar(64)"`
	CategoryID    string `json:"categoryId" gorm:"primaryKey;type:varchar(64)"`
}
