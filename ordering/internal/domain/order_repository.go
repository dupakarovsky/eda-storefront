package domain

import (
	"context"
	"errors"
)

var ErrOrderNotFound = errors.New("order not found")

// OrderRepository defines the behavior needed by a concrete
// type to communicate with the Postgres database.
type OrderRepository interface {
	Save(ctx context.Context, order *Order) error
	FindByIDForUpdate(ctx context.Context, id string) (*Order, error)
	WithLock(ctx context.Context, fn func(txCtx context.Context) error) error

	FindByID(ctx context.Context, id string) (*Order, error)
	FindByShoppingListID(ctx context.Context, shoppingLisgID string) (*Order, error)
}
