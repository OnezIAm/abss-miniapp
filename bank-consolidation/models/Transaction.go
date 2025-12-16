package models

type Transaction struct {
	ID               string `json:"id"`
	RawCSV           string `json:"rawCsv"`
	ImportSource     string `json:"importSource"`
	ImportTimestamp  string `json:"importTimestamp"`
	ValidationStatus string `json:"validationStatus"`
}
