package workflow

import (
	"fmt"
	"time"

	"github.com/sunneydev/pave-billing-api/bills/money"
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

	billingPeriodTimeout := workflow.NewTimer(ctx, time.Hour*24*30)
	addItemChan := workflow.GetSignalChannel(ctx, SignalAddLineItem)
	closeChan := workflow.GetSignalChannel(ctx, SignalCloseBill)

	for {
		selector := workflow.NewSelector(ctx)

		selector.AddReceive(addItemChan, func(ch workflow.ReceiveChannel, more bool) {
			var lineItem LineItem
			ch.Receive(ctx, &lineItem)

			if bill.Status == BillStatusClosed {
				logger.Info("ignoring line item for closed bill", "bill_id", bill.ID)
				return
			}

			bill.LineItems = append(bill.LineItems, lineItem)

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
				logger.Info("bill already closed", "bill_id", bill.ID)
				return
			}

			bill.Status = BillStatusClosed
			bill.ClosedAt = &signal.ClosedAt
			logger.Info("closed bill", "bill_id", bill.ID)
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
		})

		selector.Select(ctx)

		if bill.Status == BillStatusClosed {
			break
		}
	}

	return nil
}
