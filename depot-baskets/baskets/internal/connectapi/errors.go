package connectapi

import (
	"errors"

	"connectrpc.com/connect"
	"github.com/dupakarovsky/eda-storefront/baskets/internal/catalog"
	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
)

func mapError(err error) error {

	switch {
	// NOT FOUND domain errors are mapped to connect CodeNotFound
	// rpc error
	case errors.Is(err, domain.ErrBasketNotFound),
		errors.Is(err, catalog.ErrProductNotFound):
		return connect.NewError(connect.CodeNotFound, err)

	// Invalid codes from the domain are mapped to the RPCs
	// CodeInvalidArgument type error
	case errors.Is(err, domain.ErrProductIDIsBlank),
		errors.Is(err, domain.ErrProductNotInBasket),
		errors.Is(err, domain.ErrBasketNotOpen),
		errors.Is(err, domain.ErrBasketAlreadyCheckedOut),
		errors.Is(err, domain.ErrBasketAlreadyCanceled),
		errors.Is(err, domain.ErrBasketIsEmpty):
		return connect.NewError(connect.CodeInvalidArgument, err)

	// any unknown error is mapped to CodeInternal error
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
