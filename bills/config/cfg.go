package config

import (
	"encore.app/bills/money"
	"encore.dev"
	"github.com/shopspring/decimal"
)

var (
	Rates             = &money.ExchangeRates{USDToGEL: decimal.NewFromFloat(2.7777), GELToUSD: decimal.NewFromFloat(0.3601)}
	TemporalServerURL = "127.0.0.1:7233"
	EnvName           = encore.Meta().Environment.Name
	BillingTaskQueue  = EnvName + "-billing"
)
