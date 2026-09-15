// depot/internal/application/get_shopping_list.go
package application

import (
	"context"
	"fmt"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
)

type GetShoppingList struct {
	shoppingListRepo domain.ShoppingListRepository
}

func NewGetShoppingList(shoppingListRepo domain.ShoppingListRepository) *GetShoppingList {
	return &GetShoppingList{
		shoppingListRepo: shoppingListRepo,
	}
}

func (uc *GetShoppingList) Execute(ctx context.Context, id string) (*domain.ShoppingList, error) {

	shoppingList, err := uc.shoppingListRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("finding shopping list: %w", err)
	}

	return shoppingList, nil
}
