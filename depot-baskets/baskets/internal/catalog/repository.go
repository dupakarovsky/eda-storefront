// baskets/internal/catalog/repository.go — replaces Step 4's version
package catalog

import (
	"context"
	"errors"
)

var (
	ErrProductNotFound        = errors.New("product not found in catalog")
	ErrPriceAdjustmentSkipped = errors.New("price adjustment skipped: already applied")
)

// Repository defines the behavior a concrete must have to access the
// catalog_store and catalog_product tables
type Repository interface {
	// --- store ---
	CreateStore(ctx context.Context, id, name, location string) error
	RenameStore(ctx context.Context, id, name string) error
	SetStoreParticipation(ctx context.Context, id string, participating bool) error

	// --- product ---
	CreateProduct(ctx context.Context, id, storeID, name, description, sku string, price float64, seq uint64) error
	SetProductPrice(ctx context.Context, id string, price float64, seq uint64) error
	RebrandProduct(ctx context.Context, id, name, description string) error
	AdjustProductPrice(ctx context.Context, id string, delta float64, seq uint64) error
	DeleteProduct(ctx context.Context, id string) error
	FindStoreIDsByProductIDs(ctx context.Context, productIDs []string) (map[string]string, error)

	FindProductByID(ctx context.Context, id string) (*Product, error)
}
