package connectapi

import (
	"errors"

	"connectrpc.com/connect"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
)

// mapError will map domain type sentinel Errors to a connect type errors
func mapError(err error) error {
	switch {

	// NOT FOUND domain errors are mapped to connect CodeNotFound
	// rpc error
	case errors.Is(err, domain.ErrOrderNotFound):
		return connect.NewError(connect.CodeNotFound, err)

	// Invalid codes from the domain are mapped to the RPCs
	// CodeInvalidArgument type error
	case errors.Is(err, domain.ErrOrderAlreadyReadied),
		errors.Is(err, domain.ErrOrderAlreadyCanceled),
		errors.Is(err, domain.ErrOrderIsEmpty),
		errors.Is(err, domain.ErrShoppingListIDIsBlank),
		errors.Is(err, domain.ErrShoppingListAlreadyLinked),
		errors.Is(err, domain.ErrShoppingListNotLinked):
		return connect.NewError(connect.CodeInvalidArgument, err)

	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
