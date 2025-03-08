package errors

import (
	"fmt"

	"encore.dev/beta/errs"
	"encore.dev/rlog"
)

// wrap internal error with logging while returning a generic error to the user.
// safe suggests that the error is not user-facing.
func SafeInternalError(err error, msg string) error {
	rlog.Error("internal error", "msg", msg, "error", err)

	return &errs.Error{
		Code:    errs.Internal,
		Message: "internal error occurred",
	}
}

func BadRequestError(msg string) error {
	return &errs.Error{Code: errs.InvalidArgument, Message: msg}
}

func NotFoundError(err error, resource string) error {
	if err != nil {
		rlog.Error("not found error", "resource", resource, "error", err)
	}

	return &errs.Error{
		Code:    errs.NotFound,
		Message: fmt.Sprintf("requested %s was not found", resource),
	}
}
