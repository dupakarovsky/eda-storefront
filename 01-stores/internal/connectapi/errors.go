package connectapi

// Real error mapping. So far every handler failure has come back as
// connect.CodeInternal — fine for CreateStore, where the only real failure
// modes were Postgres/NATS being down. Rebrand/EnableParticipation/DisableParticipation
// introduce a new failure mode: calling them against a store ID that doesn't exist,
// or toggling participation into a state it's already in — both are domain-level
// rejections (domain.ErrStoreNotFound, domain.ErrStoreIsAlreadyParticipating, etc.),
// not infrastructure failures, and a caller needs to be able to tell those apart
// (a 404/CodeNotFound vs. a 400/CodeInvalidArgument vs. a genuine 500)

import (
	"errors"

	"connectrpc.com/connect"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

// mapError will map returned domain level Errors to the connectRPC error types
func mapError(err error) error {
	switch {

	// domain errors during creation are mapped to CodeNotFound
	case errors.Is(err, domain.ErrStoreNotFound),
		errors.Is(err, domain.ErrProductNotFound):
		return connect.NewError(connect.CodeNotFound, err)

	// remaining domain errors are mapped to CodeInvalidArgument
	case errors.Is(err, domain.ErrStoreNameIsBlank),
		errors.Is(err, domain.ErrStoreLocationIsBlank),
		errors.Is(err, domain.ErrStoreIsAlreadyParticipating),
		errors.Is(err, domain.ErrStoreIsAlreadyNotParticipating):
		return connect.NewError(connect.CodeInvalidArgument, err)

	// connectivity errors are mapped to CodeInternal
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
