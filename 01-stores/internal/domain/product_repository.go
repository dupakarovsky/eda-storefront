package domain

import (
	"context"
	"errors"
)

var ErrProductNotFound = errors.New("product not found")

type ProductRepository interface {
	Save(ctx context.Context, product *Product) error
	FindByID(ctx context.Context, id string) (*Product, error)
}
