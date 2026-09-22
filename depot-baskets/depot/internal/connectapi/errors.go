// depot/internal/connectapi/errors.go
package connectapi

import (
	"errors"

	"connectrpc.com/connect"
	"github.com/dupakarovsky/eda-storefront/depot/internal/catalog"
	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
)

func mapError(err error) error {

	switch {
	// NOT FOUND domain errors are mapped to connect CodeNotFound
	// rpc error
	case errors.Is(err, domain.ErrShoppingListNotFound),
		errors.Is(err, catalog.ErrStoreNotFound),
		errors.Is(err, catalog.ErrProductNotFound):
		return connect.NewError(connect.CodeNotFound, err)

	// Invalid codes from the domain are mapped to the RPCs
	// CodeInvalidArgument type error
	case errors.Is(err, domain.ErrStoreIDIsBlank),
		errors.Is(err, domain.ErrProductIDIsBlank),
		errors.Is(err, domain.ErrOrderIDIsBlank),
		errors.Is(err, domain.ErrStopNotFound),
		errors.Is(err, domain.ErrProductNotInStop),
		errors.Is(err, domain.ErrShoppingListNotPending),
		errors.Is(err, domain.ErrShoppingListNotActive),
		errors.Is(err, domain.ErrShoppingListIsEmpty),
		errors.Is(err, domain.ErrShoppingListAlreadyCompleted),
		errors.Is(err, domain.ErrShoppingListAlreadyCanceled):
		return connect.NewError(connect.CodeInvalidArgument, err)

	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
