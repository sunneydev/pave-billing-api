package money

import "github.com/shopspring/decimal"

type ExchangeRates struct {
	USDToGEL decimal.Decimal
	GELToUSD decimal.Decimal
}

type Currency string

const (
	USD Currency = "USD"
	GEL Currency = "GEL"
)
