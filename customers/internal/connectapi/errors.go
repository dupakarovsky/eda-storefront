package connectapi

import (
	"errors"

	"connectrpc.com/connect"
	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
)

// mapError will map domain type sentinel Errors to a connect type errors
func mapError(err error) error {
	switch {

	// NOT FOUND domain errors are mapped to connect CodeNotFound
	// rpc error
	case errors.Is(err, domain.ErrCustomerNotFound):
		return connect.NewError(connect.CodeNotFound, err)

	// Invalid codes from the domain are mapped to the RPCs
	// CodeInvalidArgument type error
	case errors.Is(err, domain.ErrCustomerSmsNumberIsBlank),
		errors.Is(err, domain.ErrCustomerSmsNumberUnchanged),
		errors.Is(err, domain.ErrCustomerNameIsBlank):

		return connect.NewError(connect.CodeInvalidArgument, err)

	// For Authorize errors we map to CodePermissionDenied
	case errors.Is(err, domain.ErrCustomerNotAuthorized):

		return connect.NewError(connect.CodePermissionDenied, err)

	// For disable and enable we map to CodeFailedPrecondition as
	// these methods don't accept any arguments.
	case errors.Is(err, domain.ErrCustomerAlreadyDisabled),
		errors.Is(err, domain.ErrCustomerAlreadyEnabled):

		return connect.NewError(connect.CodeFailedPrecondition, err)

	// 500 Errors
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
