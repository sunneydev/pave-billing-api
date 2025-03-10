package bill

import (
	"github.com/sunneydev/pave-billing-api/bills/errors"
	"github.com/sunneydev/pave-billing-api/bills/money"
	workflow "github.com/sunneydev/pave-billing-api/bills/workflow"
)

type CreateBillParams struct {
	CustomerID int            `json:"customer_id"`
	Currency   money.Currency `json:"currency"`
}

type ListBillsParams struct {
	CustomerID int    `json:"customer_id" query:"customer_id,omitempty"`
	Status     string `json:"status" query:"status,omitempty"`
}

type AddLineItemParams struct {
	CustomerID int            `json:"customer_id"`
	Amount     string         `json:"amount"`
	Currency   money.Currency `json:"currency"`
}

type CloseBillParams struct {
	CustomerID int `json:"customer_id" query:"customer_id"`
}

type GetBillParams struct {
	CustomerID int `json:"customer_id" query:"customer_id"`
}

type ListBillsResponse struct {
	Bills []*workflow.Bill `json:"bills"`
}

func (p *AddLineItemParams) Validate() (err error) {
	_, err = money.NewFromString(p.Amount, p.Currency)
	if err != nil {
		err = errors.BadRequestError("invalid amount format")
	}

	return
}

func (p *CreateBillParams) Validate() (err error) {
	if p.Currency != money.USD && p.Currency != money.GEL {
		err = errors.BadRequestError("invalid currency")
	}

	return
}
