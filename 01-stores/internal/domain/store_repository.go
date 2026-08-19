package domain

import (
	"context"
	"errors"
)

var ErrStoreNotFound = errors.New("store not found")

type StoreRepository interface {
	Save(ctx context.Context, store *Store) error
	FindByID(ctx context.Context, id string) (*Store, error)
}
