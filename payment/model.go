package payment

import "time"

type Transaction struct {
	ID        string    `db:"id"`
	OrderID   int64     `db:"order_id"`
	Amount    float64   `db:"amount"`
	CardLast4 string    `db:"card_last4"`
	Status    string    `db:"status"`
	Message   string    `db:"message"`
	CreatedAt time.Time `db:"created_at"`
}

type ChargeRequest struct {
	OrderID    int64   `json:"order_id"`
	Amount     float64 `json:"amount"`
	CardNumber string  `json:"card_number"`
	Expiry     string  `json:"expiry"`
	CVV        string  `json:"cvv"`
}

type ChargeResponse struct {
	TxID    string `json:"tx_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type StatusResponse struct {
	TxID    string  `json:"tx_id"`
	OrderID int64   `json:"order_id"`
	Amount  float64 `json:"amount"`
	Status  string  `json:"status"`
	Message string  `json:"message"`
}
