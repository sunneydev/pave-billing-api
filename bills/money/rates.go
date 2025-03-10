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

func (c Currency) Symbol() string {
	switch c {
	case USD:
		return "$"
	case GEL:
		return "₾"
	default:
		return string(c)
	}
}
