package domain

import (
	"context"
	"errors"
)

var ErrStoreNotFound = errors.New("store not found")

// StoreRepository defines the behaviour needed for a concrete type
// to persiste data in a repository
type StoreRepository interface {
	// write
	Save(ctx context.Context, store *Store) error
	FindByIDForUpdate(ctx context.Context, id string) (*Store, error)
	// read
	FindByID(ctx context.Context, id string) (*Store, error)
}
