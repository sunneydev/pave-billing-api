package workflow

import (
	"time"

	"encore.app/bills/money"
)

type BillStatus string

const (
	BillStatusOpen   BillStatus = "OPEN"
	BillStatusClosed BillStatus = "CLOSED"
)

type Bill struct {
	ID         string         `json:"id"`
	CustomerID int            `json:"customer_id"`
	Status     BillStatus     `json:"status"`
	Currency   money.Currency `json:"currency"`
	CreatedAt  time.Time      `json:"created_at"`
	ClosedAt   *time.Time     `json:"closed_at,omitempty"`
	LineItems  []LineItem     `json:"line_items"`
	Total      money.Money    `json:"total"`
}

type LineItem struct {
	ID        string      `json:"id"`
	Amount    money.Money `json:"amount"`
	CreatedAt time.Time   `json:"created_at"`
}

type CloseBillSignal struct {
	ClosedAt time.Time `json:"closed_at"`
}
