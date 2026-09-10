// baskets/internal/domain/basket_repository.go
package domain

import (
	"context"
	"errors"
)

var ErrBasketNotFound = errors.New("basket not found")

// BasketRepository defines the behavior needed by any concrete type
// to access any or the repositories.
type BasketRepository interface {
	Save(ctx context.Context, basket *Basket) error
	FindByIDForUpdate(ctx context.Context, id string) (*Basket, error)
	WithLock(ctx context.Context, fn func(txCtx context.Context) error) error

	FindByID(ctx context.Context, id string) (*Basket, error)
}
