package domain

import (
	"context"
	"errors"
)

var ErrProductNotFound = errors.New("product not found")

// ProductRepository defines the behavior needed for a concrete type
// to persist data in a repository
type ProductRepository interface {
	// write
	Save(ctx context.Context, product *Product) error
	FindByID(ctx context.Context, id string) (*Product, error)
	FindByIDForUpdate(ctx context.Context, id string) (*Product, error)
	Delete(ctx context.Context, id string) error
	WithLock(ctx context.Context, fn func(txCtx context.Context) error) error

	// read
	ListByStoreID(ctx context.Context, storeID string) ([]*Product, error)
}
