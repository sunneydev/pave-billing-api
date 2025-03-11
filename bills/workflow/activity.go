package workflow

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
)

type EmailDetails struct {
	Bill *Bill
}

func SendBillClosedEmail(ctx context.Context, details EmailDetails) error {
	logger := activity.GetLogger(ctx)

	msg := fmt.Sprintf(`
Dear Customer #%d,

Your bill #%s has been closed on %s.

Total: %s
`,
		details.Bill.CustomerID,
		details.Bill.ID,
		details.Bill.ClosedAt.Format("January 2, 2006"),
		details.Bill.Total.String())

	for i := 0; i < len(details.Bill.LineItems); i++ {
		msg += fmt.Sprintf(`
Line Item #%d:
Amount: %s
Currency: %s
Created At: %s
`,
			i+1,
			details.Bill.LineItems[i].Amount.String(),
			details.Bill.LineItems[i].Amount.Currency,
			details.Bill.LineItems[i].CreatedAt.Format("January 2, 2006"))
	}

	logger.Info("Sending bill closed email notification",
		"error_type", "EMAIL_SERVICE",
		"customer_id", details.Bill.CustomerID,
		"bill_id", details.Bill.ID,
		"total", details.Bill.Total.String(),
		"message", msg)

	return nil
}
