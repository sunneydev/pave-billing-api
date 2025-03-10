package config

import (
	"github.com/shopspring/decimal"
	"github.com/sunneydev/pave-billing-api/bills/money"
)

var (
	Rates             = &money.ExchangeRates{USDToGEL: decimal.NewFromFloat(2.7777), GELToUSD: decimal.NewFromFloat(0.3601)}
	TemporalServerURL = "127.0.0.1:7233"
	BillingTaskQueue  = "billing-task-queue"
)
