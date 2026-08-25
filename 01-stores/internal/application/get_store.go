package application

import (
	"context"
	"fmt"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

// GetStore is a type that can communicate with the Store Repository
// for read queries related to the Store
type GetStore struct {
	storeRepo domain.StoreRepository
}

// NewGetStore instantiates a new GetStore type
func NewGetStore(storeRepo domain.StoreRepository) *GetStore {
	return &GetStore{
		storeRepo: storeRepo,
	}
}

// Execute will communicate with the database and execute the query,
func (uc *GetStore) Execute(ctx context.Context, storeID string) (*domain.Store, error) {
	store, err := uc.storeRepo.FindByID(ctx, storeID)
	if err != nil {
		return nil, fmt.Errorf("finding store: %w", err)
	}

	return store, nil
}
