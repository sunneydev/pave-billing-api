package money

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

type Money struct {
	cents    int64
	Currency Currency `json:"currency"`
}

func New(amount decimal.Decimal, currency Currency) Money {
	return Money{
		cents:    decimalToCents(amount),
		Currency: currency,
	}
}

func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("cannot add different currencies: %s and %s", m.Currency, other.Currency)
	}

	return Money{
		cents:    m.cents + other.cents,
		Currency: m.Currency,
	}, nil
}

func (m Money) String() string {
	return m.FormatWithSymbol()
}

func (m Money) validate() error {
	if m.Currency != USD && m.Currency != GEL {
		return fmt.Errorf("invalid currency: %s", m.Currency)
	}

	if m.cents < 0 {
		return fmt.Errorf("amount cannot be negative")
	}

	return nil
}

func NewFromString(amount string, currency Currency) (money Money, err error) {
	decimal, err := decimal.NewFromString(amount)
	if err != nil {
		err = fmt.Errorf("invalid amount format: %w", err)
		return
	}

	money = New(decimal, currency)
	if err = money.validate(); err != nil {
		err = fmt.Errorf("invalid amount: %w", err)
		return
	}

	return
}

func (m Money) ConvertTo(targetCurrency Currency, rates *ExchangeRates) (Money, error) {
	if m.Currency == targetCurrency {
		return m, nil
	}

	var convertedAmount decimal.Decimal
	amount := centsToDecimal(m.cents)

	if m.Currency == USD && targetCurrency == GEL {
		convertedAmount = amount.Mul(rates.USDToGEL)
	} else if m.Currency == GEL && targetCurrency == USD {
		convertedAmount = amount.Mul(rates.GELToUSD)
	} else {
		return Money{}, fmt.Errorf("unsupported currency conversion from %s to %s", m.Currency, targetCurrency)
	}

	return New(convertedAmount, targetCurrency), nil
}

func decimalToCents(amount decimal.Decimal) int64 {
	return amount.Mul(decimal.NewFromInt(100)).Round(0).IntPart()
}

func centsToDecimal(cents int64) decimal.Decimal {
	return decimal.NewFromInt(cents).Div(decimal.NewFromInt(100))
}

func centsToDecimalString(cents int64) string {
	return fmt.Sprintf("%.2f", float64(cents)/100)
}

func ZeroAmount() decimal.Decimal {
	return decimal.NewFromInt(0)
}

func (m Money) Amount() decimal.Decimal {
	return centsToDecimal(m.cents)
}

func (m Money) FormatWithSymbol() string {
	amount := centsToDecimalString(m.cents)
	symbol := m.Currency.Symbol()

	return fmt.Sprintf("%s%s", symbol, amount)
}

func (m Money) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.FormatWithSymbol())
}

func (m *Money) UnmarshalJSON(data []byte) error {
	var valueStr string
	if err := json.Unmarshal(data, &valueStr); err != nil {
		return err
	}

	valueStr = strings.TrimSpace(valueStr)

	if len(valueStr) > 0 {
		if strings.HasPrefix(valueStr, "$") {
			m.Currency = USD
			valueStr = strings.TrimPrefix(valueStr, "$")
		} else if strings.HasPrefix(valueStr, "₾") {
			m.Currency = GEL
			valueStr = strings.TrimPrefix(valueStr, "₾")
		}

		valueStr = strings.TrimSpace(valueStr)
	}

	amount, err := decimal.NewFromString(valueStr)
	if err != nil {
		return fmt.Errorf("invalid amount: %v", err)
	}

	m.cents = decimalToCents(amount)
	return m.validate()
}
