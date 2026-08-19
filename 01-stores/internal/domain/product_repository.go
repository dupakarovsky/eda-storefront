package domain

import (
	"context"
	"errors"
)

var ErrProductNotFound = errors.New("product not found")

// ProductRepository defines the behavior needed for a concrete type
// to persist data in a repository
type ProductRepository interface {
	Save(ctx context.Context, product *Product) error
	FindByID(ctx context.Context, id string) (*Product, error)
}
