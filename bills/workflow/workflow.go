package workflow

import (
	"fmt"
	"time"

	"github.com/sunneydev/pave-billing-api/bills/money"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func BillingPeriodWorkflow(ctx workflow.Context, billID string, customerID int, currency money.Currency) error {
	logger := workflow.GetLogger(ctx)

	bill := &Bill{
		ID:         billID,
		CustomerID: customerID,
		Currency:   currency,
		Status:     BillStatusOpen,
		CreatedAt:  workflow.Now(ctx).UTC(),
		LineItems:  make([]LineItem, 0),
		Total:      money.New(money.ZeroAmount(), currency),
	}

	err := workflow.SetQueryHandler(ctx, QueryGetBill, func() (*Bill, error) {
		return bill, nil
	})

	if err != nil {
		return fmt.Errorf("failed to register query handler: %v", err)
	}

	billingPeriodTimeout := workflow.NewTimer(ctx, calculateTimeUntilNextMonth())
	addItemChan := workflow.GetSignalChannel(ctx, SignalAddLineItem)
	closeChan := workflow.GetSignalChannel(ctx, SignalCloseBill)

	for {
		selector := workflow.NewSelector(ctx)

		selector.AddReceive(addItemChan, func(ch workflow.ReceiveChannel, more bool) {
			var lineItem LineItem
			ch.Receive(ctx, &lineItem)

			if bill.Status == BillStatusClosed {
				logger.Warn("ignoring line item for closed bill", "bill_id", bill.ID)
				return
			}

			bill.LineItems = append(bill.LineItems, lineItem)

			// error occurs only if currencies are different,
			// which are we already handle in service.go
			newTotal, err := bill.Total.Add(lineItem.Amount)
			if err != nil {
				logger.Error("failed to add line item amount", "error", err)
				return
			}

			bill.Total = newTotal

			logger.Info("added line item", "bill_id", bill.ID)
		})

		selector.AddReceive(closeChan, func(ch workflow.ReceiveChannel, more bool) {
			var signal CloseBillSignal
			ch.Receive(ctx, &signal)

			if bill.Status == BillStatusClosed {
				logger.Warn("tried to close already closed bill", "bill_id", bill.ID)
				return
			}

			bill.Status = BillStatusClosed
			bill.ClosedAt = &signal.ClosedAt
			logger.Info("closed bill", "bill_id", bill.ID)

			sendEmailNotification(ctx, bill)
		})

		selector.AddFuture(billingPeriodTimeout, func(f workflow.Future) {
			if bill.Status == BillStatusClosed {
				return
			}

			f.Get(ctx, nil)

			now := workflow.Now(ctx).UTC()
			bill.Status = BillStatusClosed
			bill.ClosedAt = &now
			logger.Info("auto-closed bill due to billing period end", "bill_id", bill.ID)

			sendEmailNotification(ctx, bill)
		})

		selector.Select(ctx)

		if bill.Status == BillStatusClosed {
			break
		}
	}

	return nil
}

func sendEmailNotification(ctx workflow.Context, bill *Bill) {
	logger := workflow.GetLogger(ctx)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 5,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute * 5,
			MaximumAttempts:    5,
		},
	}

	activityCtx := workflow.WithActivityOptions(ctx, activityOptions)

	emailDetails := EmailDetails{
		Bill: bill,
	}

	err := workflow.ExecuteActivity(activityCtx, SendBillClosedEmail, emailDetails).Get(activityCtx, nil)
	if err != nil {
		logger.Error("Failed to send bill closed email",
			"error_type", "EMAIL_SERVICE_ERROR",
			"bill_id", bill.ID,
			"customer_id", bill.CustomerID,
			"error", err)
	}
}
