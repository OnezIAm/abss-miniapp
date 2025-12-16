package models

type BankEntry struct {
	ID              string  `json:"id"`
	TransactionDate string  `json:"transactionDate"`
	Description     string  `json:"description"`
	Branch          string  `json:"branch"`
	Amount          float64 `json:"amount"`
	AmountType      string  `json:"amountType"`
	Balance         float64 `json:"balance"`
	BankCode        string  `json:"bankCode"`
	AttachedCount   int     `json:"attachedCount"`
	MatchedTotal    float64 `json:"matchedTotal"`
}
