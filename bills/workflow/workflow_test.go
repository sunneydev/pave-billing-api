package workflow

import (
	"testing"
	"time"

	"encore.app/bills/config"
	"encore.app/bills/money"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"
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

	expectedConvertedAmount, _ := gelItem.Amount.ConvertTo(currency, config.Rates)

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
	s.Equal(expectedConvertedAmount, bill.LineItems[0].Amount)
	s.Equal(expectedConvertedAmount, bill.Total)
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

	item1 := LineItem{
		ID:        "item-1",
		Amount:    amount1,
		CreatedAt: time.Now().UTC(),
	}

	amount2, _ := money.NewFromString("15.50", currency)

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

	// Test with EUR which should not be supported
	eurAmount, _ := money.NewFromString("100.00", money.GEL)
	unsupportedCurrencyItem := LineItem{
		ID:        "eur-item",
		Amount:    eurAmount,
		CreatedAt: time.Now().UTC(),
	}

	// Make sure config.Rates only contains USD and GEL
	origRates := config.Rates
	defer func() { config.Rates = origRates }()
	config.Rates = &money.ExchangeRates{
		USDToGEL: decimal.NewFromFloat(0.38),
		GELToUSD: decimal.NewFromFloat(1.0),
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, unsupportedCurrencyItem)
	}, time.Second)

	s.env.ExecuteWorkflow(BillingPeriodWorkflow, billID, customerID, currency)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var bill *Bill
	result, err := s.env.QueryWorkflow(QueryGetBill)
	s.NoError(err)
	s.NoError(result.Get(&bill))

	// The unsupported currency item should have been skipped
	s.Equal(billID, bill.ID)
	s.Equal(customerID, bill.CustomerID)
	s.Equal(BillStatusClosed, bill.Status)
	s.Len(bill.LineItems, 0)
	// Total should remain zero since the item was skipped
	zeroAmount, _ := money.NewFromString("0", currency)
	s.Equal(zeroAmount, bill.Total)
}
