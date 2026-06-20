package model

type Payment struct {
	ID       string  `json:"id"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Status   string  `json:"status"`
}