package application

import (
	"context"
	"fmt"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

// ListProducts is type that can communicate with the Product Repository
// and make queries about Products
type ListProducts struct {
	productRepo domain.ProductRepository
}

// NewListProducts instantiates a new NewListProducts struct
func NewListProducts(productRepo domain.ProductRepository) *ListProducts {
	return &ListProducts{
		productRepo: productRepo,
	}
}

// Execute will consult with the database and return the list or products
func (uc *ListProducts) Execute(ctx context.Context, storeID string) ([]*domain.Product, error) {
	products, err := uc.productRepo.ListByStoreID(ctx, storeID)
	if err != nil {
		return nil, fmt.Errorf("listing products: %w", err)
	}

	return products, nil
}
