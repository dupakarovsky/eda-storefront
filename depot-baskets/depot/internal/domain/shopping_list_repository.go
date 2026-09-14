// depot/internal/domain/shopping_list_repository.go
package domain

import (
	"context"
	"errors"
)

var ErrShoppingListNotFound = errors.New("shopping list not found")

type ShoppingListRepository interface {
	Save(ctx context.Context, shoppingList *ShoppingList) error
	FindByIDForUpdate(ctx context.Context, id string) (*ShoppingList, error)
	WithLock(ctx context.Context, fn func(txCtx context.Context) error) error

	FindByID(ctx context.Context, id string) (*ShoppingList, error)
}
