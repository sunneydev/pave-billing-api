package workflow

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"
	"github.com/sunneydev/pave-billing-api/bills/config"
	"github.com/sunneydev/pave-billing-api/bills/money"
	"go.temporal.io/sdk/testsuite"
)

type BillingWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *BillingWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *BillingWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func TestBillingWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(BillingWorkflowTestSuite))
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_BasicExecution() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(currency, bill.Currency)
	s.Equal(BillStatusClosed, bill.Status)
	s.NotNil(bill.ClosedAt)
	s.Len(bill.LineItems, 0)
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_AddLineItems() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	amount1, _ := money.NewFromString("10.00", currency)
	amount2, _ := money.NewFromString("15.50", currency)

	item1 := LineItem{
		ID:        "item-1",
		Amount:    amount1,
		CreatedAt: time.Now().UTC(),
	}

	item2 := LineItem{
		ID:        "item-2",
		Amount:    amount2,
		CreatedAt: time.Now().UTC(),
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item1)
	}, time.Second)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item2)
	}, time.Second*2)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(currency, bill.Currency)
	s.Equal(BillStatusClosed, bill.Status)
	s.NotNil(bill.ClosedAt)
	s.Len(bill.LineItems, 2)

	expectedTotal, _ := item1.Amount.Add(item2.Amount)
	s.Equal(expectedTotal, bill.Total)
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_GELCurrencyConversion() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	gelAmount, _ := money.NewFromString("100.00", money.GEL)
	gelItem := LineItem{
		ID:        "gel-item",
		Amount:    gelAmount,
		CreatedAt: time.Now().UTC(),
	}

	expectedUsdAmount, _ := money.NewFromString("36.01", money.USD)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, gelItem)
	}, time.Second)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(currency, bill.Currency)
	s.Len(bill.LineItems, 1)

	s.Equal(expectedUsdAmount, bill.LineItems[0].Amount)
	s.Equal(expectedUsdAmount, bill.Total)
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_CloseBill() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	closedAt := time.Now().UTC()
	closeSignal := CloseBillSignal{
		ClosedAt: closedAt,
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalCloseBill, closeSignal)
	}, time.Second)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(currency, bill.Currency)
	s.Equal(BillStatusClosed, bill.Status)
	s.NotNil(bill.ClosedAt)
	s.Equal(closedAt, *bill.ClosedAt)
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_IgnoreItemsAfterClose() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	amount1, _ := money.NewFromString("10.00", currency)
	amount2, _ := money.NewFromString("15.50", currency)

	item1 := LineItem{
		ID:        "item-1",
		Amount:    amount1,
		CreatedAt: time.Now().UTC(),
	}

	item2 := LineItem{
		ID:        "item-2",
		Amount:    amount2,
		CreatedAt: time.Now().UTC(),
	}

	closedAt := time.Now().UTC()
	closeSignal := CloseBillSignal{
		ClosedAt: closedAt,
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item1)
	}, time.Second)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalCloseBill, closeSignal)
	}, time.Second*2)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item2)
	}, time.Second*3)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(currency, bill.Currency)
	s.Equal(BillStatusClosed, bill.Status)
	s.NotNil(bill.ClosedAt)
	s.Len(bill.LineItems, 1)
	s.Equal(item1.Amount, bill.Total)
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_BillingPeriodTimeout() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	amount1, _ := money.NewFromString("10.00", currency)
	item1 := LineItem{
		ID:        "item-1",
		Amount:    amount1,
		CreatedAt: time.Now().UTC(),
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item1)
	}, time.Second)

	s.env.RegisterDelayedCallback(func() {
		var bill *Bill
		result, err := s.env.QueryWorkflow(QueryGetBill)
		s.NoError(err)
		s.NoError(result.Get(&bill))

		s.Equal(BillStatusOpen, bill.Status)
		s.Nil(bill.ClosedAt)
	}, time.Hour*24*15)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(BillStatusClosed, bill.Status)
	s.NotNil(bill.ClosedAt)
	s.Len(bill.LineItems, 1)
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_CloseAlreadyClosedBill() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	firstCloseAt := time.Now().UTC()
	firstCloseSignal := CloseBillSignal{
		ClosedAt: firstCloseAt,
	}

	secondCloseAt := time.Now().UTC().Add(time.Hour)
	secondCloseSignal := CloseBillSignal{
		ClosedAt: secondCloseAt,
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalCloseBill, firstCloseSignal)
	}, time.Second)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalCloseBill, secondCloseSignal)
	}, time.Second*2)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(BillStatusClosed, bill.Status)
	s.NotNil(bill.ClosedAt)
	s.Equal(firstCloseAt, *bill.ClosedAt)
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_UnsupportedCurrency() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	jsonData := []byte(`"€100.00"`)
	var unsupportedAmount money.Money
	err := json.Unmarshal(jsonData, &unsupportedAmount)
	s.Error(err)

	usdAmount, _ := money.NewFromString("100.00", money.USD)

	item := LineItem{
		ID:        "usd-currency-item",
		Amount:    usdAmount,
		CreatedAt: time.Now().UTC(),
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item)
	}, time.Second)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(BillStatusClosed, bill.Status)
	s.Len(bill.LineItems, 1)
	s.Equal("usd-currency-item", bill.LineItems[0].ID)
	s.Equal(usdAmount, bill.LineItems[0].Amount)
	s.Equal(usdAmount, bill.Total)
	s.Equal("$100.00", bill.Total.String())

	origRates := config.Rates
	defer func() { config.Rates = origRates }()

	customRates := &money.ExchangeRates{
		USDToGEL: decimal.NewFromFloat(2.7777),
		GELToUSD: decimal.NewFromFloat(0.3601),
	}
	config.Rates = customRates

	gelAmount, _ := money.NewFromString("200.00", money.GEL)

	_, convErr := gelAmount.ConvertTo(money.Currency("EUR"), config.Rates)
	s.Error(convErr)
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_NegativeAmount() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	_, err := money.NewFromString("-10.00", currency)
	s.Error(err)

	validAmount, _ := money.NewFromString("10.00", currency)

	item := LineItem{
		ID:        "valid-item",
		Amount:    validAmount,
		CreatedAt: time.Now().UTC(),
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item)
	}, time.Second)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(BillStatusClosed, bill.Status)
	s.Len(bill.LineItems, 1)
	s.Equal(validAmount, bill.LineItems[0].Amount)
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_MultipleItemsWithDifferentCurrencies() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	usdAmount1, _ := money.NewFromString("10.00", money.USD)
	usdAmount2, _ := money.NewFromString("20.00", money.USD)
	gelAmount1, _ := money.NewFromString("50.00", money.GEL)
	gelAmount2, _ := money.NewFromString("100.00", money.GEL)

	expectedGelToUsd1, _ := money.NewFromString("18.01", money.USD)
	expectedGelToUsd2, _ := money.NewFromString("36.01", money.USD)

	usdItem1 := LineItem{
		ID:        "usd-item-1",
		Amount:    usdAmount1,
		CreatedAt: time.Now().UTC(),
	}

	gelItem1 := LineItem{
		ID:        "gel-item-1",
		Amount:    gelAmount1,
		CreatedAt: time.Now().UTC().Add(time.Minute),
	}

	usdItem2 := LineItem{
		ID:        "usd-item-2",
		Amount:    usdAmount2,
		CreatedAt: time.Now().UTC().Add(time.Minute * 2),
	}

	gelItem2 := LineItem{
		ID:        "gel-item-2",
		Amount:    gelAmount2,
		CreatedAt: time.Now().UTC().Add(time.Minute * 3),
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, usdItem1)
	}, time.Second)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, gelItem1)
	}, time.Second*2)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, usdItem2)
	}, time.Second*3)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, gelItem2)
	}, time.Second*4)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(BillStatusClosed, bill.Status)
	s.Len(bill.LineItems, 4)

	expectedTotal, _ := money.NewFromString("84.02", money.USD)

	s.Equal(expectedTotal, bill.Total)

	s.Equal(usdAmount1, bill.LineItems[0].Amount)
	s.Equal(expectedGelToUsd1, bill.LineItems[1].Amount)
	s.Equal(usdAmount2, bill.LineItems[2].Amount)
	s.Equal(expectedGelToUsd2, bill.LineItems[3].Amount)
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_ZeroAmountLineItems() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	zeroAmount, _ := money.NewFromString("0.00", currency)
	normalAmount, _ := money.NewFromString("10.00", currency)

	zeroItem := LineItem{
		ID:        "zero-item",
		Amount:    zeroAmount,
		CreatedAt: time.Now().UTC(),
	}

	normalItem := LineItem{
		ID:        "normal-item",
		Amount:    normalAmount,
		CreatedAt: time.Now().UTC().Add(time.Minute),
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, zeroItem)
	}, time.Second)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, normalItem)
	}, time.Second*2)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(BillStatusClosed, bill.Status)
	s.Len(bill.LineItems, 2)

	s.Equal(normalAmount, bill.Total)
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_DuplicateLineItemIDs() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	amount1, _ := money.NewFromString("10.00", currency)
	amount2, _ := money.NewFromString("20.00", currency)

	item1 := LineItem{
		ID:        "duplicate-id",
		Amount:    amount1,
		CreatedAt: time.Now().UTC(),
	}

	item2 := LineItem{
		ID:        "duplicate-id",
		Amount:    amount2,
		CreatedAt: time.Now().UTC().Add(time.Minute),
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item1)
	}, time.Second)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item2)
	}, time.Second*2)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(BillStatusClosed, bill.Status)
	s.Len(bill.LineItems, 2)

	expectedTotal, _ := amount1.Add(amount2)
	s.Equal(expectedTotal, bill.Total)
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_HighPrecisionAmounts() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	amount1, _ := money.NewFromString("0.01", currency)
	amount2, _ := money.NewFromString("0.02", currency)
	amount3, _ := money.NewFromString("0.001", currency)
	amount4, _ := money.NewFromString("0.005", currency)
	amount5, _ := money.NewFromString("0.009", currency)

	s.Equal("$0.01", amount1.String())
	s.Equal("$0.02", amount2.String())
	s.Equal("$0.00", amount3.String())
	s.Equal("$0.01", amount4.String())
	s.Equal("$0.01", amount5.String())

	item1 := LineItem{
		ID:        "precision-item-1",
		Amount:    amount1,
		CreatedAt: time.Now().UTC(),
	}

	item2 := LineItem{
		ID:        "precision-item-2",
		Amount:    amount2,
		CreatedAt: time.Now().UTC().Add(time.Minute),
	}

	item3 := LineItem{
		ID:        "precision-item-3",
		Amount:    amount3,
		CreatedAt: time.Now().UTC().Add(time.Minute * 2),
	}

	item4 := LineItem{
		ID:        "precision-item-4",
		Amount:    amount4,
		CreatedAt: time.Now().UTC().Add(time.Minute * 3),
	}

	item5 := LineItem{
		ID:        "precision-item-5",
		Amount:    amount5,
		CreatedAt: time.Now().UTC().Add(time.Minute * 4),
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item1)
	}, time.Second)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item2)
	}, time.Second*2)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item3)
	}, time.Second*3)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item4)
	}, time.Second*4)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item5)
	}, time.Second*5)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(BillStatusClosed, bill.Status)
	s.Len(bill.LineItems, 5)

	expectedTotal, _ := money.NewFromString("0.05", currency)
	s.Equal(expectedTotal, bill.Total)
	s.Equal("$0.05", bill.Total.String())
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_LargeNumberOfLineItems() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	numItems := 100
	baseAmount, _ := money.NewFromString("1.00", currency)

	for i := 0; i < numItems; i++ {
		itemID := fmt.Sprintf("item-%d", i)
		item := LineItem{
			ID:        itemID,
			Amount:    baseAmount,
			CreatedAt: time.Now().UTC().Add(time.Duration(i) * time.Minute),
		}

		delay := time.Duration(i+1) * time.Millisecond * 10
		s.env.RegisterDelayedCallback(func() {
			s.env.SignalWorkflow(SignalAddLineItem, item)
		}, delay)
	}

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(BillStatusClosed, bill.Status)
	s.Len(bill.LineItems, numItems)

	expectedTotal, _ := money.NewFromString("100.00", currency)
	s.Equal(expectedTotal, bill.Total)
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_AddLineItemAfterClose() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	amount1, _ := money.NewFromString("10.00", currency)
	amount2, _ := money.NewFromString("20.00", currency)

	item1 := LineItem{
		ID:        "item-before-close",
		Amount:    amount1,
		CreatedAt: time.Now().UTC(),
	}

	item2 := LineItem{
		ID:        "item-after-close",
		Amount:    amount2,
		CreatedAt: time.Now().UTC().Add(time.Minute),
	}

	closeTime := time.Now().UTC().Add(time.Minute * 2)
	closeSignal := CloseBillSignal{
		ClosedAt: closeTime,
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item1)
	}, time.Second)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalCloseBill, closeSignal)
	}, time.Second*2)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item2)
	}, time.Second*3)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(BillStatusClosed, bill.Status)
	s.NotNil(bill.ClosedAt)
	s.Equal(closeTime, *bill.ClosedAt)
	s.Len(bill.LineItems, 1)
	s.Equal("item-before-close", bill.LineItems[0].ID)
	s.Equal(amount1, bill.LineItems[0].Amount)
	s.Equal(amount1, bill.Total)
	s.Equal("$10.00", bill.Total.String())
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_CurrencyConversionSuccess() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	usdAmount, _ := money.NewFromString("10.00", money.USD)
	gelAmount, _ := money.NewFromString("100.00", money.GEL)

	expectedGelToUsd, _ := money.NewFromString("36.01", money.USD)

	usdItem := LineItem{
		ID:        "usd-item",
		Amount:    usdAmount,
		CreatedAt: time.Now().UTC(),
	}

	gelItem := LineItem{
		ID:        "gel-item",
		Amount:    gelAmount,
		CreatedAt: time.Now().UTC().Add(time.Minute),
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, usdItem)
	}, time.Second)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, gelItem)
	}, time.Second*2)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(BillStatusClosed, bill.Status)
	s.Len(bill.LineItems, 2)

	s.Equal("usd-item", bill.LineItems[0].ID)
	s.Equal(usdAmount, bill.LineItems[0].Amount)
	s.Equal("gel-item", bill.LineItems[1].ID)
	s.Equal(expectedGelToUsd, bill.LineItems[1].Amount)

	expectedTotal, _ := money.NewFromString("46.01", currency)
	s.Equal(expectedTotal, bill.Total)
	s.Equal("$46.01", bill.Total.String())
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_CurrencyConversionFailure() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	usdAmount, _ := money.NewFromString("10.00", money.USD)
	gelAmount, _ := money.NewFromString("100.00", money.GEL)

	usdItem := LineItem{
		ID:        "usd-item",
		Amount:    usdAmount,
		CreatedAt: time.Now().UTC(),
	}

	gelItem := LineItem{
		ID:        "gel-item",
		Amount:    gelAmount,
		CreatedAt: time.Now().UTC().Add(time.Minute),
	}

	origRates := config.Rates
	defer func() { config.Rates = origRates }()

	config.Rates = &money.ExchangeRates{
		USDToGEL: decimal.NewFromInt(0),
		GELToUSD: decimal.NewFromInt(0),
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, usdItem)
	}, time.Second)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, gelItem)
	}, time.Second*2)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(BillStatusClosed, bill.Status)
	s.Len(bill.LineItems, 2)

	s.Equal("usd-item", bill.LineItems[0].ID)
	s.Equal(usdAmount, bill.LineItems[0].Amount)

	s.Equal("gel-item", bill.LineItems[1].ID)
	zeroAmount, _ := money.NewFromString("0", currency)
	s.Equal(zeroAmount, bill.LineItems[1].Amount)

	s.Equal(usdAmount, bill.Total)
	s.Equal("$10.00", bill.Total.String())
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_LineItemCurrencyMismatch() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	usdAmount, _ := money.NewFromString("10.00", money.USD)
	gelAmount, _ := money.NewFromString("50.00", money.GEL)

	expectedGelToUsd, _ := money.NewFromString("18.01", money.USD)

	usdItem := LineItem{
		ID:        "usd-item",
		Amount:    usdAmount,
		CreatedAt: time.Now().UTC(),
	}

	gelItem := LineItem{
		ID:        "gel-item",
		Amount:    gelAmount,
		CreatedAt: time.Now().UTC().Add(time.Minute),
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, usdItem)
	}, time.Second)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, gelItem)
	}, time.Second*2)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(BillStatusClosed, bill.Status)
	s.Len(bill.LineItems, 2)

	s.Equal("usd-item", bill.LineItems[0].ID)
	s.Equal(usdAmount, bill.LineItems[0].Amount)
	s.Equal(money.USD, bill.LineItems[0].Amount.Currency)

	s.Equal("gel-item", bill.LineItems[1].ID)
	s.Equal(expectedGelToUsd, bill.LineItems[1].Amount)
	s.Equal(money.USD, bill.LineItems[1].Amount.Currency)

	expectedTotal, _ := money.NewFromString("28.01", currency)
	s.Equal(expectedTotal, bill.Total)
	s.Equal("$28.01", bill.Total.String())
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_LineItemCurrencyMismatch_GELBill() {

	s.SetupTest()

	gelBillID := "gel-bill-123"
	customerID := 456
	gelCurrency := money.GEL

	usdAmount, _ := money.NewFromString("10.00", money.USD)

	usdItem := LineItem{
		ID:        "usd-item",
		Amount:    usdAmount,
		CreatedAt: time.Now().UTC(),
	}

	expectedUsdToGel, _ := money.NewFromString("27.78", money.GEL)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, usdItem)
	}, time.Second)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, gelBillID, customerID, gelCurrency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var gelBill *Bill
	gelResult, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(gelResult.Get(&gelBill))

	s.Equal(gelBillID, gelBill.ID)
	s.Equal(customerID, gelBill.CustomerID)
	s.Equal(gelCurrency, gelBill.Currency)
	s.Equal(BillStatusClosed, gelBill.Status)
	s.Len(gelBill.LineItems, 1)

	s.Equal("usd-item", gelBill.LineItems[0].ID)
	s.Equal(expectedUsdToGel, gelBill.LineItems[0].Amount)
	s.Equal(money.GEL, gelBill.LineItems[0].Amount.Currency)
	s.Equal(expectedUsdToGel, gelBill.Total)
	s.Equal("₾27.78", gelBill.Total.String())
}

func (s *BillingWorkflowTestSuite) Test_BillingPeriodWorkflow_TotalCalculationOverflow() {
	billID := "bill-123"
	customerID := 456
	currency := money.USD

	largeAmount, _ := money.NewFromString("9999999999999999.99", currency)

	anotherLargeAmount, _ := money.NewFromString("9999999999999999.99", currency)

	item1 := LineItem{
		ID:        "large-amount-item-1",
		Amount:    largeAmount,
		CreatedAt: time.Now().UTC(),
	}

	item2 := LineItem{
		ID:        "large-amount-item-2",
		Amount:    anotherLargeAmount,
		CreatedAt: time.Now().UTC().Add(time.Minute),
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item1)
	}, time.Second)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, item2)
	}, time.Second*2)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(BillStatusClosed, bill.Status)
	s.Len(bill.LineItems, 2)

	expectedDecimal := decimal.NewFromFloat(9999999999999999.99).Add(decimal.NewFromFloat(9999999999999999.99))

	actualDecimal := bill.Total.Amount()
	s.True(actualDecimal.GreaterThanOrEqual(expectedDecimal.Sub(decimal.NewFromFloat(0.1))),
		"Total should be approximately equal to expected value")
	s.True(actualDecimal.LessThanOrEqual(expectedDecimal.Add(decimal.NewFromFloat(0.1))),
		"Total should be approximately equal to expected value")

	s.Equal("$20000000000000000.00", bill.Total.String())
}
