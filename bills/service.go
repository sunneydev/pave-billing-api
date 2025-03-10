package bill

import (
	"context"
	"fmt"
	"time"

	"encore.dev/rlog"
	"github.com/google/uuid"
	"github.com/sunneydev/pave-billing-api/bills/config"
	"github.com/sunneydev/pave-billing-api/bills/errors"
	"github.com/sunneydev/pave-billing-api/bills/money"
	"github.com/sunneydev/pave-billing-api/bills/workflow"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	temporalworker "go.temporal.io/sdk/worker"
)

//encore:service
type Service struct {
	temporalClient client.Client
	worker         temporalworker.Worker
}

func initService() (service *Service, err error) {
	temporalClient, err := client.NewLazyClient(client.Options{HostPort: config.TemporalServerURL})
	if err != nil {
		err = fmt.Errorf("failed to create Temporal client: %v", err)
		return
	}

	worker := temporalworker.New(temporalClient, config.BillingTaskQueue, temporalworker.Options{})

	worker.RegisterWorkflow(workflow.BillingPeriodWorkflow)

	if err = worker.Start(); err != nil {
		err = fmt.Errorf("failed to start worker: %v", err)
		return
	}

	service = &Service{temporalClient: temporalClient, worker: worker}

	return
}

func (s *Service) Shutdown(force context.Context) {
	s.temporalClient.Close()
	s.worker.Stop()
}

// CreateBill creates a new bill for a customer.
//
//encore:api public method=POST path=/bills
func (s *Service) CreateBill(ctx context.Context, params *CreateBillParams) (bill *workflow.Bill, err error) {
	if err = params.Validate(); err != nil {
		return
	}

	billID := uuid.New().String()
	_, err = s.temporalClient.ExecuteWorkflow(
		ctx,
		client.StartWorkflowOptions{
			ID:                       billID,
			TaskQueue:                config.BillingTaskQueue,
			SearchAttributes:         map[string]interface{}{"CustomerID": params.CustomerID},
			WorkflowExecutionTimeout: time.Hour * 24 * 30,
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval:    time.Second,
				BackoffCoefficient: 2.0,
				MaximumInterval:    time.Minute,
				MaximumAttempts:    5,
			},
		},
		workflow.BillingPeriodWorkflow,
		billID,
		params.CustomerID,
		params.Currency,
	)

	if err != nil {
		err = errors.SafeInternalError(err, "failed to start workflow")
		return
	}

	return s.getBill(ctx, billID, params.CustomerID)
}

// AddLineItem adds a line item to a bill.
//
//encore:api public method=POST path=/bills/:billID/items
func (s *Service) AddLineItem(ctx context.Context, billID string, params *AddLineItemParams) (bill *workflow.Bill, err error) {
	if err = params.Validate(); err != nil {
		return
	}

	bill, err = s.getBill(ctx, billID, params.CustomerID)
	if err != nil {
		return
	}

	if bill.Status == workflow.BillStatusClosed {
		err = errors.BadRequestError("bill is closed")
		return
	}

	amount, err := money.NewFromString(params.Amount, params.Currency)
	if err != nil {
		err = errors.BadRequestError("invalid amount or currency")
		return
	}

	var processedAmount money.Money

	if amount.Currency != bill.Currency {
		processedAmount, err = amount.ConvertTo(bill.Currency, config.Rates)
		if err != nil {
			err = errors.BadRequestError("invalid amount or currency")
			return
		}
	} else {
		processedAmount = amount
	}

	lineItem := workflow.LineItem{
		ID:        uuid.New().String(),
		Amount:    processedAmount,
		CreatedAt: time.Now().UTC(),
	}

	err = s.temporalClient.SignalWorkflow(ctx, billID, "", workflow.SignalAddLineItem, lineItem)

	if err != nil {
		err = errors.SafeInternalError(err, "failed to add line item")
		return
	}

	return s.getBill(ctx, billID, params.CustomerID)
}

// CloseBill closes a bill so no more items can be added.
//
//encore:api public method=POST path=/bills/:billID/close
func (s *Service) CloseBill(ctx context.Context, billID string, params *CloseBillParams) (bill *workflow.Bill, err error) {
	bill, err = s.getBill(ctx, billID, params.CustomerID)
	if err != nil {
		return
	}

	if bill.Status == workflow.BillStatusClosed {
		err = errors.BadRequestError("bill is already closed")
		return
	}

	now := time.Now().UTC()

	signal := workflow.CloseBillSignal{ClosedAt: now}

	err = s.temporalClient.SignalWorkflow(ctx, billID, "", workflow.SignalCloseBill, signal)
	if err != nil {
		err = errors.SafeInternalError(err, "failed to close bill")
		return
	}

	return s.getBill(ctx, billID, params.CustomerID)
}

// GetBill retrieves a bill by ID.
//
//encore:api public method=GET path=/bills/:billID
func (s *Service) GetBill(ctx context.Context, billID string, params *GetBillParams) (*workflow.Bill, error) {
	return s.getBill(ctx, billID, params.CustomerID)
}

// getBill is an internal helper to retrieve a bill by ID and customer ID.
func (s *Service) getBill(ctx context.Context, billID string, customerID int) (bill *workflow.Bill, err error) {
	descResp, err := s.temporalClient.DescribeWorkflowExecution(ctx, billID, "")
	if err != nil {
		if _, ok := err.(*serviceerror.NotFound); ok {
			err = errors.NotFoundError(err, "bill")
		} else {
			err = errors.SafeInternalError(err, "failed to describe workflow")
		}
		return
	}

	if descResp.WorkflowExecutionInfo.Status != enums.WORKFLOW_EXECUTION_STATUS_RUNNING {
		err = errors.SafeInternalError(nil, "workflow is not active")
		return
	}

	resp, err := s.temporalClient.QueryWorkflow(ctx, billID, "", workflow.QueryGetBill)
	if err != nil {
		switch err.(type) {
		case *serviceerror.NotFound:
			err = errors.NotFoundError(err, "bill")
		default:
			err = errors.SafeInternalError(err, "failed to query workflow")
		}

		return
	}

	if err = resp.Get(&bill); err != nil {
		err = errors.SafeInternalError(err, "failed to process bill")
		return
	}

	if bill.CustomerID != customerID {
		// not found error instead of unauthorized
		// prevents leaking information about potential existant bill id
		// note: does not prevent constant timing attacks in a real-world scenario
		err = errors.NotFoundError(err, "bill")
		return
	}

	return
}

// ListBills lists all bills for a customer.
//
//encore:api public method=GET path=/bills
func (s *Service) ListBills(ctx context.Context, params *ListBillsParams) (response *ListBillsResponse, err error) {
	query := "WorkflowType = 'BillingPeriodWorkflow'"
	if params.CustomerID != 0 {
		query += fmt.Sprintf(" AND CustomerID = %d", params.CustomerID)
	}

	resp, err := s.temporalClient.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
		Query: query,
	})

	if err != nil {
		return nil, errors.SafeInternalError(err, "failed to list workflows")
	}

	response = &ListBillsResponse{
		Bills: make([]*workflow.Bill, 0, len(resp.Executions)),
	}

	for _, execution := range resp.Executions {
		bill, err := s.getBill(ctx, execution.Execution.WorkflowId, params.CustomerID)
		if err != nil {
			rlog.Error("failed to get bill",
				"error", err,
				"workflow_id", execution.Execution.WorkflowId,
			)

			continue
		}

		// filter by status if provided
		if params.Status != "" && bill.Status != workflow.BillStatus(params.Status) {
			continue
		}

		response.Bills = append(response.Bills, bill)
	}

	return response, nil
}
