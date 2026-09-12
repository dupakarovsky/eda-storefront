// baskets/internal/catalog/repository.go — replaces Step 4's version
package catalog

import (
	"context"
	"errors"
)

var (
	ErrProductNotFound        = errors.New("product not found in catalog")
	ErrPriceAdjustmentSkipped = errors.New("price adjustment skipped: already applied or product not found")
)

// Repository defines the behavior a concrete must have to access the
// catalog_store and catalog_product tables
type Repository interface {
	CreateStore(ctx context.Context, id, name, location string) error
	RenameStore(ctx context.Context, id, name string) error
	SetStoreParticipation(ctx context.Context, id string, participating bool) error

	CreateProduct(ctx context.Context, id, storeID, name, description, sku string, price float64) error
	RebrandProduct(ctx context.Context, id, name, description string) error
	AdjustProductPrice(ctx context.Context, id string, delta float64, seq uint64) error
	DeleteProduct(ctx context.Context, id string) error

	FindProductByID(ctx context.Context, id string) (*Product, error)
}
