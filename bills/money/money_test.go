package money

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_New_CreatesMoneyWithCorrectCentsAndCurrency(t *testing.T) {
	amount := decimal.NewFromFloat(123.45)
	m := New(amount, USD)

	assert.Equal(t, int64(12345), m.cents)
	assert.Equal(t, USD, m.Currency)
}

func Test_NewFromString_ParsesValidAmountString(t *testing.T) {
	m, err := NewFromString("123.45", USD)

	require.NoError(t, err)
	assert.Equal(t, int64(12345), m.cents)
	assert.Equal(t, USD, m.Currency)
}

func Test_NewFromString_HandlesInvalidAmountString(t *testing.T) {
	_, err := NewFromString("invalid", USD)
	assert.Error(t, err)

	_, err = NewFromString("123.45", "INVALID")
	assert.Error(t, err)

	_, err = NewFromString("-123.45", USD)
	assert.Error(t, err)
}

func Test_Money_Add_CombinesSameMoneyTypes(t *testing.T) {
	m1 := New(decimal.NewFromFloat(100), USD)
	m2 := New(decimal.NewFromFloat(50), USD)

	result, err := m1.Add(m2)
	require.NoError(t, err)
	assert.Equal(t, int64(15000), result.cents)
	assert.Equal(t, USD, result.Currency)
}

func Test_Money_Add_RejectsAddingDifferentCurrencies(t *testing.T) {
	m1 := New(decimal.NewFromFloat(100), USD)
	m2 := New(decimal.NewFromFloat(50), GEL)

	_, err := m1.Add(m2)
	assert.Error(t, err)
}

func Test_Money_Add_HandlesPotentialIntegerOverflow(t *testing.T) {
	m1 := Money{cents: math.MaxInt64 - 100, Currency: USD}
	m2 := New(decimal.NewFromFloat(1), USD)

	result, err := m1.Add(m2)
	require.NoError(t, err)
	assert.Equal(t, int64(math.MaxInt64-100+100), result.cents)
}

func Test_Money_String_FormatsCorrectly(t *testing.T) {
	m := New(decimal.NewFromFloat(123.45), USD)
	assert.Equal(t, "$123.45", m.String())

	m = New(decimal.NewFromFloat(123.45), GEL)
	assert.Equal(t, "₾123.45", m.String())
}

func Test_Money_Validate_ChecksCurrencyValidity(t *testing.T) {
	m := Money{cents: 100, Currency: "INVALID"}
	assert.Error(t, m.validate())

	m = Money{cents: 100, Currency: USD}
	assert.NoError(t, m.validate())

	m = Money{cents: 100, Currency: GEL}
	assert.NoError(t, m.validate())
}

func Test_Money_Validate_ChecksNegativeAmounts(t *testing.T) {
	m := Money{cents: -100, Currency: USD}
	assert.Error(t, m.validate())

	m = Money{cents: 0, Currency: USD}
	assert.NoError(t, m.validate())

	m = Money{cents: 100, Currency: USD}
	assert.NoError(t, m.validate())
}

func Test_Money_ConvertTo_HandlesIdenticalCurrencies(t *testing.T) {
	m := New(decimal.NewFromFloat(100), USD)
	rates := &ExchangeRates{
		USDToGEL: decimal.NewFromFloat(2.5),
		GELToUSD: decimal.NewFromFloat(0.4),
	}

	result, err := m.ConvertTo(USD, rates)
	require.NoError(t, err)
	assert.Equal(t, m.cents, result.cents)
	assert.Equal(t, m.Currency, result.Currency)
}

func Test_Money_ConvertTo_ConvertsUSDToGEL(t *testing.T) {
	m := New(decimal.NewFromFloat(100), USD)
	rates := &ExchangeRates{
		USDToGEL: decimal.NewFromFloat(2.5),
		GELToUSD: decimal.NewFromFloat(0.4),
	}

	result, err := m.ConvertTo(GEL, rates)
	require.NoError(t, err)
	assert.Equal(t, int64(25000), result.cents)
	assert.Equal(t, GEL, result.Currency)
}

func Test_Money_ConvertTo_ConvertsGELToUSD(t *testing.T) {
	m := New(decimal.NewFromFloat(100), GEL)
	rates := &ExchangeRates{
		USDToGEL: decimal.NewFromFloat(2.5),
		GELToUSD: decimal.NewFromFloat(0.4),
	}

	result, err := m.ConvertTo(USD, rates)
	require.NoError(t, err)
	assert.Equal(t, int64(4000), result.cents)
	assert.Equal(t, USD, result.Currency)
}

func Test_Money_ConvertTo_RejectsUnsupportedCurrencyPairs(t *testing.T) {
	m := Money{cents: 10000, Currency: "EUR"}
	rates := &ExchangeRates{
		USDToGEL: decimal.NewFromFloat(2.5),
		GELToUSD: decimal.NewFromFloat(0.4),
	}

	_, err := m.ConvertTo(USD, rates)
	assert.Error(t, err)

	m = New(decimal.NewFromFloat(100), USD)
	_, err = m.ConvertTo("EUR", rates)
	assert.Error(t, err)
}

func Test_Money_ConvertTo_HandlesRoundingEdgeCases(t *testing.T) {
	m := New(decimal.NewFromFloat(0.01), USD)
	rates := &ExchangeRates{
		USDToGEL: decimal.NewFromFloat(2.5),
		GELToUSD: decimal.NewFromFloat(0.4),
	}

	result, err := m.ConvertTo(GEL, rates)
	require.NoError(t, err)
	assert.Equal(t, int64(3), result.cents)
}

func Test_DecimalToCents_ConvertsCorrectly(t *testing.T) {
	tests := []struct {
		amount decimal.Decimal
		want   int64
	}{
		{decimal.NewFromFloat(123.45), 12345},
		{decimal.NewFromFloat(0.01), 1},
		{decimal.NewFromFloat(0), 0},
		{decimal.NewFromFloat(0.001), 0},
		{decimal.NewFromFloat(0.005), 1},
		{decimal.NewFromFloat(0.009), 1},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, decimalToCents(tt.amount))
	}
}

func Test_DecimalToCents_HandlesRounding(t *testing.T) {
	tests := []struct {
		amount decimal.Decimal
		want   int64
	}{
		{decimal.NewFromFloat(0.004), 0},
		{decimal.NewFromFloat(0.005), 1},
		{decimal.NewFromFloat(0.994), 99},
		{decimal.NewFromFloat(0.995), 100},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, decimalToCents(tt.amount))
	}
}

func Test_CentsToDecimal_ConvertsCorrectly(t *testing.T) {
	tests := []struct {
		cents int64
		want  decimal.Decimal
	}{
		{12345, decimal.NewFromFloat(123.45)},
		{1, decimal.NewFromFloat(0.01)},
		{0, decimal.NewFromFloat(0)},
	}

	for _, tt := range tests {
		assert.True(t, tt.want.Equal(centsToDecimal(tt.cents)))
	}
}

func Test_CentsToDecimalString_FormatsWithTwoDecimalPlaces(t *testing.T) {
	tests := []struct {
		cents int64
		want  string
	}{
		{12345, "123.45"},
		{1, "0.01"},
		{0, "0.00"},
		{10, "0.10"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, centsToDecimalString(tt.cents))
	}
}

func Test_ZeroAmount_ReturnsZeroDecimal(t *testing.T) {
	zero := ZeroAmount()
	assert.True(t, zero.Equal(decimal.NewFromInt(0)))
}

func Test_Money_Amount_ReturnsCorrectDecimal(t *testing.T) {
	m := Money{cents: 12345, Currency: USD}
	expected := decimal.NewFromFloat(123.45)
	assert.True(t, expected.Equal(m.Amount()))
}

func Test_Money_FormatWithSymbol_FormatsUSDCorrectly(t *testing.T) {
	m := Money{cents: 12345, Currency: USD}
	assert.Equal(t, "$123.45", m.FormatWithSymbol())
}

func Test_Money_FormatWithSymbol_FormatsGELCorrectly(t *testing.T) {
	m := Money{cents: 12345, Currency: GEL}
	assert.Equal(t, "₾123.45", m.FormatWithSymbol())
}

func Test_Money_MarshalJSON_SerializesWithCurrencySymbol(t *testing.T) {
	m := Money{cents: 12345, Currency: USD}
	data, err := json.Marshal(m)
	require.NoError(t, err)
	assert.Equal(t, `"$123.45"`, string(data))

	m = Money{cents: 12345, Currency: GEL}
	data, err = json.Marshal(m)
	require.NoError(t, err)
	assert.Equal(t, `"₾123.45"`, string(data))
}

func Test_Money_UnmarshalJSON_ParsesUSDFormat(t *testing.T) {
	var m Money
	err := json.Unmarshal([]byte(`"$123.45"`), &m)
	require.NoError(t, err)
	assert.Equal(t, int64(12345), m.cents)
	assert.Equal(t, USD, m.Currency)
}

func Test_Money_UnmarshalJSON_ParsesGELFormat(t *testing.T) {
	var m Money
	err := json.Unmarshal([]byte(`"₾123.45"`), &m)
	require.NoError(t, err)
	assert.Equal(t, int64(12345), m.cents)
	assert.Equal(t, GEL, m.Currency)
}

func Test_Money_UnmarshalJSON_HandlesWhitespace(t *testing.T) {
	var m Money
	err := json.Unmarshal([]byte(`" $123.45 "`), &m)
	require.NoError(t, err)
	assert.Equal(t, int64(12345), m.cents)
	assert.Equal(t, USD, m.Currency)
}

func Test_Money_UnmarshalJSON_RejectsInvalidFormats(t *testing.T) {
	var m Money
	err := json.Unmarshal([]byte(`"invalid"`), &m)
	assert.Error(t, err)

	err = json.Unmarshal([]byte(`"€123.45"`), &m)
	assert.Error(t, err)
}

func Test_Money_ConvertTo_MaintainsPrecisionDuringConversion(t *testing.T) {
	m := New(decimal.NewFromFloat(0.01), USD)
	rates := &ExchangeRates{
		USDToGEL: decimal.NewFromFloat(2.5),
		GELToUSD: decimal.NewFromFloat(0.4),
	}

	result, err := m.ConvertTo(GEL, rates)
	require.NoError(t, err)

	backToUSD, err := result.ConvertTo(USD, rates)
	require.NoError(t, err)

	assert.Equal(t, int64(1), backToUSD.cents)
}

func Test_Money_Add_MaintainsPrecisionDuringAddition(t *testing.T) {
	m1 := New(decimal.NewFromFloat(0.01), USD)
	m2 := New(decimal.NewFromFloat(0.02), USD)

	result, err := m1.Add(m2)
	require.NoError(t, err)
	assert.Equal(t, int64(3), result.cents)
}

func Test_Money_UnmarshalJSON_HandlesEmptyString(t *testing.T) {
	var m Money
	err := json.Unmarshal([]byte(`"$0"`), &m)
	require.NoError(t, err)
	assert.Equal(t, int64(0), m.cents)
	assert.Equal(t, USD, m.Currency)
}

func Test_Money_ConvertTo_HandlesZeroAmounts(t *testing.T) {
	m := New(decimal.NewFromFloat(0), USD)
	rates := &ExchangeRates{
		USDToGEL: decimal.NewFromFloat(2.5),
		GELToUSD: decimal.NewFromFloat(0.4),
	}

	result, err := m.ConvertTo(GEL, rates)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.cents)
	assert.Equal(t, GEL, result.Currency)
}

func Test_Money_UnmarshalJSON_HandlesNegativeValues(t *testing.T) {
	var m Money
	err := json.Unmarshal([]byte(`"-$123.45"`), &m)
	assert.Error(t, err)
}

func Test_Money_ConvertTo_HandlesExtremeExchangeRates(t *testing.T) {
	m := New(decimal.NewFromFloat(100), USD)
	rates := &ExchangeRates{
		USDToGEL: decimal.NewFromFloat(1000000),
		GELToUSD: decimal.NewFromFloat(0.000001),
	}

	result, err := m.ConvertTo(GEL, rates)
	require.NoError(t, err)
	assert.Equal(t, int64(10000000000), result.cents)
	assert.Equal(t, GEL, result.Currency)
}
